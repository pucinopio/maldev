# tui-gif tape — cycle through every detail tab on the licences screen.
# Demonstrates [I/B/P/A/C] switching with content for each.
#
# Run:  go run ./cmd/tui-gif vhs/tui-gif/licenses-detail-tabs-cycle.tape

Width 144
Height 44
Delay 350ms
Output vhs/out/licenses-detail-tabs-cycle.gif

Key 2
Seed licenses 4
Sleep 600ms

# Identity tab (default — row 0 selected).
Key I
Sleep 800ms

# Bindings tab.
Key B
Sleep 800ms

# PEM tab.
Key P
Sleep 800ms

# Audit tab (will show "loading" — service is nil so the lazy-load never
# completes; still demonstrates the tab strip changes).
Key A
Sleep 800ms

# Chain tab.
Key C
Sleep 800ms

# Back to Identity.
Key I
Sleep 500ms

# Toggle detail closed/open with [d].
Key d
Sleep 600ms
Key d
Sleep 600ms
