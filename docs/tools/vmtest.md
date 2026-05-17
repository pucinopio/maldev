# `vmtest`

Source: [`cmd/vmtest/`](https://github.com/oioio-space/maldev/tree/master/cmd/vmtest) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/vmtest](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/vmtest)

## What it does

Command vmtest runs the maldev Go test suite inside isolated VMs with
snapshot restore between runs. Two drivers are supported: VirtualBox
(guestcontrol + shared folder) and libvirt (virsh + ssh + rsync).
Usage:
	vmtest [flags] <windows|windows11|linux|all> [packages] [test-flags]
Examples:
	vmtest windows
	vmtest windows11 "./credentials/..." "-v"
	vmtest linux "./persistence/..." "-v"
	vmtest all "./..." "-count=1"
Configuration lives in scripts/vm-test/config.yaml (committed) with a
per-host override in scripts/vm-test/config.local.yaml (gitignored) and
environment-variable overrides (MALDEV_VM_*, MALDEV_VBOX_EXE).

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o vmtest.exe ./cmd/vmtest
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./vmtest -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
