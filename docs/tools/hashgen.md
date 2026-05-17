# `hashgen`

Source: [`cmd/hashgen/`](https://github.com/oioio-space/maldev/tree/master/cmd/hashgen) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/hashgen](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/hashgen)

## What it does

Command hashgen prints pre-computed API-name hash constants for use
in shellcode-style API resolution.
Usage:
	go run ./cmd/hashgen -algo ror13 LoadLibraryA GetProcAddress
	go run ./cmd/hashgen -algo fnv1a64 -package winsyms NtAllocateVirtualMemory NtCreateThreadEx
	echo -e "LoadLibraryA\nGetProcAddress" | go run ./cmd/hashgen -algo djb2 -stdin
Algorithms: ror13, ror13module, fnv1a32, fnv1a64, jenkins, djb2,
crc32. See [github.com/oioio-space/maldev/hash] for the underlying
implementations and reference vectors.
Designed for `go generate` — emit a generated `.go` file once,
commit it, ship without the runtime cost of re-hashing on every
process start. The output uses the constant naming convention
`Hash<AlgoSuffix><Symbol>` (e.g. `HashRor13LoadLibraryA`,
`HashFnv1a64NtAllocateVirtualMemory`) to avoid collisions when
multiple families coexist in the same package.

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o hashgen.exe ./cmd/hashgen
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./hashgen -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
