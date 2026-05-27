# tui-gif tape — exhaustive scroll bindings on the PEM tab.
# Seeds 5 licences (row 0 has a 200-line PEM) so every scroll affordance
# produces observable motion in the GIF.
#
# Sequence:
#   2          → go to Licences
#   Seed       → inject 5 fake licences with a long PEM on row 0
#   P          → open PEM detail tab on row 0
#   j j j      → line-down × 3 (3 lines each = 9 lines down)
#   k k        → line-up × 2 (back up 6 lines)
#   Space      → half-page down
#   Space      → half-page down
#   b          → half-page up
#   PgDn       → half-page down (terminal-key variant)
#   PgUp       → half-page up
#   G          → jump to bottom
#   g          → jump to top
#   Up Up      → table nav still works on PEM tab (preview updates to row -2... clamps at 0)
#   Down       → back to row 1 (PEM auto-reloads)
#
# Run:  go run ./cmd/tui-gif vhs/tui-gif/pem-scroll-all-bindings.tape

Width 144
Height 44
Delay 300ms
Output vhs/out/pem-scroll-all-bindings.gif

Key 2
Seed licenses 5
Sleep 600ms

Key P
Sleep 700ms

# Line scroll with j (down).
Key j
Sleep 250ms
Key j
Sleep 250ms
Key j
Sleep 500ms

# Line scroll with k (up).
Key k
Sleep 250ms
Key k
Sleep 500ms

# Half-page with space.
Space
Sleep 300ms
Space
Sleep 500ms

# Half-page back with b.
Key b
Sleep 500ms

# Terminal-key variants (PgDn / PgUp).
PgDn
Sleep 400ms
PgUp
Sleep 500ms

# Jump to bottom + top.
Key G
Sleep 700ms
Key g
Sleep 500ms

# Table nav while PEM tab open — selection changes, PEM auto-reloads.
Down
Sleep 400ms
Down
Sleep 400ms
Up
Sleep 500ms
