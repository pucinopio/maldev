---
title: license-manager — v2+ backlog
last_reviewed: 2026-05-21
---

# license-manager backlog (post-M1)

## TUI layer (next milestone)
- ~~Awaits design handoff~~ — shipped in v0.162.0 (Phases 1..4)

### TUI polish backlog (post-v0.162.0)
- [ ] **Tab / Shift-Tab cycles views** — currently only `1-9` number keys route between views. `Tab` is more discoverable.
- [ ] **`--no-tui` on fresh DB should auto-bootstrap from env** — today it still launches the bubbletea onboarding TUI even with `--no-tui`, which hangs in non-TTY shells (CI, scripts). Skip TUI when `MALDEV_MGR_PASSPHRASE` + `MALDEV_MGR_OPERATOR` are set.
- [ ] **Env var name harmonisation** — `MALDEV_MGR_PASSPHRASE` vs the rest of the codebase using `MALDEV_LICENSE_PASSPHRASE`. Pick one and alias.

### v0.162.1 — E2E workflow coverage expansion (2026-05-21)

- 64 tests in `internal/manager/tui` (up from 28 at v0.162.0)
- 45.5% statement coverage on the tui package (uncovered = lipgloss styling in View() funcs)
- Three onboarding bugs fixed + guarded against regression:
  1. Enter on first field advances focus instead of validating prematurely
  2. Global `1`-`9` tab routing gated on `SessionReady` (digits reach inputs during onboarding)
  3. `OnboardingDoneMsg` now persists KEK + canary + first issuer to the DB (was a stub before)
- Coverage now spans every overlay (confirm/input/revoke/qr/filepicker/probe), the 8-step wizard transitions, the dashboard snapshot data flow, all 9 view routes, responsive layout at 4 window sizes, and the licenses screen keybind matrix (filter/search/detail/revoke/new)

## Backend refactoring (nice-to-have)

- [ ] **Extract `baseServer` for httpsrv** — Revocation/Heartbeat/Probe share ~120 lines of identical Start/Stop/Status/emit lifecycle code. An embedded `baseServer` struct would remove the duplication. Deferred from /simplify sweep (substantial refactor, not blocking).
- [ ] **Type `ListFilter.Status` as `licenseent.Status`** — currently a raw string; a caller typo silently returns empty.
- [ ] **Rename `GetServerConfig` → `ServerConfig`** — drops Go `Get-` prefix convention violation.
- [ ] **Probe subscriber leak on timeout** — `ProbeService.Subscribe` channels are cleaned up on consume but not on token expiry. Add a reaper goroutine that closes orphan channels when their token expires.
- [ ] **`gen/main.go` AgentResult missing `CPUBrand`** — the agent doesn't populate the field that the main `probe.AgentResult` declares. Sync.

## Operational features
- [ ] OS keystore integration (DPAPI / Keychain / libsecret) for passphrase resolution
- [ ] Stateful seat counter (floating licenses) — depends on stateful server in `license/`
- [ ] Push webhooks for downstream backends
- [ ] License-chain graph visualisation
- [ ] Bulk operations CLI (alternative to TUI for scripting)
- [ ] Encrypted backup export (signed manifest + tar.gz)

## Documentation
- [ ] FAQ page (`docs/license-manager/faq.md`) similar to `docs/license/faq.md`
- [ ] Threat model page (`docs/license-manager/threat-model.md`)
- [ ] Sequence diagram of the full lifecycle (boot → operate → quit)
