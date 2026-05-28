# `05-hard-delete-roundtrip` — export → delete → re-import same UUID

> Issues a licence, captures the PEM, hard-deletes the row, then
> re-imports the PEM. The unique `license_uuid` constraint that
> blocked re-imports pre-delete is now freed; the same UUID lands
> back in the DB cleanly. This is the workflow for moving a
> licence between manager instances or rolling back a test issue.

## What it demonstrates

- [`LicenseService.Delete`](../../../internal/manager/service/license.go)
  — cascades `Revocation` (1:1) and `TOTPSecret` (1:N) inside one
  transaction, keeps the `license.delete` audit event with the
  old UUID/subject/status. Flushes the CRL cache when the deleted
  row was revoked.
- [`LicenseService.Import`](../../../internal/manager/service/license.go)
  — parses an external PEM (no signature check here — the
  operator imports licences they trust from a backup or another
  instance) and persists the row with the same UUID.

## Run + test

```bash
go run ./examples/license-manager/05-hard-delete-roundtrip
go test ./examples/license-manager/05-hard-delete-roundtrip
```

## TUI walkthrough

1. Licences screen → cursor on the row → `[E]` exports the PEM
   to disk, then `[D]` hard-deletes (danger confirm).
2. `[i]` opens the file picker → select the exported `.pem`.
3. The row re-appears with the original UUID.
