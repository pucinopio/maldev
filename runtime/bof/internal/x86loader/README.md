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

## ABI (`abi.h` ↔ `runtime/bof/x86control_windows.go`)

The parent (Go) and the loader (C) communicate through a shared
control block placed in the child process by `VirtualAllocEx`.
The struct is mirrored byte-for-byte on both sides — any change in
`abi.h` MUST land in the Go mirror in the same commit, with a
`BOF_CTRL_VERSION` bump if the change is wire-incompatible.

| Field | Direction | Purpose |
|---|---|---|
| `magic` | parent → loader | `0x36384342` (`'BC86'`). Refuses corrupted blocks. |
| `version` | parent → loader | `1`. Loader returns `BOF_STATUS_ABI_MISMATCH` on mismatch. |
| `status` | loader → parent | One of the `BOF_STATUS_*` codes; populated before thread exit. |
| `error_code` | loader → parent | OS-level last-error / SEH code when `status` is an error. |
| `bof_addr` / `bof_len` | parent → loader | Remote address + length of the BOF `.o` bytes. |
| `args_addr` / `args_len` | parent → loader | Remote address + length of the `BeaconDataPack` blob. |
| `entry_off` | parent → loader | 0 = use `"go"` symbol; non-zero = explicit offset. |
| `spawn_to_addr` | parent → loader | NUL-terminated UTF-8 SpawnTo string (0 = none). |
| `user_data_addr` / `user_data_len` | parent → loader | `BeaconGetCustomUserData` blob (0 = none). |
| `out_addr` / `out_capacity` / `out_len` | parent allocates; loader fills `_len` | BOF stdout. |
| `err_addr` / `err_capacity` / `err_len` | parent allocates; loader fills `_len` | `BeaconErrorD/DD/NA`. |
| `reserved[16]` | — | Reserved. Keeps the struct fixed-size across minor bumps. |

## Threat model + opsec

The default injection path (parent writes DLL to disk under
`%TEMP%`, then `CreateRemoteThread → LoadLibraryA`) trades stealth
for correctness — the OS PE loader resolves the loader's own
Win32 imports for free. The disk artefact + the `LoadLibraryA`
call ARE detectable by image-load-event sensors; that's
acceptable for the phase-C-step-0 skeleton because the goal is
to validate the IPC end-to-end before optimising opsec.

Phase D (queued) will swap the loader-injection path for a
reflective injector — same DLL bytes, no disk artefact, no
`LoadLibraryA` API trail. The DLL is reflective-friendly because
it has no static imports beyond `kernel32` (resolved manually by
the reflective stub).

## See also

- `abi.h` — wire format definition (source of truth).
- `loader.c` — current skeleton; replace with the full parser in
  phase C step 1.
- `scripts/build-bof-x86-loader.sh` — build invocation.
- [BOF loader revamp plan](../../../../.dev/refactor-2026/bof-loader-revamp-plan.md).
