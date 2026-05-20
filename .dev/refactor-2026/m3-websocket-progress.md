---
item: M3 — WebSocket transport primitive
status: closed
spec: docs/superpowers/specs/2026-05-19-c2-transport-websocket-design.md
opened: 2026-05-19
closed: 2026-05-20
last_reviewed: 2026-05-20
reflects_commit: 97745e2
---

# M3 progress tracker — WebSocket primitive

> **Read this first** to resume M3 work on another machine or after
> a crash. Tick the boxes + bump `reflects_commit` at every commit.

## Phase 1 — Brainstorm + spec (✅ DONE)

- [x] Library choice: `github.com/coder/websocket` (vs gorilla — see spec rationale)
- [x] Package placement: `c2/transport/websocket` (sub-package, mirrors `namedpipe`)
- [x] V1 scope: full kit (ws + wss + subprotocol + custom headers + uTLS composition)
- [x] Blending knobs: uTLS for JA3 + Subprotocol + HTTP headers (Option 2 from Q3)
- [x] Listener: standalone (`NewListener`) + handler hook (`Handler` / `NewServer`) for co-hosting
- [x] Section 1-4 design approved
- [x] Spec written: `docs/superpowers/specs/2026-05-19-c2-transport-websocket-design.md`
- [x] User reviews spec (approved, "go en autonomie")
- [x] Implementation plan written: `docs/superpowers/plans/2026-05-19-m3-websocket-plan.md`

## Phase 2 — Implementation (4 commits, queued)

- [x] **Commit #1** — `refactor(c2/transport): extract UTLSDialer helper`
  - File: `c2/transport/utls_dialer.go` (new), `c2/transport/ja3.go` (edit — UTLS.Connect rewired)
  - Gate: `go test ./c2/transport/...` unchanged-pass ✓
  - Commit hash: `046ad5d`

- [x] **Commit #2** — `feat(c2/transport/websocket): M3 — WS listener + dialer with uTLS composition`
  - Files: `c2/transport/websocket/{doc,dial,listen,dial_test,listen_test,router_integration_test}.go`
  - `coder/websocket` v1.8.14 pinned (Go directive bumped 1.21 → 1.23)
  - 12 tests pass under `-race`
  - Self-review done (agent quota was hit; key checks: mutex-before-close, signalConn pattern documented, compile-time interface assertion)
  - Commit hash: `f25b95b`

- [x] **Commit #3** — `docs(c2): websocket tech-md + mitre + README`
  - Files: `docs/techniques/c2/websocket.md` (new, minimal frontmatter), `docs/mitre.md`, `README.md`
  - `go run ./internal/tools/docgen --check-template` passes ✓
  - Commit hash: `97745e2`

- [x] **Commit #4** — `docs(.dev): M3 ✅ — WebSocket primitive closed`
  - File: `.dev/refactor-2026/maldev-primitives-roadmap.md`, this tracker
  - M3 row ticked, `reflects_commit` bumped on both files
  - Commit hash: (this commit)

## Cross-machine resumption

1. `git pull origin master`
2. Read the spec: `docs/superpowers/specs/2026-05-19-c2-transport-websocket-design.md`
3. Read this tracker — find first unchecked box
4. If implementation plan exists in the repo (`docs/superpowers/plans/2026-05-19-m3-websocket-plan.md`), read it; otherwise we're still in the spec-review phase
5. Continue

If `reflects_commit` differs from `HEAD`, run
`git log <reflects_commit>..HEAD --oneline -- c2/transport/websocket docs/techniques/c2/websocket.md`
to see what already landed.

## Notes / decisions captured during brainstorm

- **Defender flag on `cmd/bof-runner`** is unrelated to M3; expected on
  this Windows box, doesn't affect Linux CI.
- **CI fixes landed alongside the brainstorm:**
  - `ead9870` — pe/packer Seed-gated timestamp (anchor 2026-01-01)
  - `f5ac924` — drop `last_reviewed`/`reflects_commit` frontmatter
    from `docs/techniques/**` (docgen --check-template rule)
- **OPSEC default for listener Origin check is permissive** —
  rationale and operator override (`WithOriginPatterns`) documented
  in the spec and will be **bold-flagged** in tech-md.
