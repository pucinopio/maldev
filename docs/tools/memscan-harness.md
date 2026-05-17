# `memscan-harness`

Source: [`cmd/memscan-harness/`](https://github.com/oioio-space/maldev/tree/master/cmd/memscan-harness) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/memscan-harness](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/memscan-harness)

## What it does

Command memscan-harness is the target-side companion for the
vm-test-memscan orchestrator. It applies an evasion technique (SSN
resolve, AMSI patch, ETW patch, or ntdll unhook) using one of the four
syscall caller methods (WinAPI, NativeAPI, Direct, Indirect), prints a
single READY line with relevant addresses, and sleeps until killed.
Flags:
	-group     ssn | amsi | etw | unhook    (required)
	-caller    winapi | nativeapi | direct | indirect      (default winapi)
	-resolver  hellsgate | halosgate | tartarus | hashgate (SSN group only)
	-fn        ntdll function name (SSN group only)
	-variant   classic | full (unhook group only)
Windows-only.

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o memscan-harness.exe ./cmd/memscan-harness
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./memscan-harness -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
