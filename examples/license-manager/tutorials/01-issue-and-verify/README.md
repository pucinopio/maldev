# Tutorial 01 — Issue in TUI, verify in your binary

End-to-end operator scenario. Run via VHS tape (TUI side) and
the client binary (developer side). The CI test
`TestTutorial01_VHSAndClient` drives both together — if either
half breaks, the test fails and the documented walkthrough
cannot ship stale.

| Piece | Path |
|---|---|
| User doc | [docs/license-manager/tutorials/01-issue-and-verify.md](../../../../docs/license-manager/tutorials/01-issue-and-verify.md) |
| VHS tape | [vhs/tui-gif/tutorial-01-issue-and-verify.tape](../../../../vhs/tui-gif/tutorial-01-issue-and-verify.tape) |
| Client program | [client/main.go](./client/main.go) |
| Client unit test | [client/main_test.go](./client/main_test.go) |
| TUI+client E2E | [e2e_test.go](./e2e_test.go) |

## Run the client against a real licence

```bash
# Issue a licence (any way you like — the wizard, or another example):
go run ./examples/license-manager/01-issue-basic > /tmp/alice.license

# Export the matching public key from the same tree
# (the example writes the issuer to an in-memory store, so to use the
# tutorial client paste the issuer.pub the example would have written —
# or use the e2e_test which does both halves for you).

# Run the client:
go build -o /tmp/license-check ./examples/license-manager/tutorials/01-issue-and-verify/client
/tmp/license-check --license /tmp/alice.license --issuer-pub /tmp/issuer.pub
```

## Run the full E2E (renders tape + verifies real PEM)

```bash
go test ./examples/license-manager/tutorials/01-issue-and-verify
```

The test:

1. Renders the VHS tape into `t.TempDir/tutorial-01.gif` via
   `go run ./cmd/tui-gif`. Asserts the gif is ≥ 1 KB.
2. Issues a real licence with `service.Services`, writes the
   PEM + the issuer's public key to disk.
3. Compiles the client binary and invokes it against those
   files. Asserts stdout contains the verification verdict.
4. Tampers one byte of the PEM and invokes the client again —
   asserts the verifier rejects it.
