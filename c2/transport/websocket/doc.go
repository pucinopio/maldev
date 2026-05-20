// Package websocket implements a WebSocket [transport.Transport]
// (dial side) and [transport.Listener] (accept side) for C2
// channels that ride HTTP/1.1 + WS upgrade.
//
// Dial side ([NewWebSocket]) is composable with
// [github.com/oioio-space/maldev/c2/transport.Router] as a fallback
// tier and with [github.com/oioio-space/maldev/c2/transport.UTLSDialer]
// for JA3 spoofing of the TLS handshake under wss://.
//
// Accept side ([NewServer] / [NewListener] / [Handler]) supports
// both stand-alone operation and co-hosting the C2 endpoint inside a
// larger http.Server alongside decoy paths.
//
// # MITRE ATT&CK
//
//   - T1071 (Application Layer Protocol)
//   - T1090.004 (Domain Fronting) — when paired with WithUTLSConfig
//
// # Detection level
//
// moderate
//
// Plain ws:// is loud (browser-style upgrade with no surrounding
// page traffic is a signal). wss:// + uTLS + realistic
// User-Agent / Origin headers degrades detection to "moderate" by
// blending with normal browser WebSocket traffic to chat / API
// endpoints.
//
// # Required privileges
//
// unprivileged.
//
// # Platform
//
// Cross-platform. Pure Go on top of [github.com/coder/websocket].
//
// # Example
//
// See [ExampleNewWebSocket] in dial_test.go and the four worked
// examples in docs/techniques/c2/websocket.md.
//
// # See also
//
//   - docs/techniques/c2/websocket.md
//   - [github.com/oioio-space/maldev/c2/transport.Router]
//   - [github.com/oioio-space/maldev/c2/transport.UTLSDialer]
package websocket
