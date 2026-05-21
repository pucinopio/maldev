---
title: license-manager — implementation progress
last_reviewed: 2026-05-21
reflects_commit: HEAD
---

# Status — license-manager TUI complete (Phases 1..4)

All four TUI phases shipped on top of the v0.161.0 backend:

- **Phase 1** — runnable foundation (rootModel, chrome, dashboard, passphrase prompt, onboarding wizard)
- **Phase 2** — Licenses table + 6 simple screens (Issuers, Recipients, Identities, Revocation, Audit, Settings) + minimal overlays (Confirm, Error, Input, Revoke)
- **Phase 2.5** — composable widget system, responsive layout, mouse support (a978fdc..eeb3c5f)
- **Phase 3** — 8-step new-license wizard + probe drawer + QR/filepicker overlays (d61aea3)
- **Phase 4** — Servers screen + httpsrv.Bundle event fan-in + live log streaming + dashboard server counter (this commit)

Tagged v0.162.0.

# Status — license-manager backend M1 complete

All 25 tasks of `docs/superpowers/plans/2026-05-20-license-manager-backend.md` shipped to master.

- [x] T1 — Scaffold + deps (entgo.io/ent v0.13.1, modernc.org/sqlite v1.34.4, google/uuid)
- [x] T2 — crypto/KEK (Argon2id-derived)
- [x] T3 — crypto Wrap/Unwrap/Canary (ChaCha20-Poly1305)
- [x] T4 — ENT schemas (10 entities)
- [x] T5 — ENT codegen + Store wrapper
- [x] T6 — Probe agent source + cross-compiled binaries (5 OS/arch)
- [x] T7 — Probe embed.go + ServeAgent
- [x] T8 — AuditService + SettingsService
- [x] T9 — IssuerService
- [x] T10 — IdentityService
- [x] T11 — RecipientService
- [x] T12 — TOTPService
- [x] T13 — ProbeService (token lifecycle + subscriber channel)
- [x] T14 — LicenseService (Issue/ReIssue/List/Get/Inspect/Import/Export/HashFile)
- [x] T15 — RevokeService (Revoke/Unrevoke/CRL publication)
- [x] T16 — Services bundle + withTx helper
- [x] T17 — httpsrv.Server interface + Status + Event
- [x] T18 — RevocationServer
- [x] T19 — HeartbeatServer
- [x] T20 — ProbeServer
- [x] T21 — Bundle (MergedEvents + StopAll)
- [x] T22 — cmd/license-manager boot loader (passphrase cascade + --no-tui)
- [x] T23 — Integration test (TestE2E_FullPipeline)
- [x] T24 — Docs (concepts.md + workflow.md + configuration.md + SUMMARY + README)
- [x] T25 — /simplify sweep + tracker + tag

## Sweep (T25)

6 fixes applied:
1. ReIssue atomicity — supersede now in a follow-up tx
2. Admin token constant-time compare
3. SecureZero dedup (8 inline wipe loops → cleanup/memory.SecureZero)
4. Unused `*LicenseService` param removed from NewRevokeService
5. Dead `var _ = bytes.Equal` suppressor removed
6. PublishSignedList cache (invalidated on Revoke/Unrevoke)

## Verification

- `go build ./...` — clean
- `go test ./internal/manager/...` — 40+ tests PASS (race-clean)
- `cmd/license-manager --no-tui` boots fresh DB + reopens existing DB + rejects wrong passphrase
- TestE2E_FullPipeline — full pipeline (issue + verify + revoke + CRL serve)
- CI (build + docs + mdbook + mermaid-check) — green on each commit

## Out-of-scope (backlog → tui)

See [backlog.md](backlog.md).
