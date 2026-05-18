---
date: 2026-05-18
session: BOF revamp closure + runtime/pe + crash isolation
end_state: Slice 1 (Beacon API), Slice 2 (loader plug-in), Slice 1.c (goffloader parity), 1.c.9 (runtime/pe), Args memory, Execute split, crash isolation — all closed.
---

# Where we are

## Tags shipped this session

| Tag | What |
|---|---|
| `v0.152.0` | `runtime/pe` package (in-process PE loader via No-Consolation) |
| `v0.152.1` | section layout fix + first CS-SA E2E suite (4 BOFs) |
| `v0.152.2` | all-sections relocations bug fix (ipconfig canary) |
| `v0.152.3` | CS-SA expansion 10 → 18 BOFs |
| `v0.152.4` | CS-SA expansion 18 → 32 BOFs |
| `v0.152.5` | full Beacon API fixture coverage + admin CS-SA (5 more) |
| `v0.152.6` | Args memory 3x → 2x (chunk-list refactor) |
| `v0.153.0` | Execute split — prepare once, run many + `Close()` + `SetPersistent()` |
| `v0.154.0` (next) | Sacrificial thread + VEH crash isolation + `SetSacrificialThread()` |

## Architecture state — `runtime/bof`

Three explicit, composable knobs on `*BOF`:

```go
b, _ := bof.Load(coffBytes)
defer b.Close()

b.SetSpawnTo(`C:\Windows\System32\notepad.exe`)
b.SetUserData(payload)                      // surfaced via BeaconGetCustomUserData
b.SetPersistent(true)                       // keep .data warm across Execute calls
b.SetSacrificialThread(30 * time.Second)    // implant survives BOF crashes

for _, t := range targets {
    out, err := b.Execute(packArgs(t))
    // err covers: nil / BOF crash (isolated) / timeout / Load failure
}
```

Lifecycle:

1. `Load(data)` — validates COFF header, installs `runtime.SetFinalizer` safety net.
2. First `Execute` calls `prepare()` (private) — parse + VirtualAlloc + relocations + RW→RX flip. Snapshots writable sections. Idempotent.
3. Subsequent `Execute` calls — restore writable sections (if `!persistent`) + entry call.
4. In sacrificial mode — entry runs on dedicated OS thread; VEH intercepts faults inside BOF mapping; host gets a clean `error`.
5. `Close()` — VirtualFree mapping, disarm finalizer, refuse if sacrificial thread still in flight. Idempotent.

## Coverage — `runtime/bof` test suite

| Tier | Count | Statut |
|---|---|---|
| Unit tests (`TestArgs*`, `TestLoad_*`, `TestSection*`, `TestBOF_*Close*`, `TestBOF_SetPersistent*`, `TestBOF_SetSacrificialThread*`) | many | all PASS |
| In-tree example BOFs (`hello_beacon`, `parse_args`, `loadlib`, `realworld_calls`, `data_extras`, `format_output`, `format_extras`, `error_spawnto`, `beacon_api_complete`, `beacon_api_intrusive`) | 10 fixtures | all PASS |
| `TestBeaconAPI_FullSurfaceMatrix` — 28 canonical Beacon symbols + 1 No-Consolation extension, fixture-mapped | 1 test, 29 symbols | PASS |
| `TestCSSA_*` default (no admin needed) | 32 | all PASS host + Win10 VM |
| `TestCSSA_*` `MALDEV_INTRUSIVE=1` (admin / SAM / VSS / etc.) | 5 | all PASS |
| `TestBeaconAPI_Intrusive` (spawn / inject chain) | 1 | PASS |
| `TestBOF_SacrificialThread_*` (happy + crash isolation) | 3 | PASS |
| **Total** | **44 PASS default, 49 PASS intrusive** | host + Windows10 VM (fr-FR) |

## `runtime/pe` state

Single public API: `RunExecutable(peBytes []byte, opt Options) (string, error)`. 22-field `Options` (cmdline / method / timeout / headers / link-to-peb / load-all-deps / unload-libs / search-paths / …).

Internally now uses `bof.Load` once via `sync.Once` + `SetPersistent(true)` — the 63 KB No-Consolation `.o` is parsed + allocated + relocated **exactly once per process** instead of per-PE-execution.

E2E suite: 7/7 PASS on host + Windows10 VM (`hello.x64.dll` fixture).

## Bugs fixed this session

1. **runtime/bof wide-string framing** — `Args.AddWideString` wrote length in wchar units; `BeaconDataExtract` reads bytes. Discovered debugging No-Consolation. Fix in v0.151.1 era (within v0.152.0).
2. **No-Consolation BOOL return** — our `BeaconAddValue`/`RemoveValue` returned 0; the extension expects 1 on success. Silent failure of all No-Consolation init paths.
3. **win/api forwarders** — `ExportByHash` didn't follow `kernel32!HeapAlloc → ntdll!RtlAllocateHeap`-style forwarders. DEP fault on call.
4. **Section layout shared-page** — `.data` sat in the same 4 KB page as `.text`; RW→RX flip protected the whole page, breaking writes to `.data` globals. Fixed by two-pass layout (exec first, page-align gap, non-exec).
5. **All-sections relocations** — `.rdata` ADDR64 pointer tables weren't relocated (only `.text` was). Caused ipconfig.x64.o (239 .rdata relocs) to crash on first table lookup. Loader now applies relocs for every section that has them.
6. **No-Consolation packer 27 → 28 fields** — initial spec missed `unload_libs`, shifting every later field by one byte.
7. **PAT in env BOF output** — test surfaced operator's `GITHUB_PERSONAL_ACCESS_TOKEN`. Test now redacts output on failure.
8. **Args.Pack double-copy** — `bytes.Buffer` internal + `Pack` output = 3× peak for big payloads. Refactored to chunk list = 2× peak.
9. **VEH exit stub rsp alignment** — `jmp ExitThread` left rsp misaligned; `ExitThread` prologue can fault on movaps. Fixed with `and rsp,-16; sub rsp,0x28; call rax`.
10. **Close vs sacrificial thread race** — `Close()` freed mapping while sacrificial thread might still execute it. Now refuses with error if sacrificial entry registered.

## Build / test status as of this status note

- Host (Windows): `go test ./... ` and `go test -tags=pe_noconsolation ./...` both clean
- VM Win10 (French locale): all `runtime/bof` + `runtime/pe` test suites PASS
- CI: green on every push to `master` since v0.151.0 (verified via `gh run list`)
- Releases: created via `gh release create` for every tag in this session

## Open follow-ups (not blocking)

| Item | Priority | Note |
|---|---|---|
| Build a shared per-process trampoline (replaces per-call `buildEntryThunk`) | low | Saves 3 syscalls + a 4 KB page allocation per `Execute` call in sacrificial mode. Efficiency-review-flagged. Cosmetic vs the actual Wait latency. |
| Reclaim leaked thunk pages on `Close` when timeout fired | low | Requires synchronous "thread really stopped" signal Windows doesn't expose. Leak grows ~4 KB per sacrificial timeout. |
| `.x86.o` BOF support (32-bit cross-process Beacon BOF) | medium | Out of scope for the v0.15x cycle. `BeaconGetSpawnTo(BOOL bX86)` is already wired on the .o-handling side; needs an x86 loader path. |
| Slice 3 of the original revamp plan: `goloader` integration | medium | Run Go-native modules instead of CS-style BOFs. Architectural change. |
| Slice 4: `.gof` custom format | medium | Full spec in `.dev/refactor-2026/bof-loader-revamp-plan.md` Appendix B. |
| Slice 5: per-format build-tag gating | low | Mostly docs once slice 3+4 land. |

## How to resume on a different machine

```bash
git pull
# all .o test fixtures are committed; nothing else needs building.

# run the full suite (host):
go test ./runtime/... ./win/api/

# run with intrusive admin tests:
MALDEV_INTRUSIVE=1 go test ./runtime/bof/

# run on Windows10 VM:
./scripts/vm-run-tests.sh windows "./runtime/bof/... ./runtime/pe/..." \
    "-count=1 -tags=pe_noconsolation"
```

If a `runtime/pe` E2E fails because `runtime/pe/internal/noconsolation/NoConsolation.x64.o`
is missing (shouldn't be — it's committed): `bash scripts/build-no-consolation.sh`.

If a CS-SA test fails because `runtime/bof/testdata/cs-sa/*.o` is missing:
`bash scripts/fetch-cs-sa-bofs.sh` (gitignored on purpose: GPL-2 upstream).
