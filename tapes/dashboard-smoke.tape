# tapes/dashboard-smoke.tape — smallest possible smoke test, renders the
# dashboard once with the canned seed. Use as a quick CI sanity check.
# Run via:  make tape-smoke
# Output:   tapes/out/dashboard-smoke.gif

Output tapes/out/dashboard-smoke.gif

Require vhs
Set Shell "bash"
Set Width 1280
Set Height 720
Set FontSize 14
Set Theme "Catppuccin Mocha"

Hide
Type "make tui-snap >/dev/null 2>&1"
Enter
Show

Type "clear; bin/tui-snap -view dashboard -seed scripts/tui-snap-seeds/dashboard.json"
Enter
Sleep 2s
