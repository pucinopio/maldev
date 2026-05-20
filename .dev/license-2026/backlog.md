---
title: license/ — v2+ backlog
last_reviewed: 2026-05-20
---

# license/ backlog (post-v1)

- [ ] **P2-a — Stateful DB-backed server**
  SQLite optional, seat counter, audit log. Sub-package `license/server/sqlite`.

- [ ] **P2-b — CLI `cmd/license`**
  Wraps the public functions: `genkey`, `issue`, `verify`, `inspect`, `revoke`, `heartbeat-server`. ~150 LoC.

- [ ] **P2-c — COSE_Sign1 alternative format**
  CBOR ecosystem interop. Sub-package `license/cose`.

- [ ] **P2-d — HSM / PKCS#11 issuer-side signing**
  YubiKey / cloud-HSM-backed Issue. Sub-package `license/hsm`.

- [ ] **P2-e — Telemetry de-dup via `IdentitySHA256`**
  Reuse of `license/identity` outside license context (C2 dedup, etc.).

- [ ] **P2-f — Heartbeat with seat counter**
  Depends on P2-a (stateful server).

- [ ] **P2-g — VM test: `TestIdentityPinning_AfterPack`**
  End-to-end test pipe through `cmd/packer`. Currently the spec lists this — base infrastructure is present (Task 10 + Task 20), the packer-specific path is deferred until cmd/packer flags stabilise.

- [ ] **P2-h — `cleanup/memory` integration for password wipe**
  Wire `cleanup/memory.WipeString` over `WithPassword` and `IssueOptions.PrivateKey` after use.
