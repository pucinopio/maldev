# `04-reissue` — supersede a licence with a new validity window

> Re-issues an existing licence, extending validity by 90 days,
> adding a feature, and replacing the payload. Empty fields in
> `ReIssueOptions` inherit from the original — operator overrides
> only what changes. The original is marked `superseded`; both
> PEMs still verify, but a chain walk surfaces the lineage.

## What it demonstrates

- [`LicenseService.ReIssue`](../../../internal/manager/service/license.go)
  — nil/zero option fields fall back to the original's values.
- [`LicenseService.GetChain`](../../../internal/manager/service/license.go)
  — walks `ReplacesLicenseID` backwards (parents) and queries
  rows whose `ReplacesLicenseID == this.ID` forward (successors).
- Status `superseded` semantics — the audit trail keeps the old
  PEM (still cryptographically valid), the operator surfaces the
  newest one to deployments.

## Run + test

```bash
go run ./examples/license-manager/04-reissue
go test ./examples/license-manager/04-reissue
```

## TUI walkthrough

1. Licences screen → cursor on the row → `[e]` opens the
   re-issue wizard pre-populated from the original.
2. Step 5 (Validity): change the end date.
3. Step 6 (FreeFields): add features.
4. Step 8 (Review): emit. The Chain tab `[C]` on the new row
   shows the parent.
