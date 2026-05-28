# tutorial-02-bindings — operator-facing VHS replay.
#
# Visualises the wizard steps the operator clicks through to attach
# machine + password + TOTP bindings to a licence. tui-gif drives a
# synthetic-data TUI so the recording is visual only; the matching
# real issuance happens in e2e_test.go.
#
# Render:
#   go run ./cmd/tui-gif vhs/tui-gif/tutorial-02-bindings.tape

Width 144
Height 44
Delay 320ms
Output vhs/out/tutorial-02-bindings.gif

# Land on dashboard, switch to Licences.
Sleep 600ms
Key 2
Seed licenses 3
Sleep 700ms

# Open detail, cycle to the Bindings tab — what the operator
# inspects after issuing a bound licence to confirm the stamped
# evidence.
Key d
Sleep 500ms
Key B
Sleep 800ms

# Cycle to Identity then PEM to show the full detail surface.
Key I
Sleep 600ms
Key P
Sleep 800ms

# Close detail.
Key d
Sleep 500ms
