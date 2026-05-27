# tui-gif tape — mirrors vhs/licenses-nav-and-pem-scroll.tape but runs the
# TUI in-process (no shell launch step). Drives the licences nav + PEM scroll
# regression flow recorded by TestVHS_LicensesNavAndPEMScroll.
#
# Run:  go run ./cmd/tui-gif vhs/tui-gif/licenses-nav-and-pem-scroll.tape

Width 144
Height 44
Delay 250ms
Output vhs/out/licenses-nav-and-pem-scroll.gif

# Land on Licences with seeded data (row 0 has a 200-line PEM).
Key 2
Seed licenses 5
Sleep 500ms

# Move cursor down 3 rows.
Down
Down
Down
Sleep 500ms

# Open the PEM detail tab.
Key P
Sleep 600ms

# Scroll PEM with j (×3), then space (half-page ×2).
Key j
Key j
Key j
Sleep 400ms
Space
Sleep 400ms
Space
Sleep 500ms

# Jump to bottom (G) and back to top (g).
Key G
Sleep 600ms
Key g
Sleep 400ms

# ↑/↓ still navigate the table (PEM follows the selection).
Up
Up
Up
Sleep 600ms
