# `01-issue-basic` — smallest possible license issuance

> Boot an in-memory license-manager, create one Ed25519 signing
> key (issuer), sign one licence for a fictional subject, export
> the PEM to stdout, and verify it round-trips through the
> standalone `license/` package.

## What it demonstrates

- [`service.Services`](../../../internal/manager/service/services.go)
  — the single entry point a TUI / API / CLI consumer talks to.
- [`IssuerService.Generate`](../../../internal/manager/service/issuer.go)
  — creates a fresh Ed25519 key pair, encrypts the private half
  under the KEK, persists the issuer row, and writes an
  `issuer.create` audit event in one transaction.
- [`LicenseService.Issue`](../../../internal/manager/service/license.go)
  — builds the wire format, signs with the active issuer, persists
  the row with denormalised metadata, and audits.
- [`license.Verify`](../../../license/verify.go) — the standalone
  verifier the licensed binary will run at startup; this example
  uses it as a self-check so the operator knows the PEM is good
  before handing it off.

## Concepts

- [Issuer](../../../docs/license-manager/concepts/issuer.md) — what
  a signing key is and why every licence has exactly one.

## Build & run

```bash
go run ./examples/license-manager/01-issue-basic
```

No flags. The example uses fixed parameters so it is fully
deterministic — a useful smoke test of the whole pipeline.

To capture the licence PEM to disk:

```bash
go run ./examples/license-manager/01-issue-basic > /tmp/demo.license
```

## Expected output

```
[ok] services up (in-memory SQLite, KEK derived)
[ok] issuer "lab" created (key-id: demo-2026-q2)
[ok] licence issued for subject "alice@example.com" (uuid: <uuid>)
[ok] verify round-trip green
-----BEGIN MALDEV LICENSE-----
<base64 lines>
-----END MALDEV LICENSE-----
```

stderr carries the status lines; stdout is the PEM only, so the
example can be piped to a file or another tool without parsing.

## Test it

```bash
go test ./examples/license-manager/01-issue-basic
```

The test runs the exact same code path against an in-memory
store, asserts the licence verifies against the issuer's public
key, and checks that the audit event was written.

## Equivalent TUI walkthrough

In the TUI, this example corresponds to:

1. First-launch wizard creates the initial issuer.
2. Operator presses **`2`** to switch to the **Licences** screen.
3. Operator presses **`n`** to open the new-licence wizard.
4. Wizard steps: identity (auto-picked), recipient (skip),
   machine (skip), binary (skip), validity (defaults), free
   fields (subject = `alice@example.com`), TOTP (skip), review
   (Enter).
5. The PEM is shown in the **PEM** detail tab of the licence row.
