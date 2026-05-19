package socks5_test

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/net/proxy"

	"github.com/oioio-space/maldev/c2/pivot/socks5"
)

// ExampleNew — minimal: bind a SOCKS5 proxy on a random loopback
// port, no auth, no rules. Useful for tests / quick pivot probing.
func ExampleNew() {
	srv, err := socks5.New(nil)
	if err != nil {
		fmt.Println("New:", err)
		return
	}
	addr, stop, err := srv.Start()
	if err != nil {
		fmt.Println("Start:", err)
		return
	}
	defer stop()
	_ = addr // dial through this from the operator side
}

// Example_composed — auth-required proxy with PermitAll routing.
// The operator dials with user:operator / password:s3cret. Both
// Auth and Rules are independent knobs — pick them per engagement.
func Example_composed() {
	srv, _ := socks5.New(&socks5.Config{
		Listen: "127.0.0.1:0",
		Auth:   &socks5.Auth{User: "operator", Password: "s3cret"},
		Rules:  socks5.PermitAll(),
	})
	_, stop, _ := srv.Start()
	defer stop()
}

// Example_advanced — context-cancel lifecycle. Run Serve on a
// goroutine, cancel the context after some operator-defined idle
// window, and let the accept loop exit cleanly without leaking the
// listener handle. Production-shape for an implant's beacon-side
// SOCKS5 that should auto-shut after the engagement window.
func Example_advanced() {
	srv, _ := socks5.New(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() { _ = srv.Serve(ctx) }()

	// Wait until the listener is bound so the operator-side
	// connection has somewhere to land.
	for srv.Addr() == "" {
		time.Sleep(10 * time.Millisecond)
	}
	_ = srv.Addr() // operator gets this via the C2 channel and dials

	// ... operator pivots through srv.Addr() until ctx fires ...
}

// Example_complex — end-to-end: pair the maldev SOCKS5 server
// (this package) with the stdlib x/net/proxy SOCKS5 client + a
// downstream backend service the operator wants to reach. This is
// the full client-server-target loop, matching the unit-test path
// at TestServer_E2E_ProxiesToBackend but reshaped as the canonical
// operator-side usage. Run Echo on a fake LAN service to confirm
// the relay before pivoting against a real target.
func Example_complex() {
	// Target service the operator wants to reach.
	backend, _ := net.Listen("tcp", "127.0.0.1:0")
	defer backend.Close()
	go func() {
		c, err := backend.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 64)
		n, _ := c.Read(buf)
		_, _ = c.Write(buf[:n]) // echo back
	}()

	// Beacon-side SOCKS5.
	srv, _ := socks5.New(nil)
	proxyAddr, stop, _ := srv.Start()
	defer stop()

	// Operator-side dial through the proxy.
	dialer, _ := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	conn, _ := dialer.Dial("tcp", backend.Addr().String())
	defer conn.Close()

	_, _ = conn.Write([]byte("ping"))
	got := make([]byte, 4)
	_, _ = conn.Read(got)
	fmt.Println(string(got))
	// Output: ping
}
