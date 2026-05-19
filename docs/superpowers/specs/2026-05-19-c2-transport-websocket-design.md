# M3 — WebSocket transport primitive

**Status:** spec approved (Section 1–4 brainstorm) — pending plan + implementation
**Roadmap row:** `.dev/refactor-2026/maldev-primitives-roadmap.md` — M3
**Tracker:** `.dev/refactor-2026/m3-websocket-progress.md`

## Goal

Ship `c2/transport/websocket` — a WebSocket primitive offering both
dial-side `Transport` (implant) and accept-side `Listener` (operator)
implementations. Must compose cleanly with the freshly-landed
`c2/transport.Router` so an implant can fall over HTTPS → WS → DNS
seamlessly, and must compose with the existing `transport.NewUTLS`
fingerprint spoofing so the WS handshake rides on a Chrome-shaped TLS
ClientHello.

## Library: `github.com/coder/websocket`

Pinned version (decided at implementation): probably `v1.8.x`.

Why coder/websocket over gorilla:
- Context-native API (`Read(ctx)`, `Write(ctx, ...)`) matches the
  repo's `Transport.Connect(ctx)` cancellation convention.
- Active maintenance (handed off from `nhooyr.io/websocket` to
  coder.com in 2024).
- No feature gap vs gorilla relevant to this primitive.

## Public surface

```go
package websocket

// Dial-side — satisfies transport.Transport.
func NewWebSocket(rawURL string, opts ...DialOption) transport.Transport

type DialOption func(*dialConfig)
func WithSubprotocols(subs ...string) DialOption
func WithHeader(key, value string) DialOption
func WithTLSConfig(*tls.Config) DialOption
func WithUTLSConfig(*tls.Config, utls.ClientHelloID) DialOption
func WithCompression(bool) DialOption     // default true (matches Chrome)
func WithDialTimeout(time.Duration) DialOption

// Accept-side — Server returns the building blocks; NewListener
// wraps them into a stand-alone listener.
func NewServer(opts ...ListenerOption) (http.Handler, transport.Listener)
func NewListener(addr, path string, opts ...ListenerOption) (transport.Listener, error)
func Handler(opts ...ListenerOption) http.Handler  // sugar over NewServer's handler half

type ListenerOption func(*listenerConfig)
func WithTLS(*tls.Config) ListenerOption
func WithAcceptedSubprotocols(subs ...string) ListenerOption
func WithOriginPatterns(patterns ...string) ListenerOption  // CSRF defense — default permissive
func WithServerCompression(bool) ListenerOption             // default true
```

Composability with the rest of the repo:
- `Router` accepts `NewWebSocket(...)` like any other Transport.
- `Router` accepts `NewListener(...)` results' net.Conn outputs
  (operator side; doesn't ship in this primitive — separate concern).
- `WithUTLSConfig` reuses the existing `transport.UTLSDialer` helper
  (extracted from `NewUTLS` in commit #1 below).

## Internal layout

```
c2/transport/websocket/
  doc.go                          # MITRE T1071, detection moderate, godoc example
  dial.go                         # NewWebSocket + DialOption + wsTransport
  listen.go                       # NewServer + NewListener + Handler + wsListener
  dial_test.go                    # 6-7 dial tests (httptest server)
  listen_test.go                  # 3-4 accept tests (127.0.0.1:0)
  router_integration_test.go      # Router fallback HTTPS→WS proof
```

Plus a refactor commit (#1 below) that extracts a shared helper:

```
c2/transport/utls_dialer.go       # UTLSDialer(*tls.Config, utls.ClientHelloID) func(...) (net.Conn, error)
c2/transport/tls.go               # NewUTLS refactored to consume UTLSDialer
```

## Internal types

```go
// dial.go
type wsTransport struct {
    url        string
    dialOpts   websocket.DialOptions
    httpClient *http.Client    // hosts optional uTLS transport
    timeout    time.Duration

    mu     sync.Mutex
    conn   net.Conn           // websocket.NetConn(...) wrapper
    wsConn *websocket.Conn    // kept for typed close-frame
    closed bool
}

// listen.go
type wsListener struct {
    connCh   chan acceptedConn // buffered cap 16
    srv      *http.Server      // nil when caller mounts handler elsewhere
    addr     net.Addr
    closeMu  sync.Mutex
    closed   bool
}
type acceptedConn struct {
    c   net.Conn
    err error
}
```

## Defaults (opinionated)

| Knob | Default | Why |
|---|---|---|
| Compression (`permessage-deflate`) | on | Chrome enables it — off would fingerprint anomalously |
| Origin check (listener) | permissive (`InsecureSkipVerify=true`) | Implant has no browser-Origin. Operator opts into `WithOriginPatterns` for co-hosting with real sites. **Documented in bold in tech-md.** |
| Subprotocol (client) | none | YAGNI |
| HTTPHeader (client) | none | YAGNI; operator sets `User-Agent`/`Origin`/`Cookie` per campaign |
| DialTimeout | 30s | Aligned with `NewUTLS` (`c2/transport/tls.go`) |
| connCh buffer (listener) | 16 | Absorb implant-checkin bursts without backpressure |

## Concurrency — lessons from Router /simplify

1. **`wsTransport.Close()` releases the mutex BEFORE calling
   `wsConn.Close()`.** Same pattern as `Router.markLost` — a slow
   TLS `close_notify` must not stall concurrent `Read`/`Write`.
2. **Handler never blocks on `connCh`.** `select { case connCh <- c:
   ; default: c.Close() }` — no auto-DoS from a flooded implant.
3. **`wsListener.Close()` uses `srv.Shutdown(ctx)` with grace 3s**,
   not `srv.Close()`. Live conns finish their current frame instead
   of being RST-cut (defender-detectable artefact).
4. **Idempotence via `closed bool` under mutex** on both types.

## Error semantics

- No new sentinel errors — wrap `coder/websocket` errors via
  `fmt.Errorf("ws dial %s: %w", url, err)`.
- Normal closure (WS status 1000) → `Read`/`Write` return `io.EOF`
  (this is `websocket.NetConn`'s built-in behaviour).
- Abnormal closure → wrapped error, `Router.markLost` engages.
- ctx cancel → `ctx.Err()`.

## Test matrix (host-runnable, no Win32, no Caller matrix)

| Test | Goal |
|---|---|
| `TestDial_RoundTrip` | dial → write → read → close |
| `TestDial_ConnectionRefused` | server down → wrapped error |
| `TestDial_UpgradeRejected` | httptest serves 200 instead of 101 → typed error |
| `TestDial_CtxCancel` | ctx cancelled mid-dial → ctx.Err() |
| `TestRead_NormalClosure` | server closes(1000) → io.EOF |
| `TestWrite_AbnormalClosure` | server kills TCP → wrapped error |
| `TestClose_Idempotent` | second Close = no-op, no panic |
| `TestListener_AcceptCtxCancel` | Accept(ctx) unblocks on cancel |
| `TestListener_BurstDropsExcess` | overflowing connCh drops, no panic |
| `TestNegotiation_DeflateAccepted` | server advertises permessage-deflate |
| `TestDial_SubprotocolMismatch` | client offers ["c2.v1"], server doesn't → typed error |
| `TestRouterIntegration_FallbackToWS` | composes with `c2/transport.Router` |
| `TestHandler_MountOnExternalServer` | handler mounted on external `http.Server` works |

Approx 13 tests. All deterministic, all host-runnable.

## Plan of commits (4 small, not one mega)

1. **`refactor(c2/transport): extract UTLSDialer helper`**
   - Pure refactor of `NewUTLS` → exports `UTLSDialer` for reuse.
   - Zero behaviour change. Existing tests unchanged.
   - Commit + push immediately.

2. **`feat(c2/transport/websocket): M3 — WS listener + dialer with uTLS composition`**
   - All new package source: `doc.go`, `dial.go`, `listen.go`,
     `dial_test.go`, `listen_test.go`,
     `router_integration_test.go`.
   - `go.mod`/`go.sum` add `coder/websocket` pinned version.
   - `/simplify` reviewed (reuse / quality / efficiency).
   - Commit + push.

3. **`docs(c2): websocket tech-md + mitre + README`**
   - `docs/techniques/c2/websocket.md` (minimal frontmatter —
     `package:` only, lesson from docgen CI fix).
   - `docs/mitre.md` row addition (T1071, T1090.004).
   - `README.md` Packages table row.
   - Commit + push.

4. **`docs(.dev): M3 ✅ — WebSocket primitive closed`**
   - Tick M3 to ✅ with commit hash in
     `.dev/refactor-2026/maldev-primitives-roadmap.md`.
   - Bump `reflects_commit` in front-matter.
   - Commit + push.

Per CLAUDE.md: every commit runs `go build ./...`, every Go diff
runs `/simplify`, every API addition updates tech-md Examples +
Limitations in the SAME commit (commits #2 and #3 are paired but
land separately for diff readability — the tech-md draft is
maintained in step with #2's surface, just committed in #3).

## Risks + mitigations

| Risk | Mitigation |
|---|---|
| `coder/websocket` API breaking | Pin exact version in go.mod. Lib is mature. |
| uTLS + HTTP/1.1 upgrade interaction breaks | `TestRouterIntegration_FallbackToWS` exercises end-to-end uTLS+WS; if it breaks, fall back to `WithTLSConfig` pure. |
| Refactor #1 breaks existing TLS tests | Isolated commit; `go test ./c2/transport/...` is the gate. Trivial rollback. |
| `connCh` overflow silent drop | Pinned in `TestListener_BurstDropsExcess`; documented in tech-md Limitations. |
| Docgen frontmatter rejection | Tech-md uses minimal frontmatter (`package:` only). Saved to memory `feedback_docgen_frontmatter`. |

## Out of scope (intentional)

- WS-over-HTTP/2 (RFC 8441) — marginal OPSEC value, large fork
  cost, easier-to-fingerprint NPN/ALPN. YAGNI.
- Applicative auth (bearer token, mTLS gating) — wraith teamserver
  concern, not a primitive concern.
- Multi-path single-listener — operator uses `Handler()` +
  `http.ServeMux` for this.

## Companion docs

After implementation, tech-md lives at
`docs/techniques/c2/websocket.md` following the 4-layer pedagogy
pattern (TL;DR table → primer → mermaid flow → narrated examples →
OPSEC + MITRE → composability → limitations → see-also). Per
`feedback-techmd-pedagogy` memory.
