# `bof-runner`

Source: [`cmd/bof-runner/`](https://github.com/oioio-space/maldev/tree/master/cmd/bof-runner) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/bof-runner](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/bof-runner)

## What it does

go:build windows
bof-runner — execute a Beacon Object File and print its output.
Usage:
	bof-runner -file path/to/file.o [-arg-int N] [-arg-string S] [-arg-short N] [-arg-bytes hex]
	bof-runner -url https://... [...]
Args are packed in CS-compatible BeaconDataPack format and consumed
by the BOF via BeaconDataParse / DataInt / DataShort / DataExtract.
Designed to run real-world BOFs from the public ecosystem
(TrustedSec situational-awareness, Outflank, FortyNorth TerraTwist,
the Cobalt-Strike-community-kit, …). Constraints documented in
docs/techniques/runtime/bof-loader.md "Beacon-API limitations".

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o bof-runner.exe ./cmd/bof-runner
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./bof-runner -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
