# Runnable examples

> Tutorial binaries under [`examples/`](https://github.com/oioio-space/maldev/tree/master/examples)
> in the repo — each one builds a small chain of `maldev` packages
> and demonstrates a single technique end-to-end. Cross-link the
> markdown pages in this section (`docs/examples/*.md`) with the
> binary you want to actually compile and run.

The full catalogue with one-line descriptions, technique mapping,
and a "What it demonstrates" column lives in the repo at
[`examples/README.md`](https://github.com/oioio-space/maldev/blob/master/examples/README.md).

## Naming convention

`<domain>-<technique>` to align with `docs/techniques/<domain>/<technique>.md`.
Operators reading the technique page in this handbook see a direct
pointer to the runnable companion.

## Highlights

- **`privesc-dll-hijack`** — full chain from `lowuser` shell to
  `NT AUTHORITY\SYSTEM` via DLL hijack, with packer + AMSI bypass +
  preset.Aggressive evasion stack. Ships its own
  [README walkthrough](https://github.com/oioio-space/maldev/blob/master/examples/privesc-dll-hijack/README.md).
- **`packer-tour`** — every packer mode (Mode 1 EXE+SGN, Mode 6
  shellcode-self-exec, Mode 7 DLL+SGN+LZ4, Mode 8 EXE→DLL convert,
  Mode 10 proxy DLL).
- **`syscall-matrix`** — same routine run through `wsyscall.MethodWinAPI`,
  `MethodNative`, `MethodDirect`, `MethodIndirect`; useful as a base
  for measuring per-method telemetry.

## Building

```bash
GOOS=windows GOARCH=amd64 go build -o /tmp/example.exe ./examples/<name>
```

## Adding a new example

See the "Adding a new example" section in
[`examples/README.md`](https://github.com/oioio-space/maldev/blob/master/examples/README.md#adding-a-new-example).
