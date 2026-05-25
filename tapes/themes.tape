# tapes/themes.tape — visual proof that ApplyTheme swaps the live palette.
# Renders the dashboard three times in a row, one per shipped palette.
# Run via:  make tape-themes   (or vhs tapes/themes.tape directly)
# Output:   tapes/out/themes.gif

Output tapes/out/themes.gif

Require vhs
Set Shell "bash"
Set Width 1280
Set Height 720
Set FontSize 14
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms

# Build the binary first (idempotent) so a fresh checkout works.
Hide
Type "make license-manager tui-snap >/dev/null 2>&1"
Enter
Show

# --- neon (default palette) ---
Type "clear; bin/tui-snap -view dashboard -theme neon -seed scripts/tui-snap-seeds/dashboard.json"
Enter
Sleep 1500ms

# --- mono (grayscale) ---
Type "clear; bin/tui-snap -view dashboard -theme mono -seed scripts/tui-snap-seeds/dashboard.json"
Enter
Sleep 1500ms

# --- nord-soft ---
Type "clear; bin/tui-snap -view dashboard -theme nord-soft -seed scripts/tui-snap-seeds/dashboard.json"
Enter
Sleep 1500ms
