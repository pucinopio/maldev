package socks5

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	gosocks5 "github.com/armon/go-socks5"
)

// ErrAlreadyServing is returned by Serve / Start when the server is
// already running. Stop before launching a new listener.
var ErrAlreadyServing = errors.New("c2/pivot/socks5: server already serving")

// RuleSet aliases armon/go-socks5's RuleSet so consumers don't need
// to import the underlying package for the common Permit* surface.
type RuleSet = gosocks5.RuleSet

// PermitAll returns a RuleSet that accepts every CONNECT request.
// Re-exports armon/go-socks5's helper so callers can build a Config
// without importing the underlying package.
func PermitAll() RuleSet { return gosocks5.PermitAll() }

// PermitNone returns a RuleSet that rejects every CONNECT request.
// Useful for self-test loops and as a starting point for custom
// restrictive rule sets.
func PermitNone() RuleSet { return gosocks5.PermitNone() }

// Auth pairs a username + password the SOCKS5 client must present
// over the protocol's RFC 1929 user/password method. Zero-value
// disables authentication (the listener accepts any client).
//
// Plaintext credentials over the wire — useful only when the
// operator's connection to the beacon is already encrypted (TLS
// tunnel, in-process via Caller, etc.). For genuine no-tunnel
// exposure use [Config.Rules] to constrain destinations instead of
// relying on the password.
type Auth struct {
	User     string
	Password string
}

// Config controls a [Server]. The zero value gives an
// authentication-less, permit-all proxy on a random loopback port
// — fine for unit tests, NOT what you want shipped.
type Config struct {
	// Listen is the bind address ("host:port") of the SOCKS5
	// socket. Empty defaults to "127.0.0.1:0" (loopback, random
	// port). Use "0.0.0.0:1080" only when the network surface is
	// explicitly part of the operator plan — exposing SOCKS5 to a
	// hostile network is an instant red-team-loud signal.
	Listen string

	// Auth optionally enables RFC 1929 user/password
	// authentication. nil = no auth challenge.
	Auth *Auth

	// Rules optionally constrains the destinations the proxy will
	// connect to. nil defaults to permit-all. Use [PermitAll],
	// [PermitNone], or any custom [RuleSet] (e.g. deny CONNECT to
	// the operator's own infrastructure).
	Rules RuleSet
}

// Server is a SOCKS5 listener. Build it with [New], then call
// [Server.Serve] (blocking) or [Server.Start] (background). After
// the listener is closed via Stop / ctx cancel, the *Server is
// inert — build a fresh one for the next pivot session.
type Server struct {
	listenAddr string
	inner      *gosocks5.Server
	listener   net.Listener
}

// New builds a configured but un-bound [Server]. The OS resources
// (socket, goroutine) are not yet allocated — use [Server.Serve]
// or [Server.Start] to actually bind and accept.
func New(cfg *Config) (*Server, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	gconf := &gosocks5.Config{Rules: cfg.Rules}
	if cfg.Auth != nil {
		gconf.Credentials = gosocks5.StaticCredentials{cfg.Auth.User: cfg.Auth.Password}
	}
	inner, err := gosocks5.New(gconf)
	if err != nil {
		return nil, fmt.Errorf("c2/pivot/socks5: build inner server: %w", err)
	}
	return &Server{listenAddr: cfg.Listen, inner: inner}, nil
}

// bind reserves the listener socket using the configured listen
// address (or the 127.0.0.1:0 loopback default). Mutates s.listener
// on success — callers MUST not call bind concurrently. Shared by
// Serve + Start.
func (s *Server) bind() (net.Listener, error) {
	if s.listener != nil {
		return nil, ErrAlreadyServing
	}
	addr := s.listenAddr
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("c2/pivot/socks5: listen %s: %w", addr, err)
	}
	s.listener = l
	return l, nil
}

// Serve binds the listen socket and blocks accepting connections
// until ctx is cancelled or the underlying listener is closed.
// Returns [ErrAlreadyServing] if the server is already accepting,
// nil on clean shutdown (ctx-cancel or listener.Close), or the
// wrapped accept-loop error otherwise.
//
// The bound address is observable via [Server.Addr] once the
// listener is up. Callers that need the resolved port without
// racing the accept goroutine should prefer [Server.Start] which
// returns the address synchronously.
func (s *Server) Serve(ctx context.Context) error {
	l, err := s.bind()
	if err != nil {
		return err
	}

	// done guards the ctx-watch goroutine against leaking when the
	// accept loop exits before ctx fires (transient listener error,
	// platform-level interruption, etc.). Close done on return so
	// the watcher unblocks regardless of cancellation order.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = l.Close()
		case <-done:
		}
	}()

	// armon/go-socks5 returns net.ErrClosed when the underlying
	// listener is closed — surface that as nil so shutdown via
	// ctx cancel is a clean exit, not an error path.
	if err := s.inner.Serve(l); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("c2/pivot/socks5: accept loop: %w", err)
	}
	return nil
}

// Start binds the listen socket and launches the accept loop on a
// goroutine. Returns the bound address immediately so callers can
// wire clients without racing the accept goroutine.
//
// The returned `stop` closure shuts the listener down + drains the
// accept goroutine. Safe to call multiple times (sync.Once-guarded);
// the second call is a no-op.
func (s *Server) Start() (addr string, stop func(), err error) {
	l, err := s.bind()
	if err != nil {
		return "", nil, err
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.inner.Serve(l)
	}()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			_ = l.Close()
			<-done
		})
	}
	return l.Addr().String(), stop, nil
}

// Addr returns the bound address (host:port) once the listener is
// up, or the empty string before. Useful for tests / log messages
// when the port was assigned dynamically (Listen: "" or "*:0").
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}
