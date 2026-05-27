# VHS tape — operator inspects an issuer key detail panel, then visits the
# revocation screen and opens the revocation detail (which lazy-loads the
# underlying licence metadata).
#
# Run:  vhs vhs/issuers-and-revocation.tape

Output vhs/out/issuers-and-revocation.gif

Set Shell bash
Set FontSize 14
Set Width 1280
Set Height 720
Set Theme "Catppuccin Mocha"

Type "MALDEV_DEMO=1 ./bin/license-manager"
Enter
Sleep 1.5s

# Issuer keys (tab 3).
Type "3"
Sleep 800ms

# Detail panel is open by default — navigate rows so the right column updates.
Down
Sleep 400ms
Down
Sleep 400ms
Up
Sleep 600ms

# Revocation (tab 6).
Type "6"
Sleep 800ms

# Toggle detail open if not already; navigate.
Down
Sleep 400ms
Down
Sleep 400ms

# Close.
Type "q"
Sleep 800ms
