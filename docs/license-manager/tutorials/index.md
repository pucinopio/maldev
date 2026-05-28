# Tutorials

Each tutorial is two things:

1. **What you do in the TUI** — a numbered key sequence.
2. **The client program** — the Go code your binary runs to use
   the licence the TUI produced.

Nothing else. Each ships a CI-tested example so the documented
keys + code stay green.

| # | Scenario | TUI action | Client code |
|---|---|---|---|
| [01](./01-issue-and-verify.md) | Issue a basic licence, verify it in a binary | Wizard → defaults → confirm | `license.Verify` |
| [02](./02-bindings-and-verify.md) | Bind a licence to machine + password + TOTP | Wizard → bindings ON | `Verify` with `WithMachineID`, `WithPassword`, `WithTOTPCode` |
| [03](./03-revocation-server.md) | Start the CRL server, client polls it | Servers tab → start revocation | `Verify` with `WithRevocation(HTTPSource)` |
| [04](./04-totp-authenticator.md) | Generate a TOTP, the user types the code | TOTP tab → `[n]` → `[Q]` to share QR | `Verify` with `WithTOTPCode` |
| [05](./05-sealed-payload.md) | Encrypt a payload for one recipient only | Recipients tab → `[n]` → wizard step 2 | `seal.Open` with the recipient's private key |

Tutorials 04 and 05 land in the next batch with the same shape.

## Running them

```bash
# Render every tape into vhs/out/*.gif:
go run ./cmd/tui-gif vhs/tui-gif/tutorial-NN-*.tape

# Run every E2E (drives tape + client together):
go test ./examples/license-manager/tutorials/...
```
