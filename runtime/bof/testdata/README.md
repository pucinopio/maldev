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

The curated subset (18 BOFs):

| BOF | Surface exercised |
|---|---|
| `dir.x64.o` | filesystem enum + (string, short) args + `MSVCRT$strlen/strcat/_strnicmp/strstr` |
| `env.x64.o` | `KERNEL32$GetEnvironmentStrings` + `lstrlenA`, no args |
| `ipconfig.x64.o` | `IPHLPAPI$GetAdaptersAddresses` + 239-entry .rdata ADDR64 pointer-table relocs |
| `listmods.x64.o` | PEB Ldr walk, int arg |
| `arp.x64.o` | `IPHLPAPI$GetIpNetTable` — ARP cache |
| `routeprint.x64.o` | `IPHLPAPI$GetIpForwardTable` — routing table |
| `listdns.x64.o` | `DNSAPI$DnsGetCacheDataTable` — DNS resolver cache |
| `netstat.x64.o` | `IPHLPAPI$GetExtendedTcpTable` / `Udp` — TCP/UDP, int arg |
| `nslookup.x64.o` | `DNSAPI$DnsQuery_A` — active DNS query (distinct from cache) |
| `locale.x64.o` | `KERNEL32$GetLocaleInfoEx` — locale dump |
| `netuptime.x64.o` | `NETAPI32$NetStatisticsGet` — server uptime, wstring arg |
| `netlocalgroup.x64.o` | `NETAPI32$NetLocalGroupEnum` — local groups, (short, wstring, wstring) |
| `netloggedon.x64.o` | `NETAPI32$NetWkstaUserEnum` — logged-on users, wstring arg |
| `enumlocalsessions.x64.o` | `WTSAPI32$WTSEnumerateSessionsExA` — new module surface |
| `sc_enum.x64.o` | `ADVAPI32$EnumServicesStatusEx` — service enum, wstring arg |
| `list_firewall_rules.x64.o` | HNetCfg COM (INetFwPolicy2) — CoInitialize + CoCreateInstance |
| `driversigs.x64.o` | `ADVAPI32$EnumServicesStatusExW` (driver filter) |
| `md5.x64.o` | `ADVAPI32` CryptCreateHash + MSVCRT file I/O, string arg |

Tests live in `runtime/bof/cs_sa_e2e_windows_test.go`. The
18-BOF suite exercises PEB-walk on a dozen modules (kernel32,
ntdll, msvcrt, iphlpapi, netapi32, dnsapi, advapi32, wtsapi32,
setupapi, shlwapi, ole32, hnetcfg), export forwarders, all args
shapes, the multi-section relocation path, and COM init from
within a BOF.

## Test wiring

Each `.o` is committed to `testutil/` next to the existing
`nop.o` / `whoami.o` once built locally. Test files in
`runtime/bof/` (e.g. `realbof_windows_test.go`) load them via
`testutil.LoadPayload(t, "<name>.o")`.

## Status

- [x] In-tree examples (`hello_beacon.c` / `parse_args.c` /
  `loadlib.c` / `realworld_calls.c`) — `.o` committed, tests
  pass on host + VM.
- [x] CS-SA-BOF subset (10 BOFs) — fetch-on-demand, all 10
  PASS on host + Windows10 VM as of 2026-05-18.

Pickup ideas (no current owner):

- Add x86 variants (`.x86.o`) to exercise the future
  `BeaconGetSpawnTo(BOOL bX86)` operator path.
- Cover the SE-required BOFs (`netuser`, `netshares /asAdmin`)
  in a separate intrusive suite gated by `MALDEV_INTRUSIVE=1`.

This file unblocks future contributors: once mingw is available,
running the build commands above produces the missing artefacts
without further design work.
