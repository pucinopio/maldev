# `packer`

Source: [`cmd/packer/`](https://github.com/oioio-space/maldev/tree/master/cmd/packer) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/packer](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/packer)

## What it does

Command packer is a thin CLI wrapper around
[github.com/oioio-space/maldev/pe/packer].
Usage:
	packer pack   -in <file> -out <file> [-key <hex32>] [-keyout <file>]
	              [-format blob|windows-exe|linux-elf] [-rounds N] [-seed S]
	              [-cover]
	packer unpack -in <file> -out <file>  -key <hex32>
	packer bundle -out <file> -pl <spec> [-pl <spec> ...] [-fallback exit|crash|first]
pack:
  - reads `-in`,
  - when -format=blob (default): runs Pack with default options
    (AES-GCM, no compression) and writes the encrypted blob to
    `-out`, printing the AEAD key to stdout as hex (or to
    `-keyout` when set).
  - when -format=windows-exe (Phase 1e-A): runs PackBinary, writes
    a runnable PE32+ to `-out`, and prints the AEAD key to stdout.
    Use -rounds (default 3) and -seed (default 0 = crypto-random)
    to tune the polymorphic stage-1 decoder.
  - when -format=linux-elf (Phase 1e-B): runs PackBinary, writes
    a runnable ELF64 static-PIE to `-out`, and prints the AEAD key
    to stdout. Same -rounds/-seed knobs as windows-exe.
unpack:
  - reads `-in`,
  - runs Unpack with the `-key` hex string,
  - writes the recovered bytes to `-out`.
bundle (C6 multi-target wire format):
  - takes one or more `-pl <file>:<vendor>:<min>-<max>` specs and
    packs them into a single bundle blob. <vendor> is one of
    "intel", "amd", or "*" (wildcard); <min>-<max> is the inclusive
    Windows build-number range (use "*" on either side for "no
    bound"). E.g. -pl payload-w11.exe:intel:22000-99999.
  - -fallback selects the no-match behaviour: "exit" (default),
    "crash", or "first".
  - The output is the bundle blob — the runtime stub-side evaluator
    is C6-P3/P4 work; until then operators inspect the bundle on
    the build host via `packer bundle -inspect`.

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o packer.exe ./cmd/packer
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./packer -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
