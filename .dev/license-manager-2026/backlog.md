---
title: license-manager — v2+ backlog
last_reviewed: 2026-05-21
---

# license-manager backlog (post-M1)

## TUI layer (next milestone)
- Awaits design handoff from Claude Design (prompt in `docs/superpowers/specs/2026-05-20-license-manager-backend-design.md`)
- Will live under `cmd/license-manager/` (bubbletea models) and `internal/manager/tui/`

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
