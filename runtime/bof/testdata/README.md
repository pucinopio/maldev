# runtime/bof example BOFs

> Source for example BOFs that exercise the loader's Beacon API
> stubs + dollar-import resolver. The compiled `.o` files are not
> committed because they require mingw-w64 cross-compilation
> from this Linux dev box; the build is reproducible from the
> sources here.

## Build

```bash
# One-time toolchain install on a Fedora/Debian dev box.
dnf install mingw64-gcc      # Fedora
apt install mingw-w64        # Debian/Ubuntu

# Build all examples.
cd runtime/bof/testdata
for src in *.c; do
    x86_64-w64-mingw32-gcc -c "$src" -o "${src%.c}.o" \
        -O2 -Wall -ffreestanding -fno-stack-protector
done
```

The flags mirror the Cobalt-Strike convention: `-c` (compile only,
emit COFF), `-O2 -Wall` (sensible defaults), `-ffreestanding`
(no libc startup), `-fno-stack-protector` (CS BOFs do not link
`__chkstk`).

## Examples

### `hello_beacon.c` — BeaconPrintf round-trip

Smallest possible BOF that proves the Beacon API surface works:

- Imports `__imp_BeaconPrintf` (resolved by the Beacon API stub
  table to the Go callback that appends to the BOF's output
  buffer).
- Entry point `go(char *, int)` prints a fixed greeting plus
  the args buffer's first byte.
- After load, `BOF.Execute` should return the printed string.

### `parse_args.c` — Beacon data-parser round-trip

Exercises the data-parsing API:

- Imports `__imp_BeaconDataParse`, `__imp_BeaconDataInt`,
  `__imp_BeaconDataExtract`, `__imp_BeaconPrintf`.
- Reads (int32, length-prefixed string) from the args buffer,
  prints them via BeaconPrintf.
- Caller pre-packs the args via `bof.NewArgs().AddInt(...).AddString(...).Pack()`.

### `loadlib.c` — dollar-import resolution

Proves the `__imp_<DLL>$<Func>` resolver path works for a
real Win32 import:

- Imports `__imp_KERNEL32$LoadLibraryA` and
  `__imp_KERNEL32$GetModuleHandleA`.
- Entry point loads a benign DLL ("crypt32.dll"), retrieves
  its handle, prints the handle as hex via BeaconPrintf,
  then unloads.
- Confirms the PEB walk + ROR13 export-table match patches in
  the real kernel32 entry-point address (no GetProcAddress /
  LoadLibrary import in the BOF's COFF symbol table beyond
  the dollar-import name itself).

## `cs-sa/` — TrustedSec CS-Situational-Awareness-BOF subset

A second set of E2E fixtures drawn from the public
[CS-Situational-Awareness-BOF](https://github.com/trustedsec/CS-Situational-Awareness-BOF)
project. These are battle-tested public BOFs that exercise a
broader Beacon API surface than our hand-written examples —
each `.o` is the real artefact red teams run against Beacon.

**Why fetched, not vendored.** The upstream is GPL-2.0 while
maldev is MIT. Committing GPL-licensed binaries into the MIT
tree creates a licensing tangle even when the `.o` is only test
input. The fetch script pins a known-good upstream commit and
copies four `.o` files into `cs-sa/` (git-ignored). Tests
`t.Skip` cleanly when the directory is absent — local dev or CI
without the fetch step degrades gracefully.

Populate with:

```bash
bash scripts/fetch-cs-sa-bofs.sh
```

The curated subset is:

| BOF | Surface exercised |
|---|---|
| `dir.x64.o` | filesystem enum + args parsing + `MSVCRT$strlen/strcat/_strnicmp/strstr` |
| `env.x64.o` | `KERNEL32$GetEnvironmentStrings` + `lstrlenA`, no args |
| `ipconfig.x64.o` | `IPHLPAPI$GetAdaptersInfo`, exercises non-kernel32 PEB-walk + forwarders |
| `listmods.x64.o` | loaded-module enum, walks PEB Ldr list |

Tests live in `runtime/bof/cs_sa_e2e_windows_test.go`.

## Test wiring

Each `.o` is committed to `testutil/` next to the existing
`nop.o` / `whoami.o` once built locally. Test files in
`runtime/bof/` (e.g. `realbof_windows_test.go`) load them via
`testutil.LoadPayload(t, "<name>.o")`.

## Status

- [ ] `hello_beacon.c` source written, .o build pending
- [ ] `parse_args.c` source written, .o build pending
- [ ] `loadlib.c` source written, .o build pending
- [ ] Tests authored once .o files land

This file unblocks future contributors: once mingw is available,
running the build commands above produces the missing artefacts
without further design work.
