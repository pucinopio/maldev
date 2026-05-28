# tutorial-04-totp — operator-facing VHS replay.
#
# Operator opens the TOTP tab, mints a fresh secret (n), then pops
# up the QR code (Q) for the licensee to scan with their
# authenticator app. The real provisioning happens inside e2e_test.go.
#
# Render:
#   go run ./cmd/tui-gif vhs/tui-gif/tutorial-04-totp.tape

Width 144
Height 44
Delay 320ms
Output vhs/out/tutorial-04-totp.gif

Sleep 600ms

# Switch to the TOTP screen.
Key 4
Seed totp 3
Sleep 800ms

# New TOTP secret.
Key n
Sleep 700ms

# Pop the QR overlay.
Key Q
Sleep 1200ms

# Dismiss the overlay.
Key esc
Sleep 500ms
