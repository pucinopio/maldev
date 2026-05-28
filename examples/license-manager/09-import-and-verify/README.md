# `09-import-and-verify` — cross-instance handoff

> Simulates two manager instances. Instance A issues a licence
> and exports the issuer (public + private). Instance B imports
> the issuer + the licence PEM, verifies it via the standalone
> license/ package. The licence survives the handoff with its
> UUID intact and its signature still valid.

## What it demonstrates

- [`IssuerService.Import`](../../../internal/manager/service/issuer.go)
  — adopt an Ed25519 key generated outside the current manager.
- [`LicenseService.Import`](../../../internal/manager/service/license.go)
  — parse a PEM and persist the row with the same UUID. No
  signature check at import — the operator imports licences
  they trust from a backup or another instance.
- [`license.Verify`](../../../license/verify.go) — the deployed
  binary only ever needs the issuer's **public** key. The fact
  that instance B holds the private half (for further issuing)
  is orthogonal to verification.

## Run + test

```bash
go run ./examples/license-manager/09-import-and-verify
go test ./examples/license-manager/09-import-and-verify
```

## TUI walkthrough

1. Instance A — Issuers tab `[3]` → `[E]` export public key to
   disk. Licences tab → `[E]` export the .pem.
2. Move both files to instance B.
3. Instance B — Issuers tab → `[i]` import the public key.
   Licences tab → `[i]` import the .pem.
4. The licence shows up with its original UUID; the Chain tab
   confirms no parent / no successor on this instance.
