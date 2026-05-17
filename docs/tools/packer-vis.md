# `packer-vis`

Source: [`cmd/packer-vis/`](https://github.com/oioio-space/maldev/tree/master/cmd/packer-vis) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/packer-vis](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/packer-vis)

## What it does

Command packer-vis is a small introspection tool that renders
human-readable views of packer artefacts:
	packer-vis entropy <file>     # Shannon-entropy heatmap (Unicode shading + ANSI color)
	packer-vis bundle  <bundle>   # bundle wire-format ASCII art (header + entries + data)
No TUI framework, no external assets — pure stdlib + ANSI 256-color
codes. The output is paste-friendly into terminals, READMEs, demo
recordings.
Pedagogical intent: anyone who wants to *see* what the packer does
to a binary can run `packer-vis entropy notepad.exe` before and
after `packer pack` and watch the .text section flip from low-
entropy code to high-entropy ciphertext. The bundle view exposes
the wire format byte-by-byte so the spec at
.dev/superpowers/specs/2026-05-08-packer-multi-target-bundle.md
stops being an abstract document and becomes a thing on screen.

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o packer-vis.exe ./cmd/packer-vis
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./packer-vis -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
