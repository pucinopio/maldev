// Package socks5 wraps the armon/go-socks5 server in a thin maldev
// primitive — a beacon-side SOCKS5 listener the operator pivots
// through to reach the beacon's network.
//
// The typical deployment shape: an implant calls [Serve] on a
// loopback port (or a port reachable from the operator over an
// existing C2 tunnel), the operator points their browser / curl /
// nmap at it through a forward, and every request lands inside the
// target network as if it had originated on the beacon's host.
//
// # MITRE ATT&CK
//
//   - T1090 (Proxy) — sub-technique T1090.001 (Internal Proxy)
//
// # Detection level
//
// moderate. A SOCKS5 listener on a non-standard port is a generic
// pivot signal; defenders watching outbound connections to RFC1918
// neighbours from an unexpected process will notice. Pair with
// [crypto] for traffic encapsulation when the operator-side tunnel
// is HTTP(S)-based.
//
// # Required privileges
//
// unprivileged. The listener binds an unprivileged TCP port; the
// outbound connections it initiates use the calling process's own
// network handle. No token escalation needed.
//
// # Platform
//
// Cross-platform. The package is pure-Go and depends only on
// stdlib `net` + [github.com/armon/go-socks5] (MPL-2.0 — file-level
// copyleft, compatible with maldev's MIT license when imported as
// a Go dependency).
//
// # Example
//
// See [ExampleServe] in socks5_example_test.go.
//
// # See also
//
//   - [maldev primitives roadmap M5] — `.dev/refactor-2026/maldev-primitives-roadmap.md`
//   - [github.com/oioio-space/maldev/c2/transport] — sibling C2 plumbing (TLS / Malleable profiles)
//   - [github.com/oioio-space/maldev/crypto] — pair with this when the operator-side tunnel is plaintext HTTP
//
// [github.com/oioio-space/maldev/c2/transport]: https://pkg.go.dev/github.com/oioio-space/maldev/c2/transport
// [github.com/oioio-space/maldev/crypto]: https://pkg.go.dev/github.com/oioio-space/maldev/crypto
package socks5
