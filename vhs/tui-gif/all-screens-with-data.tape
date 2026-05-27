# tui-gif tape — tour every screen WITH seeded data so the detail panels
# and tables actually have something to render. Useful as a single demo GIF
# that shows the entire TUI surface populated.
#
# Run:  go run ./cmd/tui-gif vhs/tui-gif/all-screens-with-data.tape

Width 144
Height 44
Delay 400ms
Output vhs/out/all-screens-with-data.gif

# Dashboard.
Key 1
Sleep 600ms

# Licences.
Key 2
Seed licenses 6
Sleep 800ms
Down
Down
Sleep 500ms

# Issuer keys.
Key 3
Seed issuers 4
Sleep 800ms
Down
Sleep 500ms

# Recipients.
Key 4
Seed recipients 3
Sleep 800ms
Down
Sleep 500ms

# Identities.
Key 5
Seed identities 3
Sleep 800ms

# Revocation.
Key 6
Seed revocation 4
Sleep 800ms
Down
Sleep 500ms

# Servers (no seed — empty state is the relevant view).
Key 7
Sleep 700ms

# TOTP.
Key 8
Seed totp 3
Sleep 700ms

# Audit (filter chips).
Key 9
Seed audit 12
Sleep 800ms

# Settings.
Key 0
Sleep 700ms
