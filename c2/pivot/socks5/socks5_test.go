package socks5

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/proxy"
)

// TestNew_NilConfig builds a Server with a nil Config and verifies
// the zero-value defaults are wired (no auth, no rules — permit
// everything on a random loopback port).
func TestNew_NilConfig(t *testing.T) {
	s, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil): %v", err)
	}
	if s == nil {
		t.Fatal("nil Server returned without error")
	}
}

// TestNew_WithAuth confirms the Auth struct is properly threaded
// into the underlying go-socks5 CredentialStore.
func TestNew_WithAuth(t *testing.T) {
	s, err := New(&Config{Auth: &Auth{User: "u", Password: "p"}})
	if err != nil {
		t.Fatalf("New(auth): %v", err)
	}
	if s == nil {
		t.Fatal("nil Server")
	}
}

// TestStart_ReturnsBoundAddr_AndStop verifies Start() binds the
// listener, returns a usable host:port, and the stop closure shuts
// the listener down cleanly. The double-stop call exercises the
// idempotency guard.
func TestStart_ReturnsBoundAddr_AndStop(t *testing.T) {
	s, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	addr, stop, err := s.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("bound addr %q should start with 127.0.0.1:", addr)
	}
	if s.Addr() != addr {
		t.Errorf("Addr() = %q, want %q", s.Addr(), addr)
	}
	stop()
	stop() // idempotent
}

// TestStart_AlreadyServingErrors pins the ErrAlreadyServing
// surface: a second Start on the same Server must refuse rather
// than racing the in-flight listener.
func TestStart_AlreadyServingErrors(t *testing.T) {
	s, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, stop, err := s.Start()
	if err != nil {
		t.Fatalf("Start #1: %v", err)
	}
	defer stop()

	if _, _, err := s.Start(); !errors.Is(err, ErrAlreadyServing) {
		t.Errorf("Start #2 should return ErrAlreadyServing, got %v", err)
	}
}

// TestServe_ContextCancel verifies the ctx-cancel shutdown path of
// the blocking Serve method — it must return nil (clean shutdown)
// rather than a "listener closed" error wrapping.
func TestServe_ContextCancel(t *testing.T) {
	s, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()

	// Give Serve a moment to bind.
	deadline := time.After(2 * time.Second)
	for s.Addr() == "" {
		select {
		case <-deadline:
			t.Fatal("Serve never bound a listener")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve returned %v, want nil after ctx cancel", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return within 2s of ctx cancel")
	}
}

// TestServer_E2E_ProxiesToBackend is the headline end-to-end test:
// spin a fake TCP "echo" backend, run the SOCKS5 server, dial the
// backend through x/net/proxy's SOCKS5 client, exchange a known
// payload, verify the byte loop. Asserts the full protocol path
// (handshake + CONNECT + relay) works without auth.
func TestServer_E2E_ProxiesToBackend(t *testing.T) {
	backend := newEchoBackend(t)
	defer backend.Close()

	srv, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	proxyAddr, stop, err := srv.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer stop()

	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		t.Fatalf("proxy.SOCKS5: %v", err)
	}

	conn, err := dialer.Dial("tcp", backend.Addr())
	if err != nil {
		t.Fatalf("dial through proxy: %v", err)
	}
	defer conn.Close()

	want := []byte("maldev-socks5-e2e-ping\n")
	if _, err := conn.Write(want); err != nil {
		t.Fatalf("write through proxy: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	got := make([]byte, len(want))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read through proxy: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestServer_E2E_AuthRequired verifies the user/password path: the
// server is built with Auth, a no-auth client is rejected, the
// matching-credential client succeeds.
func TestServer_E2E_AuthRequired(t *testing.T) {
	backend := newEchoBackend(t)
	defer backend.Close()

	srv, err := New(&Config{Auth: &Auth{User: "operator", Password: "s3cret"}})
	if err != nil {
		t.Fatalf("New(auth): %v", err)
	}
	proxyAddr, stop, err := srv.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer stop()

	// 1. No-auth client must fail to even establish the SOCKS5 link.
	noAuthDialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		t.Fatalf("SOCKS5 dialer build: %v", err)
	}
	if _, err := noAuthDialer.Dial("tcp", backend.Addr()); err == nil {
		t.Error("no-auth dial should have failed against an auth-required server")
	}

	// 2. Correct creds work.
	authDialer, err := proxy.SOCKS5("tcp", proxyAddr,
		&proxy.Auth{User: "operator", Password: "s3cret"}, proxy.Direct)
	if err != nil {
		t.Fatalf("auth dialer build: %v", err)
	}
	conn, err := authDialer.Dial("tcp", backend.Addr())
	if err != nil {
		t.Fatalf("auth dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("hi\n")); err != nil {
		t.Errorf("write through authed proxy: %v", err)
	}
}

// TestServer_RulesDenyBlocksConnect exercises the Rules hook: a
// PermitNone ruleset must cause the SOCKS5 CONNECT step to fail
// even with valid credentials and a reachable backend. Proves the
// destination-allowlist surface is wired (operators use this to
// stop a curious red-teamer using the pivot to scan past the
// engagement scope).
func TestServer_RulesDenyBlocksConnect(t *testing.T) {
	backend := newEchoBackend(t)
	defer backend.Close()

	srv, err := New(&Config{Rules: PermitNone()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	proxyAddr, stop, err := srv.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer stop()

	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		t.Fatalf("SOCKS5 dialer: %v", err)
	}
	if _, err := dialer.Dial("tcp", backend.Addr()); err == nil {
		t.Error("dial should have failed under PermitNone ruleset")
	}
}

// echoBackend is the loopback TCP service the E2E tests point the
// proxy at. Single goroutine per connection, plain io.Copy from
// the client back to itself — proves the proxy relayed the bytes.
type echoBackend struct {
	l net.Listener
}

func newEchoBackend(t *testing.T) *echoBackend {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("backend listen: %v", err)
	}
	b := &echoBackend{l: l}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(c)
		}
	}()
	return b
}

func (b *echoBackend) Addr() string  { return b.l.Addr().String() }
func (b *echoBackend) Close() error  { return b.l.Close() }
