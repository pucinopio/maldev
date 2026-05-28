# Tutorials

Each tutorial is two things:

1. **What you do in the TUI** — a numbered key sequence.
2. **The client program** — the Go code your binary runs to use
   the licence the TUI produced.

Each page opens with **Objectif / Concepts / Attendu** so you know
what to expect before pressing a single key.

Read them in order — each builds on the last:

| # | Scenario | Concept introduced | Client API |
|---|---|---|---|
| [01](./01-issue-and-verify.md) | Issue a basic licence, verify it | Ed25519 signing, trust chain | `license.Verify` |
| [02](./02-bindings-and-verify.md) | Machine + password + TOTP bindings | Evidence AND semantics | `WithMachineID`, `WithPassword`, `WithTOTPCode` |
| [03](./03-revocation-server.md) | Manager publishes a CRL, client polls it | Live revocation, cache fallback | `WithRevocation(HTTPSource)` |
| [04](./04-totp-authenticator.md) | Hand off a TOTP secret via QR code | Rolling code, clock-skew window | `WithTOTPCode` |
| [05](./05-sealed-payload.md) | Encrypt a payload to one recipient | X25519 sealed box, per-licensee secret | `seal.Open` |

Each page ships a CI-tested example so the documented keys + code
can't silently drift from reality.

## Running them

```bash
# Render every tape into docs/.../tutorials/assets/*.gif:
go run ./cmd/tui-gif vhs/tui-gif/tutorial-NN-*.tape

# Run every E2E (drives tape + client together):
go test ./examples/license-manager/tutorials/...
```
