# `sleepmask-demo`

> Demo harness for evaluating sleep-mask techniques against a memory scanner.

**Source:** [`cmd/sleepmask-demo/`](https://github.com/oioio-space/maldev/tree/master/cmd/sleepmask-demo) · **godoc:** [pkg.go.dev/…/cmd/sleepmask-demo](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/sleepmask-demo)
**Audience:** researcher / detection engineer · **Platforms:** Windows

## What it does

Runs the `evasion/sleepmask` masking scenarios in-process while a concurrent
scanner reads the heap, so you can compare detection rates per mask
(XOR / RC4 / AES-CTR / Ekko). Not an operational tool — purpose is to
empirically validate a mask before wiring it into a payload.

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o sleepmask-demo.exe ./cmd/sleepmask-demo
```

## Example

```cmd
sleepmask-demo.exe -h
```

## See also

- Technique: [`evasion/sleepmask`](../techniques/evasion/sleep-mask.md).
