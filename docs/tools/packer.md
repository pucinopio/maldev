# `packer`

> Pack, unpack, and bundle PE/ELF payloads with the SGN+LZ4 stub.

**Source:** [`cmd/packer/`](https://github.com/oioio-space/maldev/tree/master/cmd/packer) · **godoc:** [pkg.go.dev/…/cmd/packer](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/packer)
**Audience:** operator · **Platforms:** Windows + Linux output, builds on any host

## Synopsis

```text
packer pack    -in <file> -out <file> -format <blob|windows-exe|linux-elf> [options]
packer unpack  -in <file> -out <file> -key <hex32>
packer bundle  -out <file> -pl <file>:<vendor>:<min>-<max> [-pl ...] [-fallback exit|crash|first]
packer bundle  -wrap <launcher> -bundle <blob> -out <exe>
```

## Subcommands

### `pack`

Wraps `pe/packer.PackBinary`. Produces a runnable binary that decrypts and
executes the original payload in-memory.

| Flag | Default | Meaning |
|---|---|---|
| `-in` | — | Input payload (PE or ELF). |
| `-out` | — | Packed output path. |
| `-format` | `blob` | `blob` = raw encrypted blob (key to stdout). `windows-exe` / `linux-elf` = self-running stub-wrapped binary. |
| `-key` | random | 32-byte AEAD key (hex). |
| `-keyout` | stdout | Write key to file instead of stdout. |
| `-rounds` | `3` | SGN polymorphic rounds (`windows-exe` / `linux-elf`). |
| `-seed` | random | Decoder seed; pin for reproducible builds. |
| `-compress` | off | LZ4 the payload before encryption. |
| `-antidebug` | off | Embed anti-debug checks in the stub. |
| `-randomize` | off | Randomise section names + stub layout. |

### `unpack`

Inverse of `pack -format blob`. Reads the blob, decrypts with `-key`, writes
the original payload to `-out`.

### `bundle`

Multi-target dispatch — one blob holds N payloads, each matched by CPUID
vendor + Windows build range at runtime.

```text
-pl <file>:<vendor>:<min>-<max>     # vendor: intel | amd | *   range: <num>-<num> or *-*
-fallback exit|crash|first          # behaviour when no entry matches
```

Wrap into a runnable executable via `-wrap <bundle-launcher.exe>`.

## Build

```bash
go build -o packer ./cmd/packer
```

## Examples

```bash
# Pack a Windows EXE with anti-debug + compression
packer pack -in implant.exe -out packed.exe -format windows-exe \
  -compress -antidebug -randomize

# Build a CPU-aware bundle dispatching by vendor
packer bundle -out app.bin \
  -pl payload-intel.exe:intel:22000-99999 \
  -pl payload-amd.exe:amd:22000-99999

# Wrap the bundle inside the launcher
packer bundle -wrap bundle-launcher.exe -bundle app.bin -out app.exe
```

## See also

- Technique: [`pe/packer`](../techniques/pe/packer.md).
- Runbook: [Defender catch on dropper](../examples/runbooks/defender-catch.md).
- Companion tool: [`bundle-launcher`](bundle-launcher.md).
