# tutorial-01-issue-and-verify — operator-facing VHS replay.
#
# Captures the operator flow for tutorials/01-issue-and-verify:
#   1. Land on the dashboard.
#   2. Switch to the Licences screen ([2]).
#   3. Seed-display so the table isn't empty (cmd/tui-gif synthetic rows).
#   4. Open the licence detail to expose the PEM tab — what the
#      operator copies for the client binary.
#
# Note: cmd/tui-gif drives a synthetic-data TUI (no real backend), so
# the recording demonstrates the visual flow only — the matching real
# issuance happens in main_test.go's E2E pipeline.
#
# Render with:
#   go run ./cmd/tui-gif vhs/tui-gif/tutorial-01-issue-and-verify.tape

Width 144
Height 44
Delay 320ms
Output vhs/out/tutorial-01-issue-and-verify.gif

# 1. Boot lands on the dashboard.
Sleep 700ms

# 2. Switch to the Licences screen.
Key 2
Seed licenses 4
Sleep 700ms

# 3. Open the detail panel for the first row.
Key d
Sleep 600ms

# 4. Cycle to the PEM tab — this is what the operator exports for the
# client binary in the matching tutorial.
Key P
Sleep 800ms

# 5. Cycle to the Chain tab to show supersession lineage UI.
Key C
Sleep 800ms

# 6. Back to the Identity tab and close detail to end on a clean frame.
Key I
Sleep 500ms
Key d
Sleep 600ms
