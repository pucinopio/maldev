# `runtime/bof/internal/x86loader` — WoW64 BOF loader DLL

32-bit (i386) DLL injected into a SysWOW64 helper process by the
parent x64 implant so a Beacon Object File compiled for x86 can
execute despite the implant being 64-bit. Slice 1.d phase C of the
[BOF loader revamp](../../../../.dev/refactor-2026/bof-loader-revamp-plan.md).

## Build

```bash
# From repo root. Uses host i686-w64-mingw32-gcc when present,
# falls back to a Podman container (fedora:42 + mingw32-gcc) for
# reproducibility on hosts that don't ship the toolchain.
bash scripts/build-bof-x86-loader.sh
```

The build produces `bof_x86_loader.x86.dll` in this directory. The
file is committed to the repo (same model as
`runtime/pe/internal/noconsolation/NoConsolation.x64.o` and
`kernel/driver/rtcore64/RTCore64.sys`) so operators don't need a
toolchain at runtime; the script exists for rebuilds when the
ABI bumps or a new feature lands.

## Architecture — rundll32-as-host

The orchestrator never injects into a separate process. Instead it
spawns `SysWOW64\rundll32.exe` against the loader DLL, and rundll32
handles `LoadLibrary` + `GetProcAddress` + the calling convention:

```
parent (x64 Go)                    helper (x86 rundll32)
─────────────────                  ─────────────────────
1. write .dll → %TEMP%\ld…dll
2. write bof  → %TEMP%\b…bin
3. write args → %TEMP%\a…bin
4. CreateProcess(
     SysWOW64\rundll32.exe,
     "%TEMP%\ld…dll,BOFExec
      v=1 bof=… args=… out=… err=…")
                                    LoadLibrary(loader.dll)
                                    GetProcAddress("BOFExec")
                                    BOFExec(HWND, HINST, lpCmdLine, nCmdShow)
                                      ├─ parse lpCmdLine tokens
                                      ├─ read bof / args files
                                      ├─ run BOF in-process       ← phase C step 1
                                      ├─ write out / err files
                                      └─ ExitProcess(BOF_EXIT_*)
5. WaitForSingleObject(rundll32)
6. ReadFile(out, err)
7. RemoveAll(%TEMP%\…)
```

## ABI (`abi.h` ↔ `runtime/bof/x86fork_windows.go`)

The wire format is the single ASCII line passed to rundll32 as
the post-comma argv, plus the helper's exit code as the structured
status word. No shared memory, no control block.

| Token | Direction | Purpose |
|---|---|---|
| `v=1`           | parent → loader | Protocol version. Mismatch → `BOF_EXIT_BAD_VERSION`. |
| `bof=<path>`    | parent → loader | Temp file with the BOF `.o` bytes. |
| `args=<path>`   | parent → loader | Temp file with the BeaconDataPack blob (may be 0-byte). |
| `out=<path>`    | parent ← loader | Temp file the loader truncates + fills with BOF output. |
| `err=<path>`    | parent ← loader | Temp file the loader truncates + fills with BeaconError*. |
| `spawnto=<path>` | parent → loader | Optional NUL-terminated UTF-8 SpawnTo (Phase C step 1). |
| `user-data=<path>` | parent → loader | Optional BeaconGetCustomUserData blob (Phase C step 1). |
| `entry=<symbol>` | parent → loader | Optional explicit entry symbol (default `"go"`). |

Helper exit codes: see `bof_exit_t` in `abi.h`. `0` = DONE; non-zero
maps to a structured Go error via `classifyX86Exit`.

## Threat model + opsec

The default path produces three artefacts: a 5 KB i386 DLL plus
two transient `.bin` files under `%TEMP%`. The DLL lives only for
the rundll32 lifetime — the orchestrator `os.RemoveAll`s the temp
dir on Execute return. The `rundll32 <dll>,BOFExec` command line
itself is the most visible IOC (sysmon event 1, etlw process
create).

Phase D (queued) will replace the rundll32 host with a reflective
injector — no disk artefact, no rundll32 process tree, no
LoadLibraryA API trail. The DLL is reflective-friendly because it
has no static imports beyond `kernel32` (resolved manually by the
reflective stub).

## See also

- `abi.h` — wire format definition (source of truth).
- `loader.c` — current skeleton; replace with the full parser in
  phase C step 1.
- `scripts/build-bof-x86-loader.sh` — build invocation.
- [BOF loader revamp plan](../../../../.dev/refactor-2026/bof-loader-revamp-plan.md).
