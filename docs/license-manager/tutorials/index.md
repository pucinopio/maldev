# Tutorials — operator + developer

Each tutorial pairs a **TUI walkthrough** (the operator side)
with a **runnable Go program** (the developer side) and a CI-
guarded **E2E test** that renders the VHS demo AND drives the
program against a real licence end-to-end.

If you only read one page about how to use license-manager,
start here.

| # | Tutorial | What the operator does | What the developer ships |
|---|---|---|---|
| 01 | [Issue & verify in your binary](./01-issue-and-verify.md) | Generate an unbound licence in the wizard, export PEM + public key. | A 60-line Go client that refuses to start without a valid PEM. |
| 02 | Bindings & evidence *(coming)* | Issue a licence bound to machine + password + TOTP. | A client that collects evidence from the user and feeds it to Verify. |
| 03 | Revocation server & client *(coming)* | Start the revocation HTTP server. | A client that polls `/revoked.pem` and refuses revoked licences. |
| 04 | TOTP secret + authenticator handoff *(coming)* | Generate a TOTP secret in the TUI, share the QR. | A client that asks for the 6-digit code at startup. |
| 05 | Sealed payload for one recipient *(coming)* | Add a recipient key, encrypt a per-licence payload. | A client that decrypts the sealed payload with its private key. |

## Skeleton

Each tutorial directory follows the same shape:

```
examples/license-manager/tutorials/NN-feature/
├── README.md         # cross-link back to the tutorials/ doc
├── client/
│   ├── main.go       # the runnable program a developer paste-imports
│   └── main_test.go  # client-only test (no TUI involvement)
└── e2e_test.go       # renders the matching VHS tape AND runs the client
```

The VHS tape lives in `vhs/tui-gif/tutorial-NN-…tape`; rendering
goes to the gitignored `vhs/out/` so CI doesn't carry GIFs in
the tree.

## How to run the whole pipeline locally

```bash
# 1. Render every tutorial tape (in-process, no ttyd/ffmpeg needed)
go run ./cmd/tui-gif vhs/tui-gif/tutorial-01-issue-and-verify.tape

# 2. Run every E2E (drives tape + client together)
go test ./examples/license-manager/tutorials/...
```

CI runs step 2 on every PR. A green test means: the documented
key sequence still reaches the screens it points at AND the
client program still verifies the real licence the wizard
produces.

## See also

- [Concepts](../concepts.md) — the architecture behind these
  workflows.
- [Cookbook](../workflow.md) — recipes for each backend
  operation (no TUI flow, no client binary).
- [Runnable backend examples](../examples/index.md) — minimal
  `service.Services` walk-throughs for the developer adding
  license-manager to their app.
