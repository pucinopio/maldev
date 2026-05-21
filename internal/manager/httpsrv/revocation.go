package httpsrv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/service"
)

// RevocationServer serves the signed revocation list and accepts admin
// add/remove POSTs.
type RevocationServer struct {
	revoke   *service.RevokeService
	license  *service.LicenseService
	settings *service.SettingsService
	kek      *crypto.KEK

	mu        sync.Mutex
	srv       *http.Server
	listener  net.Listener
	startedAt time.Time
	lastErr   string

	requests atomic.Uint64
	lastReq  atomic.Int64 // unix nano

	events chan Event
}

// NewRevocationServer wires the dependencies. Call Start to bind and serve.
func NewRevocationServer(revoke *service.RevokeService, license *service.LicenseService, settings *service.SettingsService, kek *crypto.KEK) *RevocationServer {
	return &RevocationServer{
		revoke:   revoke,
		license:  license,
		settings: settings,
		kek:      kek,
		events:   make(chan Event, 256),
	}
}

// Name implements Server.
func (s *RevocationServer) Name() string { return "revocation" }

// Start binds the listener and serves. Returns once the listener is ready.
func (s *RevocationServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		return errors.New("revocation server already running")
	}
	cfg, err := s.settings.GetServerConfig(ctx)
	if err != nil {
		return fmt.Errorf("server config: %w", err)
	}
	listen := cfg.RevocationListen
	path := cfg.RevocationPath
	if path == "" {
		path = "/revoked.pem"
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, s.handle)

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		s.lastErr = err.Error()
		return fmt.Errorf("listen %s: %w", listen, err)
	}
	s.listener = ln
	s.srv = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	s.startedAt = time.Now()
	httpSrv := s.srv // capture before releasing the mutex

	go func() {
		var serveErr error
		if cfg.RevocationTLSCert != "" && cfg.RevocationTLSKey != "" {
			serveErr = httpSrv.ServeTLS(ln, cfg.RevocationTLSCert, cfg.RevocationTLSKey)
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

// Stop shuts down gracefully up to timeout.
func (s *RevocationServer) Stop(timeout time.Duration) error {
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

// Status returns a snapshot.
func (s *RevocationServer) Status() Status {
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

// Events returns the buffered live-log channel.
func (s *RevocationServer) Events() <-chan Event { return s.events }

// emit pushes an event with drop-oldest semantics.
func (s *RevocationServer) emit(e Event) {
	select {
	case s.events <- e:
	default:
		// Drop oldest by consuming once before pushing.
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

func (s *RevocationServer) handle(w http.ResponseWriter, r *http.Request) {
	s.requests.Add(1)
	s.lastReq.Store(time.Now().UnixNano())
	switch r.Method {
	case http.MethodGet:
		s.serveGet(w, r)
	case http.MethodPost:
		s.serveAdmin(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		s.emitReq(r, http.StatusMethodNotAllowed)
	}
}

func (s *RevocationServer) serveGet(w http.ResponseWriter, r *http.Request) {
	pem, err := s.revoke.PublishSignedList(r.Context(), 7*24*time.Hour)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.emitReq(r, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	_, _ = w.Write(pem)
	s.emitReq(r, http.StatusOK)
}

// serveAdmin parses {"add":[uuid...],"remove":[uuid...]} and applies them.
func (s *RevocationServer) serveAdmin(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.settings.GetServerConfig(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.emitReq(r, http.StatusInternalServerError)
		return
	}
	if len(cfg.RevocationAdminTokenEnc) == 0 {
		http.Error(w, "admin endpoint disabled", http.StatusForbidden)
		s.emitReq(r, http.StatusForbidden)
		return
	}
	expected, err := s.kek.Unwrap(cfg.RevocationAdminTokenEnc)
	if err != nil {
		http.Error(w, "token decrypt failed", http.StatusInternalServerError)
		s.emitReq(r, http.StatusInternalServerError)
		return
	}
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token != string(expected) {
		http.Error(w, "unauthorised", http.StatusUnauthorized)
		s.emitReq(r, http.StatusUnauthorized)
		return
	}

	var body struct {
		Add    []string `json:"add"`
		Remove []string `json:"remove"`
		Reason string   `json:"reason"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		s.emitReq(r, http.StatusBadRequest)
		return
	}
	reason := body.Reason
	if reason == "" {
		reason = "admin-api"
	}
	for _, id := range body.Add {
		row, err := s.license.GetByUUID(r.Context(), id)
		if err != nil {
			http.Error(w, fmt.Sprintf("license %s not found", id), http.StatusNotFound)
			s.emitReq(r, http.StatusNotFound)
			return
		}
		if err := s.revoke.Revoke(r.Context(), row.ID, reason, "admin-api"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			s.emitReq(r, http.StatusInternalServerError)
			return
		}
	}
	for _, id := range body.Remove {
		row, err := s.license.GetByUUID(r.Context(), id)
		if err != nil {
			continue // best-effort
		}
		_ = s.revoke.Unrevoke(r.Context(), row.ID, "admin-api")
	}
	w.WriteHeader(http.StatusOK)
	s.emitReq(r, http.StatusOK)
}

func (s *RevocationServer) emitReq(r *http.Request, status int) {
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
