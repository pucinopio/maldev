# Runnable examples

This page indexes the runnable cookbook under
[`examples/license-manager/`](https://github.com/oioio-space/maldev/tree/master/examples/license-manager/).
Each entry is a standalone Go program — copy-paste, adapt to
your environment, run. Every example also ships a `main_test.go`
that runs the same scenario against an in-memory SQLite store,
so CI green ⇒ the example works.

## Why a separate examples tree

The [Cookbook](../workflow.md) prose pages explain the *why* of
each operation. The runnable examples here are the *what* — code
you can clone and execute without first reading the cookbook.

If you read the cookbook recipe and want the matching code, the
example is linked from each recipe.

If you found this page first, every example links back to the
concept page that explains the underlying notion.

## Catalogue

| # | Example | Concepts |
|---|---|---|
| 01 | [01-issue-basic](https://github.com/oioio-space/maldev/tree/master/examples/license-manager/01-issue-basic/) | [Issuer](../concepts/issuer.md) |
| 02 | [02-issue-with-bindings](https://github.com/oioio-space/maldev/tree/master/examples/license-manager/02-issue-with-bindings/) | [Bindings](../concepts/bindings.md) |
| 03 | [03-revoke-and-crl](https://github.com/oioio-space/maldev/tree/master/examples/license-manager/03-revoke-and-crl/) | [CRL](../concepts/crl.md) |
| 04 | [04-reissue](https://github.com/oioio-space/maldev/tree/master/examples/license-manager/04-reissue/) | [Audit chain](../concepts/audit-chain.md) |
| 05 | [05-hard-delete-roundtrip](https://github.com/oioio-space/maldev/tree/master/examples/license-manager/05-hard-delete-roundtrip/) | — |
| 06 | [06-totp-secret](https://github.com/oioio-space/maldev/tree/master/examples/license-manager/06-totp-secret/) | [Bindings](../concepts/bindings.md) |
| 09 | [09-import-and-verify](https://github.com/oioio-space/maldev/tree/master/examples/license-manager/09-import-and-verify/) | [KEK passphrase](../concepts/kek-passphrase.md) |

Examples 07 (sealed payload), 08 (identity pin), 10 (HTTP servers)
and 11 (backup / restore) ship as placeholders pending the backup
format spec and a fixture for the probe agent.

## How to run any of them

```bash
# One example
go run ./examples/license-manager/03-revoke-and-crl

# All examples (CI uses this)
go test ./examples/license-manager/...
```

The examples produce machine-friendly stdout (one PEM, one
otpauth URI, one CRL — never mixed) and human-friendly stderr
status lines. Pipe stdout to a file, read stderr in the
terminal.

## Skeleton

Every example follows the same shape:

```
NN-feature-name/
├── README.md       # what + why + expected output + TUI mapping
├── main.go         # runnable CLI calling a single run() helper
└── main_test.go    # E2E test against an in-memory manager
```

`main.go` exposes a `run(ctx, stdout, stderr) error` that
`main()` and `main_test.go` both call. This keeps the example
linear at the top level while the test re-uses the exact code
path the operator runs.
