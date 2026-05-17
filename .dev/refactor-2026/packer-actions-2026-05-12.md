---
status: in-progress
created: 2026-05-12
last_reviewed: 2026-05-16
reflects_commit: 1.B.1.a refactor
---

# Packer — action plan tracker (2026-05-12)

> Live tracker for the prioritised improvements coming out of
> [`packer-improvements-2026-05-12.md`](./packer-improvements-2026-05-12.md).
> Update on every commit that ships a row.

## Active priority queue

| # | Item | Effort | Status | Tag |
|---|---|---|---|---|
| **1** | **Mode 8 args injection — `DefaultArgs` opt + `RunWithArgs` export** | ~200 LOC | 🟢 in progress (1.A done, 1.B scoped) | v0.130/0.131 |
| **9** | **E2E PrivEsc DLL hijack proof** — `lowuser` shell → SYSTEM whoami marker. All sub-slices (9.1-9.4 scaffolding, 9.5 README, 9.6.a-g gap-closures, 9.7.a/b helpers, 9.8.a-c STRONG verdict) shipped. Re-verified 2026-05-17 (STRONG SUCCESS — marker `autorité nt\système` from `lowuser`). | ~2300 LOC across orchestrator/probe/victim/driver/PowerShell/docs | ✅ shipped | v0.132.0/v0.133.0 |
| 2 | Mode 7 + Compress symmetry with Mode 8: `EmitDLLStub` now takes `EmitOptions` and emits the shared `emitLZ4DecompressBlock` after SGN rounds. `InjectStubDLL` widens the appended stub section's VirtualSize by `plan.StubScratchSize` (mirrors `pe.go`) and shifts `relocRVA` past the scratch slack. `ErrCompressDLLUnsupported` removed; `TestPackBinary_FormatWindowsDLL_RejectsCompress` flipped to `_AcceptsCompress`; new VM E2E `TestPackBinary_FormatWindowsDLL_LoadLibrary_Compress_E2E`. | ~70 LOC + 1 E2E | ✅ shipped |
| 3 | Stub section name randomized by default (`KeepDefaultStubSectionName` opts back into ".mldv") | ~85 LOC w/ tests | ✅ shipped (c86fb1e) | — |
| 4 | PE32+ Machine check explicite: `transform.ValidateAMD64PE32Plus` + `ErrUnsupportedMachine`/`ErrUnsupportedOptMagic`, called at FormatPE detection in `stubgen.Generate`. Rejects non-amd64 or PE32 inputs with a readable error instead of silently producing broken output. | ~85 LOC w/ tests | ✅ shipped | — |
| 5 | Walker interface unifié (R2 in audit) — implemented properly via a sealed `DirectoryPatchEvent` sum (`RVAFileOffEvent` for Import/Resource, `BaseRelocEvent` for BASERELOC carrying block context + pre-resolved PtrSize). `DirectoryWalker` function type + `DirectoryWalkers` registry indexed by `IMAGE_DIRECTORY_ENTRY_*`. New `ApplyRVAShiftAllDirectories(pe, out, delta, rvaToFile)` collapses ShiftImageVA's three manual fixup passes into one loop; signature takes both `pe` (pre-shift for header reads) and `out` (post-shift write target). Future EXCEPTION/LOAD_CONFIG/EXPORT walkers plug in via a single map entry. | ~250 LOC w/ tests | ✅ shipped |
| 6 | LZ4-decompress block dedup (R1 in audit, repurposed — SGN body was already shared via `emitSGNRounds`). New `emitLZ4DecompressBlock(b, opts, errPrefix)` helper folds the register-setup + inflate + memcpy block previously duplicated between `EmitStub` and `EmitConvertedDLLStub`. Byte-identical output (pinned tests stay green). | ~70 LOC factored out | ✅ shipped | — |
| 7 | MSVC fixture provisioning on Win10 VM — **closed**. `scripts/vm-provision.sh` extended with a SYSTEM-scheduled-task that downloads + installs VS Build Tools 2022 (VCTools + Win10 SDK 19041). The install eventually completes despite the noisy bootstrapper logs — `cl.exe` lands at `BuildTools/VC/Tools/MSVC/14.44.35207/bin/Hostx64/x64/`. Fixture .dll built via `pe/packer/testdata/build_testlib_msvc.sh` (host-side driver that wraps `vcvars64.bat` + `build_testlib_msvc.cmd` in a remote .bat to dodge the cross-shell quoting hell). E2E tests `TestPackBinary_FormatWindowsDLL_MSVC_E2E` + `_MSVC_Compress_E2E` both PASS on Win10 — proves the packer preserves MSVC import tables (vcruntime140 + ucrtbase), `.pdata` SEH unwind, `__declspec(dllexport)` exports, and `/GS` stack-cookie init across both the SGN-only and Compress paths. | ~150 LOC + fixture .dll | ✅ shipped |
| 8 | Cert preservation opt-out: `PackBinaryOptions.PreserveAuthenticodeDirectory bool` — default-off keeps the v0.126.0 strip behaviour; opt-in keeps the (now-tampered) `DataDirectory[SECURITY]` pointer so operators can masquerade as a damaged-signed binary or steg-stash payload in the cert region. | ~85 LOC w/ tests | ✅ shipped | — |

## Item #1 — design notes

**User concerns:**
- (a) "verify args passed to a packed EXE are received by payload"
  → ✅ **VALIDATED** for Mode 3 (vanilla + RandomizeAll) via
    `TestPackBinary_Args_Vanilla_E2E` + `_RandomizeAll_E2E`
    (commit `0cfee1b`).
- (b) "set default args for a program transformed into a DLL"
  → 🟢 **IN PROGRESS** — implementing `DefaultArgs` opt for Mode 8.

### Two-part feature

**Part A — pack-time default args** (`PackBinaryOptions.ConvertEXEtoDLLDefaultArgs string`):
The operator bakes a default command-line into the packed DLL.
On `DllMain(PROCESS_ATTACH)`, before `CreateThread` spawns the
OEP, the stub patches `PEB.ProcessParameters.CommandLine` to
point at the baked args. Payload's `GetCommandLineW` /
`os.Args` returns operator-controlled values.

**Part B — runtime `RunWithArgs` export**:
The packed DLL also exposes a `RunWithArgs(LPCWSTR args)`
exported function. Operator can invoke it via
`GetProcAddress` + indirect call to spawn the payload with
custom args at any time, regardless of the default. Useful
for repeat invocations or when the operator wants to chain
the payload with their own args mid-attack.

### Implementation slices

| Slice | Scope | LOC | Status |
|---|---|---|---|
| 1.A.1 | PEB-patch asm helper (`stage1.EmitPEBCommandLinePatch`) | ~80 | ✅ shipped (2a89369) |
| 1.A.2 | Wire DefaultArgs into `EmitConvertedDLLStub`: emit PEB patch BEFORE CreateThread, append args buffer to stub section | ~60 | ✅ shipped |
| 1.A.3 | Plumb `PackBinaryOptions.ConvertEXEtoDLLDefaultArgs` → `stubgen.Options` → `stage1.EmitOptions` | ~20 | ✅ shipped |
| 1.A.4 | Win10 VM E2E: pack `probe_args.exe` with DefaultArgs="custom one two", LoadLibrary, assert marker contains "custom\|one\|two" | ~50 | ✅ PASS on Win10 VM (after asm pivot) |
| 1.A.5 | **Harden: runtime overflow guard.** Asm reads existing `MaximumLength` at +0x72 BEFORE memcpy; if `argsLen+2 > existing`, skip patch. Asm 43→48 B (+ MOVZX/CMP/JB; dropped MaxLength write — capacity is OS-allocated, not ours). | ~30 LOC | ✅ shipped |
| 1.A.6 | **Test-surface gaps.** (a) Tighten 1.A.4 to exact equality. (b) Pack-time bound (`maxConvertEXEtoDLLDefaultArgsRunes = 1500`) with readable error. (c) `LargeButValid` E2E — empirically PROVED guard fires on rundll32 with 1400 chars (loader has only ~135 B cmdline, our patch needs 2800 B → JB taken, payload safely sees rundll32 cmdline). Win11 VM not provisioned on this host — skipped. Custom small-cmdline fixture turned out unnecessary since rundll32 already triggered the path. | ~80 LOC | ✅ shipped (Win11 deferred) |
| 1.B.1.a | **Refactor**: extract resolver + PEB-patch + CreateThread block from `EmitConvertedDLLStub` into `emitConvertedSpawnBlock(b, plan, opts, args convertedSpawnArgs)` helper. Sealed-iface `convertedSpawnArgs` with one impl `convertedSpawnArgsTrailing{lenBytes}`. Byte-for-byte identical output — pinned 509-B test still passes. | ~130 | ✅ shipped |
| 1.B.1.b | Add `EmitPEBCommandLinePatchRCX` (runtime wcslen + src=RCX variant, pinned 66 B, 17 instructions) + plumb the second `convertedSpawnArgs` impl (`convertedSpawnArgsFromRCX`). Dispatch in `emitConvertedSpawnBlock` via type switch. 3 unit tests (budget, AssemblesCleanly, NoSentinels). | ~150 | ✅ shipped |
| 1.B.1.c.1 | Plumb `ConvertEXEtoDLLRunWithArgs bool` through `PackBinaryOptions` → `stubgen.Options` → `stage1.EmitOptions` (no-op until 1.B.1.c.2). | ~40 | ✅ shipped |
| 1.B.1.c.2 | `EmitConvertedDLLRunWithArgsEntry` minimal: 8-byte INT3 sentinel + own prologue + spill+CALL+POP+ADD (`prologueSentinelRWA = 0xCAFEBABF`) + spawn block (`convertedSpawnArgsFromRCX{}`) + restore + leave/ret. Returns hThread in RAX (Wait/ExitCode in 1.B.1.c.3). 403 B pinned, 8 tests including AssemblesCleanly + sentinel cross-checks. `PatchRunWithArgsTextDisplacement` patcher shipped in tandem. | ~370 | ✅ shipped |
| 1.B.1.c.3 | Add `WaitForSingleObject` + `GetExitCodeThread` resolves (R13 re-used per call, no stack-saved VAs needed — each resolve runs just before its CALL). hThread spilled at `[rbp-0x20]`, DWORD exit code at `[rbp-0x10]`, returned via `mov eax,[rbp-0x10]`. Pinned 912 B (was 403 B; +509 B = two resolves + shared shadow-space CALLs). New test `_ReturnsExitCode` locks 3×`call r13` shape + the EAX load. | ~110 | ✅ shipped |
| 1.B.1.c.4 | `PatchConvertedDLLRunWithArgsEntry(stubBytes)` (sentinel locator + 8×NOP replacement, returns offset for export-table builder in 1.B.1.d). `EmitConvertedDLLStub` emits the entry between DllMain `ret` and trailing data when `opts.RunWithArgs=true`. `stubgen.go` calls `PatchRunWithArgsTextDisplacement` + `PatchConvertedDLLRunWithArgsEntry` after the existing converted-DLL patchers. New tests `_LocatesAndNOPs` + `_RunWithArgs_EmbedsEntry`. | ~80 | ✅ shipped |
| 1.B.1.d | `transform.BuildDirectRVAExportData(moduleName, exportName, entryRVA, sectionRVA)` shipped — single-named-export shape where `AddressOfFunctions[0]` points at code RVA (not a forwarder). `stubgen.go` captures the patcher offset (`StubRVA + entryOff`), then post-Inject calls `NextAvailableRVA` + `BuildDirectRVAExportData` + `AppendExportSection`. Module name baked as `"packed.dll"` (loader uses real module name at runtime). New tests `_HeaderShape`, `_RejectsEmpty`, `TestPackBinary_ConvertEXEtoDLL_RunWithArgs_EmitsExport` — last asserts DataDirectory[EXPORT] non-zero + name string present + entry RVA inside non-export section. | ~150 | ✅ shipped |
| 1.B.2 | Win10 + Win11 VM E2E: pack `probe_args.exe` with `ConvertEXEtoDLLRunWithArgs=true`, run via subprocess loader (`testdata/runwithargs_loader.exe`, mingw-cross-built from `runwithargs_loader.c`), assert exact `operator.exe\|runtime\|alpha\|beta` marker content. Test gated behind `maldev_packer_run_e2e`. **Root cause found 2026-05-17**: `EmitResolveKernel32Export` used globally-scoped labels ("k32_module_loop", …) — three resolvers chained for CreateThread / WaitForSingleObject / GetExitCodeThread silently collided in `amd64.Builder.labels[name] = p`, routing every resolver's JCC refs into the LAST resolver's anchors. CreateThread call still happened to return a valid hThread (the misroute jumped to the GetExitCodeThread output `mov r13, rax`, which left R13 = whatever was current), but Wait + GetExitCode then saw hThread as invalid. Fix: per-call label suffix via `lbl := func(name) { return name + "_" + exportName }`. Side effect: 78 B smaller stub entry (rel32→rel8 for short jumps now that labels resolve locally). Plus secondary DllMain skip-spawn-when-RunWithArgs (dual-runtime hazard) + spawn-block PEB-patch-before-resolver reorder (RCX clobber hardening for FromRCX). Subprocess test pattern absorbs Go OEP's ExitProcess(0). | ~80 | ✅ shipped |

Each slice ships its own commit. Tags every successful slice
end (1.A complete = v0.130.0, 1.B complete = v0.131.0).

**Design decisions (locked 2026-05-16):**
- `RunWithArgs` calls `CreateThread` then `WaitForSingleObject` and returns the OEP exit code via `GetExitCodeThread`. Synchronous, blocks the caller.
- Shared inner emitter at the Go level (`emitConvertedSpawnBlock`) — DllMain and RunWithArgs each emit their own asm site, but both go through the same Go function to keep the spawn shape in lockstep.
- Opt-in via `PackBinaryOptions.ConvertEXEtoDLLRunWithArgs bool`. Off by default — no extra IOC when the operator doesn't need the runtime entry.

### Cross-machine resume — current state

## Item #5 — shipped 2026-05-17 (revisited)

Originally rejected on review for over-engineering (the bare-uint32
walker interface couldn't model BASERELOC entries without lossy
narrowing). User re-asked to ship properly; the redesign that
landed honours the shape difference via a sealed sum type:

- `DirectoryPatchEvent` (sealed interface, `isDirectoryPatchEvent`
  marker).
- `RVAFileOffEvent{FileOff}` — for IMPORT/RESOURCE descriptors.
- `BaseRelocEvent{BlockOff, BlockVA, EntryIdx, RVA, Type, PtrSize}`
  — preserves the block-shaped context, plus PtrSize pre-resolved
  from Type (0 for padding, 4 HIGHLOW, 8 DIR64).
- `DirectoryWalker` function type + `DirectoryWalkers` registry
  indexed by `IMAGE_DIRECTORY_ENTRY_*`. Three current entries
  (Import, Resource, BaseReloc); new walkers plug in as a single
  map entry.

`ApplyRVAShiftAllDirectories(pe, out, delta, rvaToFile)` is the
unified delta-applier: walks every registered walker, dispatches on
the event variant, applies the right read-modify-write at the right
byte width. The pre-shift `pe` vs post-shift `out` split is
deliberate — `pe` carries the OLD section table the directory
walkers need for descriptor traversal; `out` is where the bumped
RVAs land.

`ShiftImageVA`'s three manual fixup passes collapsed into one
loop via this helper. All existing tests pass unchanged; new
unit tests cover registry shape, BASERELOC event yield, end-to-end
patch counting, and resolver-error propagation.

## Item #9 — E2E PrivEsc DLL hijack chain

**Goal.** Validate the entire packer chain (Mode 8 ConvertEXEtoDLL,
optional Mode 10 PackProxyDLL) end-to-end on a real Win10 VM:
attacker is a non-admin shell (`lowuser`), defender is a SYSTEM
scheduled task running a deliberately-vulnerable EXE that
`LoadLibrary`s a DLL from a user-writable directory. Success =
the marker file shows `nt authority\system` (or the elevated user)
written by code that originated as a packed maldev EXE the
attacker compiled.

### Sub-slices

| # | Scope | LOC | Status |
|---|---|---|---|
| 9.1 | **VM provisioning** — `scripts/vm-test/provision-lowuser.ps1` + `provision-privesc.ps1` create `lowuser`, `C:\Vulnerable\`, `victim.exe`, SYSTEM scheduled task `MaldevHijackVictim` with lowuser /Run ACL, Defender exclusions for `C:\Vulnerable\` + `C:\ProgramData\maldev-marker\`. | ~230 (PowerShell) | ✅ shipped |
| 9.2 | **Probe** — `cmd/privesc-e2e/probe/main.go` (Go-based) + checked-in `probe.exe`. Calls `GetUserName` + writes `<identity>\|pid=<pid>` to `C:\ProgramData\maldev-marker\whoami.txt`, then sleeps so the host victim has time to flush. Slice 9.6.d.x dynamically resolves `GetUserNameA` to keep the Go runtime minimal. | ~80 | ✅ shipped |
| 9.3 | **Orchestrator** — `cmd/privesc-e2e/main.go` (299 LOC). Modes 8 (ConvertEXEtoDLL) + 10 (PackProxyDLL via `packer.PackProxyDLLFromTarget`), live `recon/dllhijack` discovery, marker poll, SUCCESS/FAIL verdict. Boots with `evasion/preset.Aggressive` (ACG + BlockDLLs + AMSI/ETW unhook) via `amsi_windows.go`. | ~300 | ✅ shipped |
| 9.4 | **Driver** — `scripts/vm-privesc-e2e.sh` (294 LOC). Auto-detects vbox/libvirt, restores INIT snapshot, builds binaries on host, SCPs as admin, provisions, runs orchestrator AS lowuser via `run-as-lowuser.ps1`, fetches marker + victim.log, prints STRONG/ADEQUATE/FAIL verdict. | ~290 | ✅ shipped |
| 9.5 | **User doc** — `cmd/privesc-e2e/README.md` pedagogical end-to-end walkthrough with mermaid attack chain diagram, cast of binaries, per-step state changes, detection & forensics, Defender bypass section, troubleshooting. Replaces sibling doc plan; lives next to the orchestrator. | ~600 (md) | ✅ shipped |

### Open answers (confirmed defaults)

- Hijack vector: DLL search-order (victim's own dir first).
- Trigger: `schtasks /Run` with lowuser-writable ACL on the task.
- Snapshot: NEW `INIT-PRIVESC` (do not mutate existing INIT).
- Pack mode: 8 (ConvertEXEtoDLL) — victim only LoadLibrary's, no exports needed.
- Bitness: x64 only.
- Marker dir: `C:\ProgramData\maldev-marker\` (default ACL).

### Cross-machine resume

After commit, pickup at next unticked sub-slice.

### Sub-slice 9.6 — close all open gaps from session 2026-05-12

Identified after the dec0466 / 8b1a1ec checkpoints. Each gap gets
its own commit so cross-machine resume always picks up at the next
checkbox.

| # | Gap | Approach | Status |
|---|---|---|---|
| 9.6.a | Defender exclusions for `C:\Vulnerable\` + `C:\ProgramData\maldev-marker\`. v2 settled on AMSI-bypass-then-registry-write inside `provision-privesc.ps1`. | `fa9605c` switched to registry-direct after the AMSI dance proved flaky. | ✅ shipped |
| 9.6.b | AMSI bypass via `evasion/amsi.PatchAll` integrated in orchestrator. | `65654ec` initial integration, `2061cab` swapped Defender-exclusions-at-orchestrator-time for `preset.Stealth`, later upgraded to `preset.Aggressive` (9.8.b). | ✅ shipped |
| 9.6.c | Marker 0-byte: probe `FlushFileBuffers` + `FILE_SHARE_READ` + 200 ms sleep before SleepInfinite. | `37daea5`. | ✅ shipped |
| 9.6.d | Big binary AS lowuser RC=1 — root-caused to provisioning password mismatch + poll-timeout race. Fixed via `b6d26c8` (force-set password via `net user`) + `a20941b` (140 s poll + 70 s post-orch sleep) + `b6e1298` (probe dyn-resolves `GetUserNameA`). | Series of 9.6.d.1/d.2/d.x commits. | ✅ shipped |
| 9.6.e | Go probe in injected thread — initial probe was C; final probe is Go (commit `bf4e8a1` "probe in Go, victim in C — STRONG E2E") proving the Go-in-injected-thread story works end-to-end with the Aggressive preset. | The "document Go-incompat" alternative is moot — Go-probe works. | ✅ shipped |
| 9.6.f | Final E2E run, both modes, STRONG verdict. | `3a67ae9` "full chain green on Fedora/libvirt — STRONG verdict". Tagged `v0.132.0`. | ✅ shipped |
| 9.6.g | User-facing doc — pedagogical README at `cmd/privesc-e2e/README.md` (pure-Windows recipe + Detection & Forensics + Defender bypass section), `9397acc` dropped all VM/driver/hypervisor references for pure-operator audience. | Lives next to the orchestrator, not under `docs/techniques/`. | ✅ shipped |

### Sub-slice 9.7 — extract reusable helpers from privesc-e2e patterns

Audit revealed two patterns in `cmd/privesc-e2e` that should become
exported helpers in their respective packages so the next operator
tool doesn't reinvent them.

| # | Helper | Lives in | Replaces |
|---|---|---|---|
| 9.7.a | `packer.PackProxyDLLFromTarget` — ✅ shipped `12cc47c`. Plus `evasion.ApplyAllAggregated` shipped same commit. Orchestrator adopts both via `73146aa`. |
| 9.7.b | `dllhijack.PickBestWritable` + sentinel — ✅ shipped `bb5549a`. Plus `recon/dllhijack/ScanPATHWritable` for MareBackup-class EXE search hijack (`e94858b`) and ApiSet contract exclusion (`41cb0fc`). |

### Sub-slice 9.8 — close gaps 2 (probe race) + 3 (Defender) + 4 (verdict)

| # | Gap | Approach | Status |
|---|---|---|---|
| 9.8.a | Probe race — victim sleeps 5 s after LoadLibrary. ✅ shipped `d75e9c4`. |
| 9.8.b | Defender flagging — `preset.Aggressive` (ACG + BlockDLLs on top of AMSI/ETW/unhook). ✅ shipped `1b7da1e`. |
| 9.8.c | Verdict ADEQUATE → STRONG. ✅ confirmed `3a67ae9` "full chain green — STRONG verdict". Re-verified 2026-05-17 (this session): SUCCESS, marker shows `autorité nt\système` from the `lowuser` shell. |

### Bonus shipped during the 9.x stream

| commit | what |
|---|---|
| `b4aa4b9` | packer-cli flags `-compress`/`-antidebug`/`-randomize` + Defender bypass docs section |
| `12cc47c` | `evasion.ApplyAllAggregated` + `packer.PackProxyDLLFromTarget` factored out of orchestrator |
| `bb5549a` | `recon/dllhijack.PickBestWritable` |
| `e94858b` | `recon/dllhijack.ScanPATHWritable` (MareBackup-class EXE search hijack) |
| `edac9c1` | `pe/packer/transform.InjectStubPE` marks stub section MEM_WRITE when `StubScratchSize > 0` (root-caused by privesc-e2e Mode 8 + Compress crashes) |
| `bf4e8a1` | Final state: probe in Go, victim in C, STRONG E2E end-to-end |

### Sub-slice 9.7 — design notes per helper

**9.7.a `packer.PackProxyDLLFromTarget(payload, targetDLLBytes, opts) (proxy, key []byte, err error)`**

Lives next to `PackProxyDLL` in `pe/packer/proxy_fused.go`. Body:

```go
func PackProxyDLLFromTarget(payload []byte, targetDLLBytes []byte, opts ProxyDLLOptions) (proxy, key []byte, err error) {
    pf, err := parse.FromBytes(targetDLLBytes, "<embedded-target>")
    if err != nil {
        return nil, nil, fmt.Errorf("packer: PackProxyDLLFromTarget parse target: %w", err)
    }
    entries, err := pf.ExportEntries()
    if err != nil {
        return nil, nil, fmt.Errorf("packer: PackProxyDLLFromTarget exports: %w", err)
    }
    var exports []dllproxy.Export
    for _, e := range entries {
        if e.Name == "" { continue } // skip ordinal-only
        exports = append(exports, dllproxy.Export{Name: e.Name, Ordinal: e.Ordinal})
    }
    if len(exports) == 0 {
        return nil, nil, fmt.Errorf("packer: PackProxyDLLFromTarget target has no named exports")
    }
    o := opts
    if o.TargetName == "" {
        return nil, nil, fmt.Errorf("packer: PackProxyDLLFromTarget TargetName required (cannot infer from binary)")
    }
    o.Exports = exports
    return PackProxyDLL(payload, o)
}
```

Tests: `TestPackProxyDLLFromTarget_MirrorsExports` (pack a known fixture, parse the proxy, assert exports match input).
Doc: extend the comment block on `PackProxyDLL` with a reference to this helper.

**9.7.b `dllhijack.PickBestWritable(opts ScanOpts) (*Opportunity, error)`**

Lives in `recon/dllhijack/dllhijack.go`. Body:

```go
func PickBestWritable(opts ScanOpts) (*Opportunity, error) {
    all, err := ScanAll(opts)
    if err != nil {
        return nil, fmt.Errorf("dllhijack: PickBestWritable ScanAll: %w", err)
    }
    ranked := Rank(all)
    // Prefer writable + integrity-gain
    for i := range ranked {
        if ranked[i].Writable && (ranked[i].IntegrityGain || ranked[i].AutoElevate) {
            return &ranked[i], nil
        }
    }
    // Fallback: any writable
    for i := range ranked {
        if ranked[i].Writable {
            return &ranked[i], nil
        }
    }
    return nil, fmt.Errorf("dllhijack: PickBestWritable found no writable opportunity (scanned %d total)", len(ranked))
}
```

Tests: stub on non-windows, real test on windows VM gates.

**9.7.c (BONUS) `evasion.ApplyAllAggregated(techs []Technique, caller *wsyscall.Caller) error`**

Companion to existing `ApplyAll` which returns `map[string]error`. Aggregates failures into single sortable-message error. Saves the same boilerplate we wrote in `cmd/privesc-e2e/amsi_windows.go`.

Lives in `evasion/evasion.go`. Tests: assert single-error path equals nil when all OK, and contains all failure names when ≥1 fails.

### Sub-slice 9.8 — brainstorm matrix per gap

**Gap 2 (probe race) — six options:**

| ID | Option | Cost | Risk |
|---|---|---|---|
| 2A | Victim sleeps 5 s after LoadLibrary | 1 LOC | none — chosen |
| 2B | Probe writes whoami.txt FIRST then breadcrumbs (single critical write) | 5 LOC | doesn't fix the race, just makes the surviving write the important one |
| 2C | Probe spawns whoami.exe via CreateProcess with redirected stdout — child outlives victim | 30 LOC C | adds advapi32-style imports to probe |
| 2D | Probe registers async I/O completion / DPC — survives ExitProcess | high LOC | Windows kernel APIs from user-mode are sketchy |
| 2E | Probe calls ExitProcess(0) itself after writes — preempts victim's cleanup race | 1 LOC | victim never gets to print its post-LoadLibrary log line; cosmetic |
| 2F | Mode 8 stub patches: DllMain blocks until spawned thread signals done | medium LOC in packer | deepest fix, benefits everyone using Mode 8 |

Recommended: **2A first (cheapest), keep 2F as a future packer enhancement**.

**Gap 3 (Defender on orchestrator binary) — six options:**

| ID | Option | Cost | Risk |
|---|---|---|---|
| 3A | preset.Aggressive (Stealth + ACG + BlockDLLs) | 1 LOC swap | ACG blocks subsequent VirtualAlloc(PAGE_EXECUTE) — must apply AFTER all our pack calls. Order matters. |
| 3B | Pack the orchestrator itself with `packer.PackBinary{Compress, AntiDebug, RandomizeAll}` (Mode 3) | host build script change | adds a build step; orchestrator becomes a packed EXE |
| 3C | Strip + UPX + reorder sections | 1 LOC ldflags | weaker than 3A/3B |
| 3D | Split orchestrator: stage1 (tiny ~10 KB loader) + stage2 (full 12 MB payload). Stage1 applies preset.Aggressive, allocates RX, decrypts + jumps stage2 | 200+ LOC | most operationally realistic; mirrors real maldev shape |
| 3E | Self-sign + add cert to Trusted Root | requires admin to install cert; fragile per machine | not portable |
| 3F | Live with no exclusions, document Defender behaviour as expected (operator tier) | 0 LOC | accepts the gap |

Recommended: **3A as a quick win** (move patchAMSI() AFTER all our VirtualAlloc-needing calls — packer is one). If 3A still flagged, **3B**.

**Gap 4 (verdict)** — auto-resolves with 2A.

**Gap 5 (driver bash quoting)** — three options:

| ID | Option | Cost | Risk |
|---|---|---|---|
| 5A | Write the full PS invocation as a heredoc-built .ps1 on the host, SCP to VM, invoke as a single argument (already partially done at scripts/vm-privesc-e2e.sh:108-110 but the heredoc still has `\${SSH_USER}` escape bugs we fixed) | 10 LOC bash | Same approach we tried; needs careful verification of heredoc expansion |
| 5B | Encode the parameters as base64, decode in PS (no quoting hell) | 15 LOC | adds a debug-unfriendly indirection |
| 5C | Replace the bash driver entirely with a Go program in `cmd/privesc-e2e-driver/` that uses crypto/ssh to SCP+exec. No shell quoting. | 200 LOC | clean but big |

Recommended: **5A first verify carefully** with `cat > tmp/run.ps1 <<EOF` then `cat /tmp/run.ps1` to inspect, then `scp` then SSH `powershell -File`. If still flaky, **5C is the proper fix**.

### Master execution order — status 2026-05-13

```
1. 9.7.a + 9.7.c (packer.PackProxyDLLFromTarget + evasion.ApplyAllAggregated)        ✅ shipped (commit 12cc47c)
2. 9.7.b (dllhijack.PickBestWritable + sentinel)                                     ✅ shipped (commit bb5549a)
   bonus: dllhijack ApiSet filter (zerotracelab/itm4n)                               ✅ shipped (commit 41cb0fc)
   bonus: dllhijack.ScanPATHWritable + KindPathHijack (MareBackup chain)             ✅ shipped (commit e94858b)
3. Refactor cmd/privesc-e2e/main.go to use the 3 helpers                             ✅ shipped (commit 73146aa)
4. 9.8.a (victim sleep 5 s after LoadLibrary)                                        ✅ shipped (commit d75e9c4)
5. 9.8.b (preset.Aggressive, audited order-of-ops)                                   ✅ shipped (commit 1b7da1e)
6. vm-privesc-e2e.sh on libvirt — STRONG verdict on -m 8                             ✅ shipped (commit 3a67ae9)
7. Mode-10 STRONG verdict (PackProxyDLLFromTarget end-to-end)                        ✅ verified live
8. tag v0.132.0                                                                      ✅ shipped (tag 99e7dd7)
9. user-facing doc — cmd/privesc-e2e/README rewritten thrice                         ✅ shipped (commits 2f9c5da, 2c9579a, 1a4aec2, 9397acc)
10. (deferred) 9.7.b extension: same scan-and-pick pattern for AutoElevate-only flows ⏳ backlog
```

**Bonus completed during this slice (out of original plan):**

- v0.133.0 — `InjectStubPE` MEM_WRITE fix unblocks `Compress` packs on large
  Go binaries. End-to-end verified: `cmd/privesc-e2e` packed with
  `-rounds 5 -compress -randomize`, Defender real-time protection ON,
  no exclusions, AS lowuser → STRONG SUCCESS marker. Two regression
  tests in `pe/packer/transform/pe_test.go`
  (`TestInjectStubPE_StubSectionWriteBit`).
- `cmd/packer` gains `-compress` / `-antidebug` / `-randomize` CLI flags
  (commit b4aa4b9 + simplify pass 21d3b81).
- `pe/packer/transform/peconst.go` exports `ScnMemWrite` / `ScnMemExec` /
  `ScnCntCode` (kills 3 magic-number sites; simplify pass).
- `cmd/privesc-e2e/README.md` gains §7-bis Detection & Forensics with
  built-in-Windows-only commands (wevtutil, Get-WinEvent, schtasks,
  certutil, fsutil USN, auditpol) + §8-bis Defender bypass via dropper
  packing.
- DiagHub SYSTEM-DLL loader (Project Zero 2018 primitive B) queued in
  memory `diaghub_privesc_loader.md` for a future slice.

**Authorship hygiene:** all 15 session commits rewritten as
`oioio-space <oioio-space@users.noreply.github.com>`, tags v0.132.0 +
v0.133.0 re-pointed, master force-pushed 2026-05-13.

### Cross-machine resume — exhaustive context dump

State at handoff:
- HEAD = 7f25ec2 (this commit will land at HEAD after the planning-doc commit).
- All 14 lessons captured in `.dev/refactor-2026/privesc-e2e-lessons-2026-05-12.md`.
- `cmd/privesc-e2e/probe/probe.exe` and `cmd/privesc-e2e/fakelib/fakelib.dll` are gitignored;
  rebuild via the README's host-side commands or just re-run `bash scripts/vm-privesc-e2e.sh`.
- Win10 VM is at INIT snapshot (clean); driver auto-restores.
- Manual SSH commands proven working at this point in time:
  - `ssh -i ~/.ssh/vm_windows_key test@192.168.56.102 'net user lowuser MaldevLow42x'` then
  - `ssh ... 'powershell -ExecutionPolicy Bypass -File C:\Users\test\run-as-lowuser.ps1 -Binary "C:\Users\Public\maldev\privesc-e2e.exe" -BinaryArgs "-mode 8" -UserName lowuser -Password MaldevLow42x -TimeoutSeconds 180'`
  - That manual flow produces full orchestrator output AND `LoadLibrary succeeded:
    handle=0x140000000` in `C:\ProgramData\maldev-marker\victim.log` after the next
    minute-trigger fires.
- The probe payload reaches main() (probe-started.txt + probe-root-marker.txt produced)
  but the third WriteFile (whoami.txt) races victim's ExitProcess. 9.8.a fixes this.

Pickup at 1. above.

---

**Slice 1.A FULLY HARDENED.** v0.130.0 shipped the feature;
follow-up slices 1.A.5 + 1.A.6 (in response to "il n'y a pas
de contournement ?") added:
- runtime asm guard (CMP existing MaxLength vs needed; JB skip)
- pack-time bound at 1500 chars with readable error
- exact-equality assertion (was Contains)
- empirical guard-firing proof (LargeButValid test on Win10 VM)

Win11 VM not provisioned on this host — deferred to whenever
the user provisions one (see `feedback_vm_testing.md`). Tag
v0.131.0 follows. Pickup at **slice 1.B.1** (`RunWithArgs`
export emitted in stub section + registered via
`transform.AppendExportSection`).

Big lesson from 1.A.4 (saved as `feedback_getcommandline_cache.md`):
the original PEB-patch design (rewrite `CommandLine.Buffer` pointer)
was a no-op because `kernel32!GetCommandLineW` caches its result on
first call — every subsequent caller (Go runtime, MSVC CRT, etc.)
reads the cache, NOT PEB. Pivoted to **in-place memcpy** at the
existing buffer pointer (43 B asm: PEB → ProcessParameters → load
existing Buffer into RDI → REP MOVSB from stub-baked args → update
Length/MaximumLength). Limitation: assumes existing buffer ≥
argsLenBytes+2; documented on the field.

The Win64 PEB layout used by the asm patch:
- `gs:[0x60]` → PEB pointer (TEB+0x60)
- `PEB+0x20` → ProcessParameters pointer
- `ProcessParameters+0x70` → CommandLine UNICODE_STRING:
  - +0x00: Length (uint16, bytes excluding null)
  - +0x02: MaximumLength (uint16, bytes including null)
  - +0x08: Buffer (PWSTR)

The patch overwrites Length, MaximumLength, Buffer with the
operator's args. Does NOT save/restore — the host process's
CommandLine stays clobbered for the duration. Documented as
an OPSEC trade-off.
