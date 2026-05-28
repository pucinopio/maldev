# `06-totp-secret` — standalone TOTP secret + QR

> Generates a TOTP secret (not bound to a licence), renders the
> otpauth URI + ASCII QR, computes the current 6-digit code so
> the operator can sanity-check the setup before handing the QR
> to the licensee. Same flow as the TUI's `[n]` action on the
> TOTP tab.

## What it demonstrates

- [`TOTPService.Generate`](../../../internal/manager/service/totp.go)
  — base32 secret + persist row with KEK-wrapped ciphertext.
- [`TOTPService.ByID`](../../../internal/manager/service/totp.go)
  — fetch the decrypted view with pre-rendered otpauth URI + ASCII
  + PNG QR ready for display.
- [`totp.Code`](../../../license/totp/totp.go) — RFC 6238 derivation
  on the issuer side; the licensed binary runs the same against
  evidence it collects from the user.

## Run + test

```bash
go run ./examples/license-manager/06-totp-secret    # stdout: URI + QR
go test ./examples/license-manager/06-totp-secret
```

## TUI walkthrough

1. TOTP tab `[8]` → `[n]` opens the create-secret input.
2. Type the account label, enter.
3. `[Q]` opens the centred QR popup; the licensee scans it with
   any RFC 6238 authenticator (Google Authenticator, Aegis…).
4. `[P]` exports the same QR as a PDF (CP1252-encoded so accents
   render correctly).
