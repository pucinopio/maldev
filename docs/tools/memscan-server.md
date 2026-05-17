# `memscan-server`

Source: [`cmd/memscan-server/`](https://github.com/oioio-space/maldev/tree/master/cmd/memscan-server) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/memscan-server](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/memscan-server)

## What it does

Command memscan-server exposes a minimal HTTP/JSON inspection API
(ReadProcessMemory, EnumProcessModules, export lookup) on port 50300 so
that host-side test orchestrators can verify byte patterns inside a
running target process. It replaces the old x64dbg+MCP setup for the
static verification matrix (75 tests) — execution verification stays
on canary scans and the Kali Meterpreter matrix.
Runs on Windows only. On any other platform the server refuses to start.
Usage:
	memscan-server [--addr 0.0.0.0:50300]

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o memscan-server.exe ./cmd/memscan-server
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./memscan-server -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
