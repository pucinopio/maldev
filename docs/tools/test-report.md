# `test-report`

Source: [`cmd/test-report/`](https://github.com/oioio-space/maldev/tree/master/cmd/test-report) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/test-report](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/test-report)

## What it does

Command test-report ingests one or more `go test -json` output streams
and emits a per-test / per-package / per-platform matrix report.
Usage:
	test-report -in linux=/tmp/linux.json -in windows=/tmp/windows.json
	test-report -in host=/tmp/host.json -out /tmp/report.md -format md
Flags:
	-in LABEL=PATH   (repeat) attach a label (linux/windows/host) to a JSON stream
	-format text|md  output format (default text)
	-out PATH        write report to file (default stdout)
	-fail-only       list only failing tests in per-test section
Exit: non-zero if any test failed.

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o test-report.exe ./cmd/test-report
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./test-report -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
