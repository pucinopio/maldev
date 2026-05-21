package httpsrv

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oioio-space/maldev/cleanup/memory"
	"github.com/oioio-space/maldev/internal/manager/service"
	licenseent "github.com/oioio-space/maldev/internal/manager/store/ent/license"
	"github.com/oioio-space/maldev/license/heartbeat"
)

// HeartbeatServer serves signed heartbeat replies for license liveness checks.
// It mirrors RevocationServer's lifecycle contract (Start/Stop/Status/Events).
type HeartbeatServer struct {
	issuer   *service.IssuerService
	license  *service.LicenseService
	settings *service.SettingsService

	mu        sync.Mutex
	srv       *http.Server
	listener  net.Listener
	startedAt time.Time
	lastErr   string

	requests atomic.Uint64
	lastReq  atomic.Int64

	events chan Event
}

// NewHeartbeatServer wires the server against the given services.
func NewHeartbeatServer(iss *service.IssuerService, lic *service.LicenseService, settings *service.SettingsService) *HeartbeatServer {
	return &HeartbeatServer{
		issuer:   iss,
		license:  lic,
		settings: settings,
		events:   make(chan Event, 256),
	}
}

func (s *HeartbeatServer) Name() string { return "heartbeat" }

// Start binds the listener using the address from SettingsService and begins
// serving. Returns an error if already running.
func (s *HeartbeatServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		return errors.New("heartbeat server already running")
	}
	cfg, err := s.settings.GetServerConfig(ctx)
	if err != nil {
		return fmt.Errorf("server config: %w", err)
	}
	path := cfg.HeartbeatPath
	if path == "" {
		path = "/heartbeat"
	}
	mux := http.NewServeMux()
	mux.HandleFunc(path, s.handle)
	ln, err := net.Listen("tcp", cfg.HeartbeatListen)
	if err != nil {
		s.lastErr = err.Error()
		return fmt.Errorf("listen %s: %w", cfg.HeartbeatListen, err)
	}
	s.listener = ln
	httpSrv := &http.Server{Handler: mux, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second}
	s.srv = httpSrv
	s.startedAt = time.Now()
	go func() {
		var serveErr error
		if cfg.HeartbeatTLSCert != "" && cfg.HeartbeatTLSKey != "" {
			serveErr = httpSrv.ServeTLS(ln, cfg.HeartbeatTLSCert, cfg.HeartbeatTLSKey)
		} else {
			serveErr = httpSrv.Serve(ln)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			s.mu.Lock()
			s.lastErr = serveErr.Error()
			s.mu.Unlock()
			s.emit(Event{At: time.Now(), Server: s.Name(), Kind: "error", Note: serveErr.Error()})
		}
	}()
	s.emit(Event{At: time.Now(), Server: s.Name(), Kind: "started", Note: ln.Addr().String()})
	return nil
}

// Stop gracefully shuts down within timeout.
func (s *HeartbeatServer) Stop(timeout time.Duration) error {
	s.mu.Lock()
	srv := s.srv
	s.srv = nil
	s.listener = nil
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := srv.Shutdown(ctx)
	s.emit(Event{At: time.Now(), Server: s.Name(), Kind: "stopped"})
	return err
}

// Status returns a non-blocking snapshot of the server's runtime state.
func (s *HeartbeatServer) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{LastError: s.lastErr}
	if s.srv != nil {
		st.Running = true
		st.StartedAt = s.startedAt
	}
	if s.listener != nil {
		st.ListenAddr = s.listener.Addr().String()
	}
	st.Requests = s.requests.Load()
	if last := s.lastReq.Load(); last != 0 {
		st.LastReq = time.Unix(0, last)
	}
	return st
}

// Events returns the per-request and lifecycle event channel.
func (s *HeartbeatServer) Events() <-chan Event { return s.events }

func (s *HeartbeatServer) emit(e Event) {
	select {
	case s.events <- e:
	default:
		// Drop oldest to keep the channel from blocking the hot path.
		select {
		case <-s.events:
		default:
		}
		select {
		case s.events <- e:
		default:
		}
	}
}

func (s *HeartbeatServer) emitReq(r *http.Request, status int) {
	s.emit(Event{
		At:     time.Now(),
		Server: s.Name(),
		Kind:   "request",
		Method: r.Method,
		Path:   r.URL.Path,
		Status: status,
		Remote: r.RemoteAddr,
	})
}

func (s *HeartbeatServer) handle(w http.ResponseWriter, r *http.Request) {
	s.requests.Add(1)
	s.lastReq.Store(time.Now().UnixNano())
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		s.emitReq(r, http.StatusMethodNotAllowed)
		return
	}
	var req heartbeat.Request
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		s.emitReq(r, http.StatusBadRequest)
		return
	}

	iss, err := s.issuer.Active(r.Context())
	if err != nil {
		http.Error(w, "no active issuer", http.StatusInternalServerError)
		s.emitReq(r, http.StatusInternalServerError)
		return
	}
	privBytes, err := s.issuer.PrivateKey(r.Context(), iss.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.emitReq(r, http.StatusInternalServerError)
		return
	}
	defer memory.SecureZero(privBytes)
	priv := ed25519.PrivateKey(privBytes)

	now := time.Now().UTC()
	reply := heartbeat.Reply{
		Version:    1,
		KeyID:      iss.KeyID,
		LicenseID:  req.LicenseID,
		NonceEcho:  req.Nonce,
		ServerTime: now,
	}
	row, err := s.license.GetByUUID(r.Context(), req.LicenseID)
	switch {
	case err != nil:
		reply.Reason = "unknown"
	case row.Status == licenseent.StatusRevoked:
		reply.Reason = "revoked"
	case row.Status == licenseent.StatusExpired, row.Status == licenseent.StatusSuperseded:
		reply.Reason = "expired"
	case row.Status == licenseent.StatusActive:
		reply.Ok = true
		reply.ValidUntil = now.Add(time.Hour)
	default:
		reply.Reason = "unknown"
	}
	signed, err := heartbeat.SignReply(reply, priv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.emitReq(r, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	_, _ = w.Write(signed)
	s.emitReq(r, http.StatusOK)
}
