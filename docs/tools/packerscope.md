# `packerscope`

Source: [`cmd/packerscope/`](https://github.com/oioio-space/maldev/tree/master/cmd/packerscope) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/packerscope](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/packerscope)

## What it does

Command packerscope is the defender-side companion to the maldev
packer: it identifies, parses, and unpacks maldev bundle artefacts
without requiring a running target.
Three verbs:
	packerscope detect  <file>                    What kind of maldev artefact is this?
	packerscope dump    <file> [-secret S]        Print the full wire-format structure.
	packerscope extract <file> [-secret S] -out D Write decrypted payloads under D/.
Per Kerckhoffs: when an operator wraps with `-secret S`, packerscope
must be invoked with the same `-secret S` to find the
deterministically-derived BundleMagic + footer. Without the secret,
canonical detection still works (catches operator builds that
shipped without `-secret`), and structural heuristics flag suspicious
shapes (single-PT_LOAD RWX ELF under 4 KiB) so a defender knows
"this looks like a maldev all-asm bundle of unknown deployment".
Pedagogical pair: every algorithm `cmd/packer` ships forward,
`cmd/packerscope` undoes (or at least describes) backward. Operator
or defender — the wire format is genuinely public.

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o packerscope.exe ./cmd/packerscope
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./packerscope -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
