# VHS tape — operator imports a PEM then exports another one back to disk.
# Covers the [i] import flow (file picker → svc.License.Import) and the
# [E] export flow (input overlay → svc.License.ExportPEM).
#
# Run:  vhs vhs/licenses-import-export.tape

Output vhs/out/licenses-import-export.gif

Set Shell bash
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"

Type "MALDEV_DEMO=1 ./bin/license-manager"
Enter
Sleep 1.5s

# Goto Licences.
Type "2"
Sleep 600ms

# Trigger import — pushes a filepicker overlay.
Type "i"
Sleep 700ms
# Cancel (escape) — the recording demonstrates the picker exists; an actual
# file path would require the demo store to ship a sample PEM, which adds
# noise to the gif.
Escape
Sleep 600ms

# Select the first row then export it via [E].
Down
Sleep 300ms
Type "E"
Sleep 800ms
# Type a target path.
Type "/tmp/exported-licence.pem"
Sleep 500ms
Enter
Sleep 1s

# Close the OK overlay.
Escape
Sleep 500ms

Type "q"
Sleep 800ms
