# VHS tape — operator flow: navigate the licences table AND scroll the PEM
# panel. Reproduces the regression "navigation ne fonctionne plus" + the
# follow-up "scroll PEM ne marche pas".
#
# Each Sleep makes the GIF readable AND gives bubbletea a frame to repaint.
# Expected end state:
#   - 4 licences visible in the table
#   - cursor moved to row 3
#   - PEM tab open, scroll indicator shows ~50% of a long PEM
#
# Run:  vhs vhs/licenses-nav-and-pem-scroll.tape

Output vhs/out/licenses-nav-and-pem-scroll.gif

Set Shell bash
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"

# Launch the license-manager pointing to an in-memory store. The TUI seeds
# itself with demo data when MALDEV_DEMO=1, so the recording always has
# enough licences to navigate + a long PEM to scroll.
Type "MALDEV_DEMO=1 ./bin/license-manager"
Enter
Sleep 1.5s

# Navigate to Licences tab (2 = second top-level tab).
Type "2"
Sleep 700ms

# Move down 3 rows — the cursor highlight follows.
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 700ms

# Open the PEM detail tab.
Type "P"
Sleep 700ms

# Scroll the PEM viewport with j (line) and space (half-page).
Type "jjj"
Sleep 500ms
Space
Sleep 500ms
Space
Sleep 500ms

# Use g/G to jump start/end.
Type "G"
Sleep 700ms
Type "g"
Sleep 500ms

# ↑/↓ now move the table (and PEM follows). Operator returns to row 1.
Up
Sleep 300ms
Up
Sleep 300ms
Up
Sleep 700ms

# Quit cleanly.
Type "q"
Sleep 800ms
