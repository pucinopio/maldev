---
title: license/ package — implementation progress
last_reviewed: 2026-05-20
reflects_commit: HEAD
---

# Status — license/ v1 complete

All 21 tasks of `docs/superpowers/plans/2026-05-20-license-package.md` shipped on branch `feat/license-package`.

- [x] Task 1 — scaffold + doc.go
- [x] Task 2 — canonical/ deterministic JSON
- [x] Task 3 — errors + clock + hash domain tags
- [x] Task 4 — Ed25519 keys + PEM marshal/parse + Trusted
- [x] Task 5 — License type + Issue/New/Inspect
- [x] Task 6 — Verify (offline: signature/time/audience/issuer)
- [x] Task 7 — Bindings (machine/password argon2id/custom + extensible verifier)
- [x] Task 8 — hostid/ cross-platform fingerprint (Windows/Linux/Darwin)
- [x] Task 9 — State file HMAC + clock-rollback detection
- [x] Task 10 — Binary + identity pinning + identity/ sub-package + gen-identity tool
- [x] Task 11 — revoke/ list + sources (HTTP/File/Embed/Multi/custom) + monotonic cache
- [x] Task 12 — Verify wired with revocation
- [x] Task 13 — heartbeat/ client + signed reply with nonce echo, wired into Verify
- [x] Task 14 — seal/ X25519 + ChaCha20-Poly1305 sealed payload
- [x] Task 15 — ntp/ SNTPv4 cross-check (soft + strict)
- [x] Task 16 — server/ http.Handlers + FileStore + StaticLicenseStore
- [x] Task 17 — E2E harness `cmd/license-test`
- [x] Task 18 — Adversarial tests (bit-flip, oversize, key swap, mutation sweep)
- [x] Task 19 — Documentation (tech-md, workflow, threat-model, README, mitre, SUMMARY)
- [x] Task 20 — VM-tagged tests (hostid, hash stability, identity pinning)
- [x] Task 21 — Final sweep + this tracker

## Verification

- `go build ./license/...` — clean
- `GOOS=linux/darwin/windows go build ./license/...` — clean cross-platform
- `go test -race ./license/...` — 10 packages PASS, 0 FAIL
- `cmd/license-test` — exit 0 (`license-test: PASS`)
- Adversarial mutation sweep — 100/100 mutations rejected

## Out-of-scope (deferred to v2)

See [backlog.md](backlog.md).
