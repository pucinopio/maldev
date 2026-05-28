# tutorial-05-sealed — operator-facing VHS replay.
#
# Operator opens the Recipients screen, mints a fresh X25519 keypair (n),
# then exports the public half (E) so the licensee can later be paired
# with sealed payloads. Real sealing happens inside e2e_test.go.
#
# Render:
#   go run ./cmd/tui-gif vhs/tui-gif/tutorial-05-sealed.tape

Width 144
Height 44
Delay 320ms
Output vhs/out/tutorial-05-sealed.gif

Sleep 600ms

# Recipients screen.
Key 4
Seed recipients 2
Sleep 800ms

# Mint a fresh recipient keypair.
Key n
Sleep 700ms

# Show the row detail.
Key d
Sleep 800ms
Key d
Sleep 500ms
