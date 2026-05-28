# `03-revoke-and-crl` — revoke a licence, publish the CRL, prove a verifier rejects

> Issues a licence, revokes it with a reason, signs and publishes
> the CRL (the same payload `/revoked.pem` serves), and shows a
> verifier armed with the CRL rejects the revoked licence — while
> a verifier WITHOUT the CRL still accepts it (signature is
> intact). The CRL is the source of truth, not the licence itself.

## What it demonstrates

- [`RevokeService.Revoke`](../../../internal/manager/service/revoke.go)
  — atomic status flip + Revocation row + audit event.
- [`RevokeService.PublishSignedList`](../../../internal/manager/service/revoke.go)
  — fetches every revoked licence, builds a `revoke.List`, signs
  it with the active issuer, and returns the PEM. Cached for
  `validFor/2`; invalidated by every Revoke/Unrevoke.
- [`license.WithRevocation`](../../../license/verify_options.go)
  + [`revoke.EmbedSource`](../../../license/revoke/source.go) —
  the wiring a deployed binary uses to consult the CRL.

## Concepts

- [CRL](../../../docs/license-manager/concepts/crl.md) *(coming)*

## Run + test

```bash
go run ./examples/license-manager/03-revoke-and-crl    # stderr=steps, stdout=CRL PEM
go test ./examples/license-manager/03-revoke-and-crl
```

## TUI walkthrough

1. Licences screen → cursor on the row → `[x]` opens the revoke
   overlay. Type the reason → enter.
2. Revocation screen (tab `[6]`) shows the row.
3. Start the revocation server from Servers (tab `[7]`); `GET
   /revoked.pem` serves the same PEM this example prints.
