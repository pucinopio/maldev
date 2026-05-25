# tapes/wizard-step3.tape — verifies the wizard step 3 hint reword
# (commit 72abb4c: "enter ok (empty=skip)" instead of "enter confirm").
# Run via:  make tape-wizard-step3
# Output:   tapes/out/wizard-step3.gif

Output tapes/out/wizard-step3.gif

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

# Render wizard step 3 (machine binding) in paste mode (default).
Type "clear; bin/tui-snap -view wizard-step3"
Enter
Sleep 2s
