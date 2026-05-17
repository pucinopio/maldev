# `bundle-launcher`

Source: [`cmd/bundle-launcher/`](https://github.com/oioio-space/maldev/tree/master/cmd/bundle-launcher) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/bundle-launcher](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/bundle-launcher)

## What it does

Command bundle-launcher is the operator-facing runtime for a C6
multi-target bundle. Build it once, append a bundle blob via
`packer bundle -wrap` (or [packer.AppendBundle]), and ship the
resulting single-file executable.
At runtime the launcher:
 1. Reads its own binary via `os.Executable()`.
 2. Validates the trailing footer (8-byte LE offset + "MLDV-END") via
    [packer.ExtractBundle].
 3. Calls [packer.MatchBundleHost] (CPUID vendor + Win build dispatch).
 4. Decrypts the matched payload via [packer.UnpackBundle].
 5. Writes plaintext to a memfd_create-backed FD on Linux (zero
    on-disk artefact) or a temp file on Windows, then execs it.
The launcher exits cleanly when no entry matches and the bundle's
FallbackBehaviour is BundleFallbackExit.
Usage:
	# Build a generic launcher once:
	go build -o bundle-launcher ./cmd/bundle-launcher
	# Pack N target binaries into a bundle:
	packer bundle -out bundle.bin \
	  -pl payload-w11.exe:intel:22000-99999 \
	  -pl payload-w10.exe:amd:10000-19999 \
	  -pl fallback.exe:*:*-*
	# Wrap the bundle into the launcher:
	packer bundle -wrap bundle-launcher -bundle bundle.bin -out app.exe
	# Ship app.exe — it dispatches at runtime:
	./app.exe

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o bundle-launcher.exe ./cmd/bundle-launcher
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./bundle-launcher -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
