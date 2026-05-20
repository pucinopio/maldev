package websocket_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cws "github.com/coder/websocket"

	wstransport "github.com/oioio-space/maldev/c2/transport/websocket"
)

// echoServer is the workhorse fake for dial tests.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := cws.Accept(w, r, &cws.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer ws.Close(cws.StatusNormalClosure, "")
		for {
			typ, data, err := ws.Read(r.Context())
			if err != nil {
				return
			}
			_ = ws.Write(r.Context(), typ, data)
		}
	}))
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

// TestDial_RoundTrip exercises the happy path: dial → write → read
// → close.
func TestDial_RoundTrip(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	tr := wstransport.NewWebSocket(wsURL(srv.URL))
	if err := tr.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer tr.Close()

	if _, err := tr.Write([]byte("ping")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	buf := make([]byte, 16)
	n, err := tr.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buf[:n]) != "ping" {
		t.Errorf("got %q, want 'ping'", buf[:n])
	}
}

// TestDial_ConnectionRefused asserts a dial to a closed port surfaces
// a wrapped error.
func TestDial_ConnectionRefused(t *testing.T) {
	tr := wstransport.NewWebSocket("ws://127.0.0.1:1") // port 1 is privileged + closed
	err := tr.Connect(context.Background())
	if err == nil {
		t.Fatal("expected dial error to a closed port")
	}
}

// TestDial_UpgradeRejected asserts a non-101 response is surfaced as
// an error rather than a successful Transport.
func TestDial_UpgradeRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK) // not 101 Switching Protocols
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	tr := wstransport.NewWebSocket(wsURL(srv.URL))
	if err := tr.Connect(context.Background()); err == nil {
		t.Fatal("expected upgrade-rejected error")
	}
}

// TestDial_CtxCancel asserts a pre-cancelled context surfaces
// context.Canceled (or DeadlineExceeded), not a successful dial.
func TestDial_CtxCancel(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel
	tr := wstransport.NewWebSocket(wsURL(srv.URL))
	err := tr.Connect(ctx)
	if err == nil {
		t.Fatal("expected ctx-cancelled error")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v, want context.Canceled or DeadlineExceeded", err)
	}
}

// TestRead_NormalClosure proves that a server-side StatusNormalClosure
// surfaces as io.EOF (the convention coder/websocket.NetConn enforces).
func TestRead_NormalClosure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := cws.Accept(w, r, &cws.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		_ = ws.Close(cws.StatusNormalClosure, "bye")
	}))
	defer srv.Close()

	tr := wstransport.NewWebSocket(wsURL(srv.URL))
	if err := tr.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer tr.Close()

	buf := make([]byte, 4)
	_, err := tr.Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Errorf("got %v, want io.EOF", err)
	}
}

// TestClose_Idempotent asserts double Close is safe (the lesson from
// Router /simplify — both halves of the channel-lifecycle API must
// be idempotent).
func TestClose_Idempotent(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	tr := wstransport.NewWebSocket(wsURL(srv.URL))
	if err := tr.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := tr.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := tr.Close(); err != nil {
		t.Errorf("second Close (idempotent): %v", err)
	}
}

// TestDial_DialTimeoutDoesNotFireOnFastDial pins that a generous
// timeout doesn't spuriously cancel a fast localhost dial.
func TestDial_DialTimeoutDoesNotFireOnFastDial(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	tr := wstransport.NewWebSocket(wsURL(srv.URL), wstransport.WithDialTimeout(2*time.Second))
	if err := tr.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	_ = tr.Close()
}
