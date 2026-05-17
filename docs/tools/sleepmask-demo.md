# `sleepmask-demo`

Source: [`cmd/sleepmask-demo/`](https://github.com/oioio-space/maldev/tree/master/cmd/sleepmask-demo) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/sleepmask-demo](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/sleepmask-demo)

## What it does

Command sleepmask-demo runs encrypted-sleep scenarios against a
concurrent memory scanner. See docs/techniques/evasion/sleep-mask.md.

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o sleepmask-demo.exe ./cmd/sleepmask-demo
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./sleepmask-demo -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
