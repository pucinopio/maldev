package websocket_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/oioio-space/maldev/c2/transport"
	wstransport "github.com/oioio-space/maldev/c2/transport/websocket"
)

// deadTransport is a Transport that always fails Connect. Used to
// force the Router onto its fallback channel — the WS transport.
type deadTransport struct{}

func (deadTransport) Connect(_ context.Context) error { return errors.New("primary dead") }
func (deadTransport) Read(_ []byte) (int, error)      { return 0, errors.New("dead") }
func (deadTransport) Write(_ []byte) (int, error)     { return 0, errors.New("dead") }
func (deadTransport) Close() error                    { return nil }
func (deadTransport) RemoteAddr() net.Addr            { return deadAddr{} }

type deadAddr struct{}

func (deadAddr) Network() string { return "dead" }
func (deadAddr) String() string  { return "dead://0" }

// Compile-time assertion.
var _ transport.Transport = (*deadTransport)(nil)

// TestRouterIntegration_FallbackToWS proves that Router composes
// cleanly with a real WebSocket Transport as a fallback tier. The
// primary channel is dead; the Router falls over to WS, and a
// round-trip succeeds.
func TestRouterIntegration_FallbackToWS(t *testing.T) {
	// Server side — WS listener on a random localhost port.
	lst, err := wstransport.NewListener("127.0.0.1:0", "/")
	if err != nil {
		t.Fatalf("NewListener: %v", err)
	}
	defer lst.Close()

	wsURL := "ws://" + lst.Addr().String() + "/"

	// Client side — Router with dead primary, WS fallback.
	wsT := wstransport.NewWebSocket(wsURL)
	r, err := transport.NewRouter(
		[]transport.Transport{deadTransport{}, wsT},
		transport.RouterConfig{
			InitialBackoff: time.Millisecond,
			MaxAttempts:    1,
		})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The Router falls over from deadTransport to the WS transport.
	if err := r.Connect(ctx); err != nil {
		t.Fatalf("Router.Connect: %v", err)
	}
	if r.ActiveIndex() != 1 {
		t.Errorf("ActiveIndex = %d, want 1 (WS fallback)", r.ActiveIndex())
	}

	// Round-trip a frame through the Router → WS dialer → server.
	serverConn, err := lst.Accept(ctx)
	if err != nil {
		t.Fatalf("server Accept: %v", err)
	}
	defer serverConn.Close()

	if _, err := r.Write([]byte("via-router")); err != nil {
		t.Fatalf("Router.Write: %v", err)
	}

	buf := make([]byte, 16)
	n, err := serverConn.Read(buf)
	if err != nil {
		t.Fatalf("server Read: %v", err)
	}
	if string(buf[:n]) != "via-router" {
		t.Errorf("server got %q, want 'via-router'", buf[:n])
	}
}
