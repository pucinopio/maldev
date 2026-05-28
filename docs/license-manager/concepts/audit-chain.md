---
package: github.com/oioio-space/maldev/internal/manager/service
---

# Audit chain

> Every mutating service method writes the business row AND an
> `AuditEvent` in the **same SQLite transaction**. The audit log
> is immutable, sequential, and survives every mutation —
> including `License.Delete` which removes the licence row but
> keeps the `license.delete` event so the forensic trail is
> intact.

## What gets audited

Any state-changing operation. Read-only methods (`Get`, `List`,
`Inspect`, `ExportPublic`) do NOT generate events — they would
flood the table without adding accountability.

Sample of audited `kind` values:

| `kind` | Service method | Target |
|---|---|---|
| `issuer.create` | `IssuerService.Generate` | `Issuer.ID` |
| `issuer.import` | `IssuerService.Import` | `Issuer.ID` |
| `issuer.set_active` | `IssuerService.SetActive` | `Issuer.ID` |
| `issuer.delete` | `IssuerService.Delete` | `Issuer.ID` |
| `license.issue` | `LicenseService.Issue` | `License.ID` |
| `license.import` | `LicenseService.Import` | `License.ID` |
| `license.reissue` | `LicenseService.ReIssue` | `License.ID` (the new one) |
| `license.supersede` | inside `ReIssue` (atomic with `reissue`) | `License.ID` (the old one, status→superseded) |
| `license.revoke` | `RevokeService.Revoke` | `License.ID` |
| `license.unrevoke` | `RevokeService.Unrevoke` | `License.ID` |
| `license.delete` | `LicenseService.Delete` | `License.ID` |
| `identity.create` | `IdentityService.Create` | `Identity.ID` |
| `identity.regenerate` | `IdentityService.Regenerate` | `Identity.ID` |
| `probe.token_created` | `ProbeService.NewToken` | `ProbeToken.ID` |
| `probe.result` | `ProbeService.ConsumeToken` | `ProbeToken.ID` |

## Event shape

```json
{
  "kind":        "license.issue",
  "target_kind": "License",
  "target_id":   "<uuid>",
  "actor":       "mathieu",
  "payload":     { "subject": "alice@example.com", "not_after": "2026-12-31T00:00:00Z" },
  "created_at":  "2026-05-20T14:00:00Z"
}
```

The `actor` is the value the caller passes to the service
method. The wizard hardcodes `"operator"`; CLI examples pass
`"demo-operator"`. Production setups should pass the operator's
real identity (e.g. `os.Getenv("USER") + "@" + hostname`).

`payload` is `map[string]any` — small contextual bag the writer
chooses. Not indexed. Useful for forensic reconstruction (e.g.
revoke audit carries the reason, delete audit keeps the old
UUID/subject/status).

## Transactional guarantee

```go
return withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
    // 1. business row
    row, err := tx.License.Create()...Save(ctx)
    if err != nil { return err }
    // 2. audit event (same tx → either both land or neither)
    return svc.audit.AppendTx(ctx, tx, "license.issue", req.Actor, ...)
})
```

If either step fails, the SQLite transaction rolls back. There
is no way to issue, revoke, or pivot without a matching audit
event — barring a bypass of `svc.*` and a direct write to the
DB, which is a different threat-model entirely.

## TUI surface

- **Audit tab `[9]`** — chronological view of every event with
  filters by kind, actor, target.
- **Licence detail → Audit tab `[A]`** — the events for one
  licence only (uses `audit.ListForTarget`).

## Why deletes keep their audit

`LicenseService.Delete` removes the licence row + cascades
`Revocation` + `TOTPSecret`. The matching `license.delete`
event keeps `license_uuid`, `subject`, and the old `status` in
its payload so even after the row disappears the forensic
trail says "UUID X for subject Y was active-then-deleted by
operator Z at time T".

## Tested in

Every example in
[`examples/license-manager/`](../../../examples/license-manager/)
exercises at least one audited operation. The
[04-reissue example](../../../examples/license-manager/04-reissue/)
specifically demonstrates the supersession chain: the original
licence gets `license.supersede`, the new one
`license.reissue`, both in the same transaction.
