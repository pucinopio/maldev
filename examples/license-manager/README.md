# `examples/license-manager/` — License manager cookbook (runnable)

> Each subdirectory is a small, self-contained Go program that
> demonstrates one feature of the license-manager backend
> (`internal/manager/service.Services`). Every example is
> end-to-end testable: the same logic runs from `main()` for an
> operator at the shell, and from `main_test.go` against an
> in-memory store for CI.

The license-manager itself is documented in
[`docs/license-manager/`](../../docs/license-manager/) — start with
[`concepts.md`](../../docs/license-manager/concepts.md) for the
architecture, then come back here for working code.

## Feature index

| # | Example | What it shows | Concepts |
|---|---|---|---|
| 01 | [`01-issue-basic`](./01-issue-basic/) | Smallest possible issue → export → verify round-trip | [Issuer](../../docs/license-manager/concepts/issuer.md) |
| 02 | `02-issue-with-bindings` *(coming)* | Machine ID + password + TOTP binding chain | Bindings |
| 03 | `03-revoke-and-crl` *(coming)* | Mark a licence revoked, sign + publish the CRL | CRL |
| 04 | `04-reissue` *(coming)* | Supersede a licence with a new validity window | Audit chain |
| 05 | `05-hard-delete-roundtrip` *(coming)* | Export → delete → re-import without UUID conflict | — |
| 06 | `06-totp-secret` *(coming)* | Issue a TOTP binding, render QR + otpauth URI | TOTP |
| 07 | `07-sealed-payload` *(coming)* | Encrypt a JSON payload for one recipient only | Sealed payload |
| 08 | `08-identity-pin` *(coming)* | Bind a licence to a host identity SHA-256 | Identity pin |
| 09 | `09-import-and-verify` *(coming)* | Take a PEM from another instance and verify it | KEK passphrase |
| 10 | `10-http-servers` *(coming)* | Start the revocation / heartbeat / probe servers | — |
| 11 | `11-backup-and-restore` *(coming)* | Snapshot the encrypted DB and restore it elsewhere | — |

## How to run an example

Every example builds as a standalone Go binary:

```bash
# Build + run example 01
go run ./examples/license-manager/01-issue-basic
```

The output is a PEM-encoded licence printed to stdout, plus
human-readable status lines on stderr. Pipe stdout to a file to
keep the PEM:

```bash
go run ./examples/license-manager/01-issue-basic > /tmp/demo.license
```

## Test every example

Each example ships an `main_test.go` that runs the same scenario
against an in-memory SQLite store with assertions. CI green ⇒
the operator can copy the example as-is.

```bash
# Test one example
go test ./examples/license-manager/01-issue-basic

# Test them all
go test ./examples/license-manager/...
```

## What the examples DON'T do

- They never write to your real DB at `~/.config/license-manager/`.
  All state lives in an in-memory SQLite created on the fly.
- They never start an HTTP listener unless the example is the
  `10-http-servers` walkthrough, and that one binds to `:0`
  (ephemeral port).
- They never prompt for a passphrase. A fixed `"testpass"` is used
  so the example is deterministic — production callers should
  supply the passphrase via the cascade documented in
  [`concepts.md`](../../docs/license-manager/concepts.md#startup-cycle).

## Layout convention

Each example follows the same skeleton:

```
NN-feature-name/
├── README.md       # what + why + expected output
├── main.go         # runnable CLI, single file
└── main_test.go    # E2E test against an in-memory manager
```

`main.go` typically exposes a `run(ctx, svc, args) error` function
that both `main()` and `main_test.go` call. That way the example
stays linear at the top level (no test scaffolding noise) while
the test re-uses the exact same code path the operator runs.
