# M3 — WebSocket transport primitive — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Sequential tasks, no parallelism benefit. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `c2/transport/websocket` — dial-side `Transport` + accept-side `Listener` for WebSocket C2 traffic, composable with `Router` and `NewUTLS`.

**Architecture:** Sibling sub-package to `c2/transport/namedpipe`. Single sub-package holds both halves. Reuses a freshly-extracted `transport.UTLSDialer` helper for JA3 spoofing on the WS handshake.

**Tech Stack:** Go 1.22+, `github.com/coder/websocket` pinned (resolved at `go get` time), existing `github.com/refraction-networking/utls v1.6.7`.

**Companion docs:**
- Spec (authoritative): `docs/superpowers/specs/2026-05-19-c2-transport-websocket-design.md`
- Progress tracker: `.dev/refactor-2026/m3-websocket-progress.md`
- CLAUDE.md per-commit discipline applies (build clean + /simplify + tech-md sync).

---

## Task 1 — Extract `UTLSDialer` (Commit #1)

**Files:** Create `c2/transport/utls_dialer.go`. Modify `c2/transport/ja3.go` (rewire `UTLS.Connect`).

- [ ] **1.1** Write `c2/transport/utls_dialer.go`. Public API:
  ```go
  func UTLSDialer(cfg *utls.Config, hello utls.ClientHelloID, timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error)
  ```
  Closure: TCP dial via `net.Dialer{Timeout}` → clone cfg (fill `ServerName` from addr if empty) → `utls.UClient` → `HandshakeContext`. Wraps errors with `fmt.Errorf(... %w)`.

- [ ] **1.2** Replace `UTLS.Connect` body in `ja3.go:121-163` with a call to `UTLSDialer(...)(ctx, "tcp", t.address)`. Same external behaviour; internal TCP+TLS sequence now factored.

- [ ] **1.3** `go build ./... && go test -count=1 ./c2/transport/...` — all existing tests must remain green. No new test required (existing `TestNewUTLS_Options` covers the public API).

- [ ] **1.4** /simplify on the diff (light review).

- [ ] **1.5** Commit:
  ```
  refactor(c2/transport): extract UTLSDialer for reuse by WebSocket primitive
  ```
  Push. Note SHA in tracker.

- [ ] **1.6** Tick Step 1 in `.dev/refactor-2026/m3-websocket-progress.md`, bump `reflects_commit`.

---

## Task 2 — `c2/transport/websocket` package (Commit #2)

**Files (all NEW):**
- `c2/transport/websocket/doc.go`
- `c2/transport/websocket/dial.go`
- `c2/transport/websocket/listen.go`
- `c2/transport/websocket/dial_test.go`
- `c2/transport/websocket/listen_test.go`
- `c2/transport/websocket/router_integration_test.go`
- `go.mod` / `go.sum` (add `coder/websocket`)

Full source for each file is captured in the spec's "Internal types" and "Test matrix" sections; this plan only enumerates the structural steps.

- [ ] **2.1** `go get github.com/coder/websocket@latest && go mod tidy`. Pin the exact resolved version.

- [ ] **2.2** Write `doc.go` — package overview, MITRE T1071 + T1090.004, detection moderate, cross-platform, link to tech-md.

- [ ] **2.3** Write `dial.go` — `Transport` struct + `NewWebSocket` + 6 `DialOption` builders (`WithSubprotocols`, `WithHeader`, `WithTLSConfig`, `WithUTLSConfig`, `WithCompression`, `WithDialTimeout`). `Connect` plumbs the dial config into `cws.Dial`. uTLS path builds `&http.Transport{DialTLSContext: transport.UTLSDialer(...)}`. NetConn wrapper for byte-stream Read/Write. Close releases mutex BEFORE close-frame (Router /simplify lesson).

- [ ] **2.4** Write `listen.go` — `Listener` struct + `listenerConfig` + 4 `ListenerOption` builders (`WithTLS`, `WithAcceptedSubprotocols`, `WithOriginPatterns`, `WithServerCompression`). Three public entry points:
  - `NewServer(opts ...) (http.Handler, *Listener)` — building blocks.
  - `Handler(opts ...) http.Handler` — sugar for the handler half.
  - `NewListener(addr, path, opts ...) (*Listener, error)` — stand-alone server.
  Handler does `cws.Accept` → `cws.NetConn` → non-blocking `select { case connCh <- c: ; default: c.Close() }`. `Close()` calls `srv.Shutdown(ctx)` with 3s grace.
  Add `var _ transport.Listener = (*Listener)(nil)` assertion.

- [ ] **2.5** Write `dial_test.go` — 7 tests against httptest WS servers:
  - `TestDial_RoundTrip` (echo server)
  - `TestDial_ConnectionRefused` (port 1)
  - `TestDial_UpgradeRejected` (http 200 instead of 101)
  - `TestDial_CtxCancel` (pre-cancelled ctx)
  - `TestRead_NormalClosure` (server closes 1000 → io.EOF)
  - `TestClose_Idempotent` (Close twice = no panic)
  - `TestDial_SubprotocolMismatch` (offered != accepted)

- [ ] **2.6** Write `listen_test.go` — 4 tests:
  - `TestListener_AcceptRoundTrip` (Accept + echo on `NewListener`)
  - `TestListener_AcceptCtxCancel` (ctx cancel unblocks Accept)
  - `TestHandler_MountOnExternalServer` (handler in user's ServeMux, decoy path co-hosted)
  - `TestListener_BurstDropsExcess` (20 dialers vs buffer-16, no deadlock, ≥16 accepted)

- [ ] **2.7** Write `router_integration_test.go` — `TestRouterIntegration_FallbackToWS`:
  1. Start `NewListener("127.0.0.1:0", "/")`.
  2. Build `Router` with `[deadTransport{}, wstransport.NewWebSocket("ws://"+lst.Addr())]`, `MaxAttempts: 1`, `InitialBackoff: 1ms`.
  3. `r.Connect(ctx)` — must succeed; `r.ActiveIndex() == 1`.
  4. Round-trip a small payload via `r.Write` / Server-side `Accept` / `conn.Read`.
  Use a real `deadTransport` stub satisfying `transport.Transport`.

- [ ] **2.8** `go build ./... && go test -count=1 ./c2/transport/...` — all 12 new tests green, no regression on existing tests.

- [ ] **2.9** /simplify on the new package — full 3-agent review (reuse / quality / efficiency). Fix issues inline.

- [ ] **2.10** Commit:
  ```
  feat(c2/transport/websocket): M3 — WS listener + dialer with uTLS composition
  ```
  Push. Note SHA in tracker.

---

## Task 3 — docs (Commit #3)

**Files:** Create `docs/techniques/c2/websocket.md`. Modify `docs/mitre.md`, `README.md`.

- [ ] **3.1** Write `docs/techniques/c2/websocket.md` — 4-layer pedagogy (per `feedback-techmd-pedagogy`):
  1. **Frontmatter** — minimal: only `package: github.com/oioio-space/maldev/c2/transport/websocket`. No `last_reviewed`/`reflects_commit` (docgen check rule).
  2. **TL;DR table** — "you want to… / use / notes".
  3. **Primer** — what WS C2 is, why over HTTPS, blending property.
  4. **How It Works** — mermaid flowchart of dial + accept.
  5. **API → godoc** — link.
  6. **Examples** — Simple (ws://), Composed (wss + uTLS + custom headers), Advanced (co-host with decoy via Handler), Complex (nested in Router fallback).
  7. **OPSEC & Detection** — origin-permissive default callout in **bold**, JA3 fingerprint pairing with uTLS, traffic-shape considerations.
  8. **MITRE ATT&CK** — T1071, T1090.004 rows.
  9. **Composability** section — interplay with Router and UTLSDialer.
  10. **Limitations** — no HTTP/2 WS, no applicative auth, single-path standalone.
  11. **See also** — Router multi-channel, UTLS transport, OPSEC paths.

- [ ] **3.2** Update `docs/mitre.md` — add rows for T1071 → `c2/transport/websocket` and T1090.004 → same (uTLS composition).

- [ ] **3.3** Update `README.md` Packages table — add `c2/transport/websocket` row with one-line description.

- [ ] **3.4** `go run ./internal/tools/docgen --check-template` — must pass.

- [ ] **3.5** Commit:
  ```
  docs(c2): websocket tech-md + mitre + README
  ```
  Push.

---

## Task 4 — Roadmap tick (Commit #4)

**Files:** Modify `.dev/refactor-2026/maldev-primitives-roadmap.md`, `.dev/refactor-2026/m3-websocket-progress.md`.

- [ ] **4.1** In `maldev-primitives-roadmap.md`:
  - Change `M3 | 🟦` to `M3 | ✅ | <commit-#2-hash>` with one-line scope summary (mirror M4/M5/M8 style).
  - Bump status front-matter: `status: execution underway — M3 + M4 + M5 + M8 closed`.
  - Bump `reflects_commit` front-matter to the new SHA (the commit being created).

- [ ] **4.2** In `m3-websocket-progress.md`:
  - Tick all remaining boxes.
  - Update front-matter `status: closed`, `reflects_commit: <commit-#4-hash placeholder, will be the final SHA>`.

- [ ] **4.3** Commit:
  ```
  docs(.dev): M3 ✅ — WebSocket primitive closed
  ```
  Push.

---

## Verification gate (after Commit #4)

- [ ] `gh run list --limit 3` — last 3 runs on master all green.
- [ ] `gh repo view oioio-space/maldev --json description` still shows the simplified description (no regression).
- [ ] Tracker `m3-websocket-progress.md` has every box ticked.

If any of the above fails, do NOT close M3 — investigate and produce a fix commit.

## Resumption protocol (cross-machine / crash recovery)

1. `git pull origin master`.
2. Read this plan.
3. Read `.dev/refactor-2026/m3-websocket-progress.md` — find first unticked box.
4. Re-read the spec for the relevant Task before resuming.
5. Continue from that step.
