# tui-gif tape — mirrors TestVHS_LicensesImportExport.
# Run:  go run ./cmd/tui-gif vhs/tui-gif/licenses-import-export.tape

Width 144
Height 44
Delay 280ms
Output vhs/out/licenses-import-export.gif

Key 2
Seed licenses 3
Sleep 500ms

# Trigger import [i] — filepicker overlay appears.
Key i
Sleep 700ms
Esc
Sleep 400ms

# Pick first row then export [E].
Down
Sleep 300ms
Key E
Sleep 600ms

# Type a target path then submit.
Type /tmp/exported.pem
Sleep 400ms
Enter
Sleep 700ms

# Close the OK overlay.
Esc
Sleep 500ms
