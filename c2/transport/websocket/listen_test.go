package websocket_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cws "github.com/coder/websocket"

	wstransport "github.com/oioio-space/maldev/c2/transport/websocket"
)

// TestListener_AcceptRoundTrip exercises NewListener end-to-end:
// stand up a server, dial it, exchange a frame.
func TestListener_AcceptRoundTrip(t *testing.T) {
	lst, err := wstransport.NewListener("127.0.0.1:0", "/sync")
	if err != nil {
		t.Fatalf("NewListener: %v", err)
	}
	defer lst.Close()

	url := "ws://" + lst.Addr().String() + "/sync"

	clientDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ws, _, err := cws.Dial(ctx, url, nil)
		if err != nil {
			clientDone <- err
			return
		}
		defer ws.Close(cws.StatusNormalClosure, "")
		_ = ws.Write(ctx, cws.MessageBinary, []byte("hi"))
		clientDone <- nil
	}()

	acceptCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := lst.Accept(acceptCtx)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	buf := make([]byte, 4)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("server Read: %v", err)
	}
	if string(buf[:n]) != "hi" {
		t.Errorf("got %q, want 'hi'", buf[:n])
	}
	if err := <-clientDone; err != nil {
		t.Errorf("client side: %v", err)
	}
}

// TestListener_AcceptCtxCancel asserts ctx cancellation unblocks
// Accept.
func TestListener_AcceptCtxCancel(t *testing.T) {
	lst, err := wstransport.NewListener("127.0.0.1:0", "/")
	if err != nil {
		t.Fatalf("NewListener: %v", err)
	}
	defer lst.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, e := lst.Accept(ctx); done <- e }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("got %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Accept did not unblock on ctx cancel")
	}
}

// TestHandler_MountOnExternalServer proves the co-host pattern: the
// WS handler is mounted alongside a decoy path on an operator-supplied
// http.Server (here httptest), and both paths work.
func TestHandler_MountOnExternalServer(t *testing.T) {
	h, lst := wstransport.NewServer()

	mux := http.NewServeMux()
	mux.HandleFunc("/decoy", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>real site</html>"))
	})
	mux.Handle("/api/sync", h)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/sync"

	clientDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ws, _, err := cws.Dial(ctx, url, nil)
		if err != nil {
			clientDone <- err
			return
		}
		_ = ws.Write(ctx, cws.MessageBinary, []byte("pong"))
		_ = ws.Close(cws.StatusNormalClosure, "")
		clientDone <- nil
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := lst.Accept(ctx)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	buf := make([]byte, 4)
	n, _ := conn.Read(buf)
	if string(buf[:n]) != "pong" {
		t.Errorf("got %q, want 'pong'", buf[:n])
	}
	if err := <-clientDone; err != nil {
		t.Errorf("client: %v", err)
	}

	// Decoy path still works through the same mux.
	resp, err := http.Get(srv.URL + "/decoy")
	if err != nil {
		t.Fatalf("decoy GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("decoy status %d, want 200", resp.StatusCode)
	}
}

// TestListener_BurstDropsExcess proves the buffered connCh +
// non-blocking handler push survives a burst — at least the buffer
// capacity must be accepted, no deadlock.
func TestListener_BurstDropsExcess(t *testing.T) {
	lst, err := wstransport.NewListener("127.0.0.1:0", "/")
	if err != nil {
		t.Fatalf("NewListener: %v", err)
	}
	defer lst.Close()

	url := "ws://" + lst.Addr().String() + "/"

	const dialers = 20
	clientErrs := make(chan error, dialers)
	for i := 0; i < dialers; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			ws, _, err := cws.Dial(ctx, url, nil)
			if err != nil {
				clientErrs <- err
				return
			}
			defer ws.Close(cws.StatusNormalClosure, "")
			clientErrs <- nil
		}()
	}

	accepted := 0
	overall, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for accepted < 16 {
		ctx, perCancel := context.WithTimeout(overall, 500*time.Millisecond)
		c, err := lst.Accept(ctx)
		perCancel()
		if err != nil {
			break
		}
		_ = c
		accepted++
	}
	if accepted < 16 {
		t.Errorf("accepted %d, want at least 16 (buffer size)", accepted)
	}
}
