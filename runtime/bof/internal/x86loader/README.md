# `runtime/bof/internal/x86loader` — WoW64 BOF loader shellcode

32-bit (i386) position-independent shellcode injected into a fresh
WoW64 host by the parent x64 implant so a Beacon Object File
compiled for x86 can execute despite the implant being 64-bit.
Slice 1.d phase B-bis of the
[BOF loader revamp](../../../../.dev/refactor-2026/bof-loader-revamp-plan.md).

## Build (maintainer only — `go build` does not need this)

```bash
# From repo root. Uses host i686-w64-mingw32-gcc when present,
# falls back to a Podman container (fedora:42 + mingw32-gcc) for
# reproducibility on hosts without the toolchain on PATH.
bash scripts/build-bof-x86-loader.sh
```

The build produces `bof_x86_loader.x86.bin` — a flat `.text` blob
(no PE wrapper, no static imports, no `.rodata`) committed to the
repo. Operators never invoke this script: `go build` (and
`go build -tags=bof_x86_loader`) embeds the committed `.bin`
verbatim via `go:embed`, so a Go-only toolchain is sufficient
to build a fully-functional implant.

## Architecture — no-disk, no-LoadLibrary

```
parent (x64 Go)                      child (x86 WoW64 host)
─────────────────                    ──────────────────────
1. CreateProcess(SysWOW64\…\rundll32.exe, CREATE_SUSPENDED)
   (rundll32 with no argv is a benign WoW64 placeholder —
   never LoadLibrary's anything from us)

2. VirtualAllocEx(child, _,  CODE_LEN, …, PAGE_READWRITE)
   → write loader shellcode bytes
   → VirtualProtectEx(PAGE_EXECUTE_READ)

3. VirtualAllocEx(child, _, len(bof)+len(args)+out_cap+err_cap)
   → write BOF .o, args, zero out/err buffers

4. VirtualAllocEx(child, _, sizeof(loader_params_t))
   → write magic + version + all four buffer addresses

5. CreateRemoteThread(child, _, _,
       lpStartAddress=<loader CODE region offset 0>,
       lpParameter=<params region address>, …)

                                      loader_entry(params):
                                        validate magic+version
                                        walk PEB → kernel32 base
                                        resolve kernel32 by ROR13
                                        run BOF in-process       ← step 1
                                        write out/err buffers
                                        params.status = DONE
                                        ExitThread(DONE)

6. WaitForSingleObject(thread, timeout)
7. ReadProcessMemory(params)         → status + lengths
8. ReadProcessMemory(out, err)       → captured BOF output
9. TerminateProcess(child)
10. CloseHandle(thread, process)
```

No file lands on disk in the BOF execution path. The implant
binary ships the shellcode as embedded bytes; the only host
artefact is the rundll32 process, which lives for the duration
of one BOF call.

## ABI (`abi.h` ↔ `runtime/bof/x86fork_windows.go`)

The parent (Go) and the loader (C) communicate through a single
`loader_params_t` struct written into the child's IO region. The
struct is mirrored byte-for-byte on both sides — any change in
`abi.h` MUST land in the Go mirror in the same commit, with a
`LOADER_ABI_VERSION` bump if the change is wire-incompatible.

| Field | Direction | Purpose |
|---|---|---|
| `magic`                       | parent → loader | `0x36384342` (`'BC86'`). Refuses corrupted blocks. |
| `version`                     | parent → loader | `1`. Loader writes `LOADER_STATUS_ABI_MISMATCH` on mismatch. |
| `status`                      | loader → parent | One of the `LOADER_STATUS_*` codes; populated before ExitThread. |
| `error_code`                  | loader → parent | SEH exception code / GetLastError on failure paths. |
| `bof_addr` / `bof_len`        | parent → loader | Remote address + length of the BOF `.o` bytes. |
| `args_addr` / `args_len`      | parent → loader | Remote address + length of the `BeaconDataPack` blob. |
| `user_data_addr` / `_len`     | parent → loader | `BeaconGetCustomUserData` blob (0 = none). |
| `spawn_to_addr`               | parent → loader | NUL-terminated UTF-8 SpawnTo path (0 = none). |
| `out_addr` / `out_cap` / `_len` | parent allocates; loader fills `_len` | BOF stdout. |
| `err_addr` / `err_cap` / `_len` | parent allocates; loader fills `_len` | `BeaconErrorD/DD/NA`. |
| `reserved[12]`                | — | Fixed-size tail for forward-compat. |

## Symbol resolution — ROR13 against kernel32

The loader has zero static imports. At entry it walks the WoW64
PEB (`fs:[0x30]` → `Ldr` → `InMemoryOrderModuleList`) to find
kernel32's base address, then hashes each export name with the
same 13-bit-right-rotate accumulator used by
`win/api.ResolveByHash`. Pre-computed hashes in `loader.c` match
the ones produced by the Go-side test
(`TestRor13_KnownAnswers` in `x86fork_present_windows_test.go`).

## Beacon API surface (25 symbols)

| Group | Symbols |
|---|---|
| Output / Errors | `BeaconPrintf`, `BeaconOutput`, `BeaconErrorD/DD/NA`, `BeaconGetOutputData` |
| Data parsing | `BeaconDataParse`, `BeaconDataInt`, `BeaconDataShort`, `BeaconDataLength`, `BeaconDataExtract` |
| Format | `BeaconFormatAlloc/Reset/Free/Append/Int/Printf/ToString` |
| Helpers | `BeaconGetCustomUserData`, `BeaconGetSpawnTo`, `toWideChar` |
| KV (per-Run) | `BeaconAddValue`, `BeaconGetValue`, `BeaconRemoveValue` |
| Token + Admin | `BeaconIsAdmin`, `BeaconUseToken`, `BeaconRevertToken` |
| Inject + Spawn | `BeaconSpawnTemporaryProcess`, `BeaconInjectProcess`, `BeaconInjectTemporaryProcess`, `BeaconCleanupProcess` |

`BeaconPrintf` and `BeaconFormatPrintf` honour `%d / %i / %u / %x
/ %X / %p / %s / %c / %%` from cdecl varargs (width / padding /
precision flags parsed but ignored). Unknown specifiers emit
raw `%<x>`.

## Step roadmap

| Step | Status | Scope |
|---|---|---|
| 0 | closed | Skeleton: PEB walk, ROR13, ExitThread resolution, status = DONE. |
| 1.a | closed | Expanded kernel32 set (VirtualAlloc / Protect / Free + LoadLibraryA + GetCurrentProcess + CloseHandle + …). |
| 1.b | closed | i386 COFF parser + IMAGE_REL_I386_ABSOLUTE/DIR32/DIR32NB/REL32 relocations + `_go` entry call. |
| 1.c | closed | Reflective-DLL pivot (drops flat-PIC + .reloc-discard) + Data + Format + Output families. |
| 1.e | closed | Helpers + KV — UserData, SpawnTo, toWideChar, Add/Get/Remove. |
| 1.f | closed | Token + IsAdmin via advapi32 (LDR/LoadLibraryA + ROR13). |
| 1.g | closed | printf-with-% expansion via cdecl-stack varargs. |
| 1.h | closed | Inject + Spawn family (CreateProcessA + VirtualAllocEx + WriteProcessMemory + CreateRemoteThread + TerminateProcess + ResumeThread). |

## Compatibility notes

- `kernel32!RtlMoveMemory` is a forwarder to `ntdll!RtlMoveMemory`
  on Win 7+. Our ROR13 walker returns the forwarder string's
  address, not code — calling it crashes. The loader uses an
  inline `loader_memcpy` instead. Same trap applies to
  `kernel32!HeapAlloc` on some SKUs; `BeaconFormatAlloc/Free`
  uses `VirtualAlloc/VirtualFree` for that reason.
- The rundll32 helper is terminated after every BOF invocation,
  so per-call leaks (Format buffer page, KV pool, thread stack)
  are reclaimed by process teardown.
- BOFs that import unknown Beacon symbols surface as
  `LOADER_STATUS_LOAD_FAIL` with `error_code = 0x10012`
  (unresolved external). Add a new hash + dispatch entry to
  `beacon_resolve` in `loader.c` to extend coverage.
