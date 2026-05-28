# `02-issue-with-bindings` — machine + password + TOTP chain

> Issues a licence whose verification requires three pieces of
> evidence the licensed binary must collect at runtime: a machine
> id, a password the operator types, and a 30-second TOTP code.
> Verification fails if any one is missing or wrong.

## What it demonstrates

- [`service.BindingSpec`](../../../internal/manager/service/license.go)
  — the wizard's representation of one binding (type + values +
  optional Argon parameters for password bindings).
- The three binding kinds the licence framing supports:
  - **machine** — the binary must call `hostid.Composite()` and
    that value must match one of the stamped IDs.
  - **password** — the binary must collect a password from the
    user and the Argon2id derivation must match the stamped hash.
    Argon parameters travel with the binding, so re-tuning the
    cost on the issuer side is a non-breaking change.
  - **totp** — a 30-second one-time code generated from a shared
    secret the manager handed back at issue time (the licensee
    typically scans it as a QR code via the TUI's `[Q]` popup).
- [`license.Verify`](../../../license/verify.go) options
  (`WithMachineID`, `WithPassword`, `WithTOTPCode`) — how the
  licensed binary feeds evidence into the verifier.

## Concepts

- [Bindings](../../../docs/license-manager/concepts/bindings.md) *(coming)*
- [Argon preset](../../../docs/license-manager/concepts/argon-preset.md) *(coming)*

## Build & run

```bash
go run ./examples/license-manager/02-issue-with-bindings
```

## Expected output

```
[ok] services up
[ok] licence issued (uuid: …)
[ok] TOTP secret returned out-of-band: JBSWY3DPEHPK3PXP
[ok] verify GREEN with full evidence (features: [])
[ok] verify RED when password evidence missing (expected)
[ok] verify RED when machine doesn't match (expected)
-----BEGIN MALDEV LICENSE-----
…
-----END MALDEV LICENSE-----
```

Note the two negative paths: dropping a single piece of evidence
makes the verifier reject the licence even though the signature
is valid. The "all bindings must be satisfied" invariant is what
makes a licence machine- AND password-bound rather than
machine- OR password-bound.

## Test it

```bash
go test ./examples/license-manager/02-issue-with-bindings
```

## TUI walkthrough

In the TUI, this maps to:

1. `[n]` on the Licences screen → wizard.
2. Step 3 (Machine): type `host-alpha,host-beta` or pick from the
   probe drawer.
3. Step 4 (Binary): skip.
4. Step 5 (Validity): defaults.
5. Step 6 (FreeFields): subject = `alice@example.com`.
6. Step 7 (TOTP): toggle ON. The wizard generates the secret;
   the QR is reachable later via `[Q]` on the TOTP screen.
7. Step 8 (Review): confirm — the password binding is supplied
   inline by the wizard's password step (or via the example
   driver in code).
