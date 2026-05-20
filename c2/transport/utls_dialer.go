package transport

import (
	"context"
	"fmt"
	"net"
	"time"

	utls "github.com/refraction-networking/utls"
)

// UTLSDialer returns a dial function that performs TCP-then-uTLS
// handshake using the provided config and ClientHelloID. The
// returned closure matches the signature expected by
// [net/http.Transport.DialTLSContext] and
// [github.com/coder/websocket.DialOptions]'s HTTPClient, so any
// consumer that takes such a hook can be transparently spoofed
// with a Chrome/Firefox/Safari JA3 fingerprint.
//
// `cfg` may have an empty ServerName — in that case the dialer
// fills it from the addr host on each call. Pass a non-zero
// timeout to bound the TCP dial; the TLS handshake honours the
// caller's context.
//
// The closure clones `cfg` on every invocation so a single
// UTLSDialer can be safely reused across goroutines and dials.
func UTLSDialer(cfg *utls.Config, hello utls.ClientHelloID, timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := &net.Dialer{Timeout: timeout}
		rawConn, err := d.DialContext(ctx, network, addr)
		if err != nil {
			return nil, fmt.Errorf("TCP dial: %w", err)
		}
		cfgClone := cfg.Clone()
		if cfgClone.ServerName == "" {
			host, _, splitErr := net.SplitHostPort(addr)
			if splitErr != nil {
				host = addr
			}
			cfgClone.ServerName = host
		}
		tlsConn := utls.UClient(rawConn, cfgClone, hello)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			rawConn.Close()
			return nil, fmt.Errorf("uTLS handshake: %w", err)
		}
		return tlsConn, nil
	}
}
