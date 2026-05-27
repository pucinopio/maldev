# tui-gif tape — filter cycling + search input flow on licenses screen.
#
# Run:  go run ./cmd/tui-gif vhs/tui-gif/licenses-filter-and-search.tape

Width 144
Height 44
Delay 280ms
Output vhs/out/licenses-filter-and-search.gif

Key 2
Seed licenses 8
Sleep 600ms

# Filter cycle [f] — all → active → expiring → expired → revoked → superseded → all
Key f
Sleep 500ms
Key f
Sleep 500ms
Key f
Sleep 500ms
Key f
Sleep 500ms
Key f
Sleep 500ms
Key f
Sleep 500ms

# Search input flow.
Key /
Sleep 400ms
Type alice
Sleep 600ms
Esc
Sleep 500ms

# Re-focus + clear via backspace.
Key /
Sleep 400ms
Backspace
Backspace
Backspace
Backspace
Backspace
Sleep 500ms

# Type a substring that matches multiple rows.
Type research
Sleep 600ms
Enter
Sleep 500ms
