---
status: in-progress
opened: 2026-05-17
owner: oioio-space
scope: runtime/bof (+ new packages)
references:
  - runtime/bof/beacon_api_windows.go (current API surface)
  - https://hstechdocs.helpsystems.com/manuals/cobaltstrike/current/userguide/content/topics/beacon-object-files_main.htm
  - https://github.com/praetorian-inc/goffloader
  - https://github.com/Binject/universal (Plan9 reflection)
  - https://github.com/eh-steve/goloader
  - https://github.com/tetratelabs/wazero
slices:
  - id: 1
    title: Beacon API completion
    status: closed
    commits:
      - ed07614  # 12 new symbols (Groups 3/4/5/6) + LockOSThread
      - ab72b6e  # behavioural tests + CI honesty (audit closure)
    vm_e2e: pass (Win10 INIT, 50+ tests, 0.165s)
  - id: 1.b
    title: Gap closure — varargs, x86 SpawnTo, lpParameter
    status: closed
    commits:
      - 'e3e63b2'  # this commit — printf_windows.go + SpawnToX86 + arg lpParameter
    vm_e2e: pass (Win10 INIT, 76 tests incl. 13 new, 0.297s)
  - id: 1.c
    title: goffloader-parity-and-then-some (see goffloader-comparison.md)
    status: closed (11/11 items — 1.c.9 runtime/pe landed 2026-05-18)
    commits:
      - ca32d94  # batch A-D — 1.c.1/2/3/4/5/6/8/10 + realworld_calls fixture
      - 9a4e381  # batch E — 1.c.7 ExecuteStream + token-mask fix + AddWideString
      - HEAD     # 1.c.9 runtime/pe — No-Consolation wrapper + 27-field packer + tests
    vm_e2e: pass (Win10 INIT, full runtime/bof + new runtime/pe suite)
    items_closed:
      - 1.c.1 string-obfuscate Beacon import names (rune-array trick + verified hidden via `strings | grep` returning 0)
      - 1.c.2 vararg capture bumped 6 → 10 (goffloader parity)
      - 1.c.3 wide-string %s heuristic (UTF-16 detection in expandCFormat)
      - 1.c.4 RW→RX section flip via VirtualProtect after relocations
      - 1.c.5 MEM_TOP_DOWN VirtualAlloc flag
      - 1.c.6 panic recover around BOF entry call
      - 1.c.7 (*BOF).ExecuteStream async channel API
      - 1.c.8 BeaconGetOutputData symbol (both packages were missing it)
      - 1.c.9 runtime/pe — RunExecutable wraps No-Consolation BOF (MIT, embed gated via `pe_noconsolation`); 28-field bofdata packer (verified by witness test against fortra/No-Consolation @ dbdb16b); build script at scripts/build-no-consolation.sh; .o committed to repo (rtcore64 model); 7 E2E tests passing on Windows10 VM via hello.x64.dll fixture. Surfaced + fixed three upstream bugs in the slice: bof.Args.AddWideString wrote length in wide-units instead of bytes (consumer reads bytes); BeaconAddValue/RemoveValue returned 0 (No-Consolation extension expects BOOL); win/api.ExportByHash didn't resolve forwarders (HeapAlloc → ntdll!RtlAllocateHeap on Win 8+). Tech md at docs/techniques/runtime/pe-loader.md
      - 1.c.10 cmd/bof-runner -arg type-prefixed CLI (i/s/z/Z/b)
      - 1.c.11 garble -literals already wired in make release
    items_deferred: []
    followups_runtime_bof:
      # Surfaced by the simplify/efficiency review of 1.c.9 — these are
      # upstream improvements that benefit every BOF consumer, not just pe.
      - status: closed
        item: bof.Args.Pack double-copy
        commit: 4636bbf+  # next commit after the runtime/bof CS-SA suite
        notes: |
          Args refactored from bytes.Buffer to a chunk list. AddBytes
          now references the caller's slice (no copy); Pack does the
          single concat into a fresh output. Peak memory dropped from
          3x peBytes to 2x. TestArgsAddBytes_ReferenceContract pins
          the new behaviour. Pack-isolation contract preserved.
      - status: closed
        item: bof.Run Load/Execute caching (Execute split)
        commit: cc9468e+  # next commit after Args memory fix
        notes: |
          Execute split into prepare() (parse + alloc + reloc +
          protect, idempotent, runs once per BOF) and the entry call
          (cheap, runs per Execute). Mapping is now owned by *BOF
          and survives across Execute calls; Close() releases it
          explicitly, runtime.SetFinalizer is the safety net.
          SetPersistent(bool) knob arbitrates the stateful-vs-stateless
          dilemma: default false zeroes writable sections between
          Execute calls (matches hello_beacon / parse_args assumption);
          true keeps the state (No-Consolation LIBS_LOADED + handle
          info cache). runtime/pe caches the prepared No-Consolation
          BOF via sync.Once with SetPersistent(true) — RunExecutable
          now amortises the .o load across the process lifetime.
          Verified: 38 default + 5 admin tests on host + VM, plus
          new lifecycle suite (Close idempotent, post-Close error,
          ExecuteTwice, SetPersistent default).
    sub_items:
      - 1.c.1 string-obfuscate Beacon import names
      - 1.c.2 bump vararg capture from 6 to 10
      - 1.c.3 wide-string %s heuristic in expandCFormat
      - 1.c.4 RW→RX section flip via VirtualProtect (drop RWX)
      - 1.c.5 MEM_TOP_DOWN VirtualAlloc flag
      - 1.c.6 panic recovery wrapper around BOF entry
      - 1.c.7 async output streaming API ExecuteStream
      - 1.c.8 BeaconGetOutputData symbol
      - 1.c.9 runtime/pe — embed No-Consolation, RunExecutable
      - 1.c.10 cmd/bof-runner — type-prefixed CLI args
      - 1.c.11 garble literal obfuscation in make release
  - id: 2
    title: Loader-format plug-in
    status: closed
    commits:
      - 3edaeda  # loader_windows.go + Run/Spec/Kind/DetectKind + 7 tests
    vm_e2e: pass (Win10 INIT, 7 tests incl. table-driven, 0.453s)
  - id: 1.d
    title: x86 (.x86.o) BOF support
    status: in-progress
    phases:
      - phase: A
        title: detection layer + clean cross-arch error path
        status: closed
        commits:
          - HEAD  # this commit
        deliverable: |
          KindCOFFx86 constant + magic-byte detection (0x014c).
          coffX86Loader registered as the dispatch target; today it
          surfaces ErrCrossArchX86Unsupported. bof.Load on an x86 .o
          wraps the same sentinel instead of leaking the raw machine
          hex code. Tests pin both Run() and Load() paths.
      - phase: B
        title: fork-and-run orchestrator (Go side)
        status: queued
        scope: |
          Spawn BeaconGetSpawnTo(TRUE) host (default
          C:\Windows\SysWOW64\rundll32.exe) suspended, VirtualAllocEx
          + WriteProcessMemory three regions (x86 loader DLL, the
          BOF .o + args, the shared control block + output buffer),
          launch via CreateRemoteThread, WaitForSingleObject, then
          ReadProcessMemory the captured output. TerminateProcess +
          CloseHandle on teardown.
      - phase: C
        title: x86 loader DLL (C, mingw32)
        status: queued
        scope: |
          runtime/bof/internal/x86loader/loader.c — exports
          RunBOF(bofBytes, bofLen, args, argsLen, ctrl). Implements
          the Beacon API in-child against a shared control block.
          Build script (scripts/build-bof-x86-loader.sh), .dll
          committed to the repo, embed gated behind
          //go:build bof_x86_loader (same pattern as
          pe_noconsolation in runtime/pe).
  - id: 3
    title: goloader integration
    status: queued
  - id: 4
    title: .gof custom format
    status: queued
  - id: 5
    title: Build-tag gating + docs
    status: queued
---

# BOF loader revamp

## Why we're not satisfied with the current loader

`runtime/bof` (2 153 LOC) covers the **easy** half of `beacon.h`:

- ✅ Group 1 — Data parsing (`BeaconDataParse/Int/Short/Length/Extract`)
- ✅ Group 2 — Format / output (`BeaconFormat*`, `BeaconPrintf`, `BeaconOutput`, `BeaconError*`)
- ⚠️ Group 4 — Only `BeaconGetSpawnTo`; **missing the four real injection primitives**
  (`BeaconInjectProcess`, `BeaconInjectTemporaryProcess`,
  `BeaconSpawnTemporaryProcess`, `BeaconCleanupProcess`)
- ❌ Group 3 — Token (`BeaconUseToken`, `BeaconRevertToken`)
- ❌ Group 5 — Helpers (`BeaconIsAdmin`, `BeaconGetCustomUserData`, `toWideChar`)
- ❌ Group 6 — KV store (`BeaconAddValue`, `BeaconGetValue`, `BeaconRemoveValue`)

That means the public-ecosystem BOFs that matter most (post-ex modules that
spawn child processes, impersonate tokens, or stash state) silently no-op or
fail. We also only support **one** module format (CS COFF) — we have no
answer for richer Go-native modules or operator-private formats.

## Goals

1. **100 % Beacon API coverage** for CS-compatible BOFs, validated against
   the public ecosystem (TrustedSec SA, Outflank, FortyNorth, CS-community).
2. **Pluggable module formats** behind one runtime façade — `bof.Run(spec)`
   shouldn't care whether the module is CS COFF, Go-native, or a custom
   `.gof`.
3. **Build-tag gating** so an implant only ships the loaders it needs
   (mission-tailored binary size).
4. **Reuse the existing `*wsyscall.Caller`** — all new Win32 / NTDLL calls
   inside the Beacon API + injectors flow through one caller, same as the
   rest of `maldev`.

## Non-goals

- Re-implementing Cobalt Strike's full Aggressor-side semantics
  (sleep / blocking models, transform hooks). The loader runs BOFs in-process
  and returns their output.
- Replacing `runtime/clr` (in-process .NET) — it stays separate.

## Architecture

```
                ┌──────────────────────────────────┐
                │  runtime/bof.Run(ctx, spec)      │   ← single entry point
                └────────────────┬─────────────────┘
                                 │
        ┌───────────────────────┼────────────────────────┐
        │                       │                        │
        ▼                       ▼                        ▼
  ┌───────────┐          ┌────────────┐           ┌─────────────┐
  │ COFF      │          │ goloader   │           │ .gof loader │
  │ loader    │          │ wrapper    │           │ (custom)    │
  │ (CS BOF)  │          │ (Go .o)    │           │             │
  └─────┬─────┘          └──────┬─────┘           └──────┬──────┘
        │                       │                        │
        └────────────┬──────────┴────────────┬───────────┘
                     │                       │
                     ▼                       ▼
            ┌─────────────────┐     ┌──────────────────┐
            │ Beacon API host │     │ maldev API host  │
            │ (CS-compatible) │     │ (rich, typed,    │
            │ via NewCallback │     │  Go-native)      │
            └────────┬────────┘     └──────────┬───────┘
                     │                         │
                     └───────────┬─────────────┘
                                 ▼
                       ┌──────────────────────┐
                       │ *wsyscall.Caller     │
                       │ (shared, opsec-aware)│
                       └──────────────────────┘
```

Key decisions:

- **`bof.Run` becomes format-agnostic.** Today it's `bof.Run` ⇒ COFF only.
  We introduce `Spec` ⇒ {kind, bytes, args, host} and a `Loader` interface.
- **Two host APIs:** the CS-compatible `BeaconXxx` surface
  (for foreign modules) **and** a richer `maldev.*` surface
  (for in-house `.gof` / Go-native modules). Same Caller underneath.
- **Build tags:** `bof_coff` (default), `bof_goloader`, `bof_gof`. An
  operator-built implant compiles only what the mission needs.

## Workstreams

The plan splits into 5 numbered slices; each one ships independently with
its own commit, VM E2E, and `simplify` pass.

### Slice 1 — Beacon API completion (~1 week)

Close the 6 groups. Reuse existing helpers wherever possible
([[promote_utility_helpers]] applies — `win/token`, `win/impersonate`,
`process/enum`).

| Symbol | Implementation strategy | Helper to reuse |
|---|---|---|
| `BeaconUseToken` | wrap `ImpersonateLoggedOnUser` (already in `win/impersonate`) via `caller`. | `win/impersonate` |
| `BeaconRevertToken` | wrap `RevertToSelf`. | `win/impersonate` |
| `BeaconIsAdmin` | `win/token.IsElevated`. | `win/token` |
| `BeaconGetCustomUserData` | per-invocation user-data pointer (passed via `Spec.UserData`). | new |
| `BeaconAddValue` / `GetValue` / `RemoveValue` | concurrent `map[string]unsafe.Pointer` keyed on KV name; mutex-guarded; values live for the BOF call lifetime. | new |
| `BeaconInjectProcess` | pluggable: default = `inject.MethodCreateThread` (already in `inject/`); operator can pin via `Spec.InjectMethod`. | `inject/` |
| `BeaconInjectTemporaryProcess` | spawn `GetSpawnTo` target suspended → `BeaconInjectProcess` → resume. | `inject/`, `process/enum` |
| `BeaconSpawnTemporaryProcess` | `CreateProcessW` suspended, default spawnto, return PID+handle. | new (thin) |
| `BeaconCleanupProcess` | `TerminateProcess` + `CloseHandle`. | new (thin) |

**Exit criteria:** every `BeaconXxx` in `goffloader`'s reference list has a
working implementation; full `realbof_windows_test` matrix passes for the
ten reference BOFs (CS-SA `whoami`, `netuser`, `ipconfig`, Outflank
`SafetyKatz-Trampoline`, …).

### Slice 2 — Loader-format plug-in (~3 days)

Refactor `bof.Run` into:

```go
type Loader interface {
    Kind() Kind             // KindCOFF, KindGoModule, KindGOF
    Load(b []byte, h Host) (Runnable, error)
}

type Spec struct {
    Bytes  []byte
    Args   []byte    // BeaconDataPack
    Host   Host      // chosen at call site
    Caller *wsyscall.Caller
    Method Kind      // auto-detected if zero
}

func Run(ctx context.Context, s Spec) (*Result, error)
```

`Host` is the interface every loader uses; the two implementations are
`BeaconHost` (CS-compatible) and `MaldevHost` (Go-native, typed).
Existing COFF loader becomes the first `Loader` and the default Kind.
Auto-detection looks at magic bytes (`L\x01` COFF / Go `.o` magic /
`.gof` magic).

**Exit criteria:** every current `bof_test` still passes, no public API
changed beyond addition.

### Slice 3 — goloader integration (~3-5 days)

Vendor or wrap [eh-steve/goloader] to load `.o` produced by `go tool compile`
against the **same Go version as the implant**. Provide:

- `bofgen` helper (or doc) that compiles a `.go` file into a module.
- `MaldevHost` exposing typed primitives: `Print(format, args ...any)`,
  `Inject(target, sc) error`, `Token(kind) Token`, etc. — no
  `BeaconDataPack` indirection.

Constraints documented up-front: implant + module Go versions must match
byte-for-byte; modules cannot import `runtime/cgo` or anything pulling
CGo.

**Exit criteria:** a sample `.go` "richmodule" performs an HTTP-based recon
(parses JSON, uses `crypto/aes`) and is loaded by the implant on
Windows VM.

### Slice 4 — `.gof` minimal custom format (~1 week)

Phase 2 (optional, lower priority). Header layout:

```
magic   "GOF1"          [4]
flags   uint32           — bit 0 = position-independent
entry   uint32           — RVA of entrypoint
nimp    uint32           — number of imports
imports [nimp]uint32     — FNV-1a hash of host symbol name
code    [...]            — flat blob, -fPIC compiled (clang or mingw)
```

Loader (≤300 LOC Go) maps the blob into RWX (or RX after fixup if `ACG`
preset is on), resolves imports against `MaldevHost` via FNV-1a, then
jumps to entrypoint. Module size target ≤5 KB.

**Exit criteria:** one `.gof` example that calls `Print` + does a thread-
hijack injection; size + entropy comparable to a CS BOF; survives
Defender real-time on a packed implant.

### Slice 5 — Build-tag gating + docs (~3 days)

- Add `//go:build bof_coff || bof_goloader || bof_gof` guards across the
  loader plug-ins.
- Default tag set in CI = `bof_coff` (matches today's behaviour).
- `examples/bof-multimod/` demonstrates building the implant with
  different tag combos.
- Update `runtime/bof/doc.go`, `docs/techniques/runtime/bof-loader.md`,
  `docs/tools/bof-runner.md` to cover the three formats + their MITRE
  IDs (`T1055`/`T1620`/etc.) + detection-level estimates.
- Glossary entries for `.gof`, `goloader`, `MaldevHost`.

**Exit criteria:** `go build -tags bof_coff,bof_goloader,bof_gof ./...` 
passes; per-tag builds also pass; doc page renders without broken links.

## Effort summary

| Slice | Effort | Risk |
|---|---|---|
| 1 — Beacon API completion | 1 week | Low — well-bounded, all helpers exist |
| 2 — Loader plug-in refactor | 3 days | Low |
| 3 — goloader integration | 3-5 days | Medium — Go-version coupling is the live trap |
| 4 — `.gof` custom format | 1 week | Medium — IOC + opsec validation needed |
| 5 — Build tags + docs | 3 days | Low |
| **Total** | **3-4 weeks elapsed** | |

## Out of scope (parked)

- **WASM modules (`wazero`)** — interesting opsec angle (non-native code
  surface), but ~Mo of runtime to embed and a host-API matrix to expose.
  Reconsider after slice 4 measures real implant size cost.
- **Open-source spin-off** of the Beacon API + COFF loader as
  `go-bofrunner`. After slice 1 we'd have a publishable subset; gate on
  user decision.

## Tracking

Bump this file's `status:` at each slice close. Slices map 1:1 to
commits (one or more PR per slice), referenced as
`feat(runtime/bof): slice N — <name>`.

Linked plans: [[backlog_plan_2026-04-25]],
[[sekurlsa_lsassdump_completion_plan]] (sister "complete the surface" plan
for credentials).

---

## Appendix A — Beacon API completion checklist

Every symbol that public BOFs call. Status against today's
`runtime/bof/beacon_api_windows.go`. Signature in `beacon.h` syntax;
"Resolves to" = the Go helper or strategy we plug in.

### A.1 Data parsing — Group 1 (5 symbols, ✅ complete)

| # | Symbol | Status | Resolves to |
|---|---|---|---|
| 1 | `void BeaconDataParse(datap *, char *buf, int size)` | ✅ | `bof/beacon_api.dataParse` |
| 2 | `int BeaconDataInt(datap *)` | ✅ | inline |
| 3 | `short BeaconDataShort(datap *)` | ✅ | inline |
| 4 | `int BeaconDataLength(datap *)` | ✅ | inline |
| 5 | `char *BeaconDataExtract(datap *, int *size)` | ✅ | inline |

### A.2 Format / output — Group 2 (9 symbols, ✅ complete)

| # | Symbol | Status |
|---|---|---|
| 6 | `void BeaconFormatAlloc(formatp *, int maxsz)` | ✅ |
| 7 | `void BeaconFormatReset(formatp *)` | ✅ |
| 8 | `void BeaconFormatFree(formatp *)` | ✅ |
| 9 | `void BeaconFormatAppend(formatp *, char *, int)` | ✅ |
| 10 | `void BeaconFormatPrintf(formatp *, char *fmt, ...)` | ✅ |
| 11 | `char *BeaconFormatToString(formatp *, int *size)` | ✅ |
| 12 | `void BeaconFormatInt(formatp *, int value)` | ✅ |
| 13 | `void BeaconPrintf(int type, char *fmt, ...)` | ✅ |
| 14 | `void BeaconOutput(int type, char *data, int len)` | ✅ |

Notes — `BeaconError*` (D / DD / NA) are also implemented; not part of
the canonical `beacon.h` but emitted by CS for runtime errors. Keep.

### A.3 Token impersonation — Group 3 (2 symbols, ❌ both missing)

| # | Symbol | Status | Resolves to |
|---|---|---|---|
| 15 | `BOOL BeaconUseToken(HANDLE token)` | ❌ | `win/impersonate.WithToken(caller, token)` — wraps `ImpersonateLoggedOnUser` |
| 16 | `void BeaconRevertToken(void)` | ❌ | `win/impersonate.Revert(caller)` — wraps `RevertToSelf` |

Implementation note — we must thread the impersonation through the
*calling OS thread* not a freshly-locked goroutine: BOFs call `BeaconUseToken`
expecting all subsequent Win32 calls in the same call frame to inherit
the token. Lock the thread for the BOF invocation duration in the loader
(`runtime.LockOSThread` at the entry trampoline, `Unlock` at return).

### A.4 Process injection — Group 4 (5 symbols, 4/5 ❌)

| # | Symbol | Status | Resolves to |
|---|---|---|---|
| 17 | `void BeaconInjectProcess(HANDLE hProc, int pid, char *payload, int p_len, int p_offset, char *arg, int a_len)` | ❌ | `inject.NewWindowsInjector(...).InjectInto(hProc, payload, arg)`; default method = `MethodCreateThread`, overridable via `Spec.InjectMethod`. |
| 18 | `void BeaconInjectTemporaryProcess(PROCESS_INFORMATION *pInfo, char *payload, int p_len, int p_offset, char *arg, int a_len)` | ❌ | `spawnSuspended(GetSpawnTo()) → BeaconInjectProcess → ResumeThread`. Returns the new process info to the BOF for cleanup. |
| 19 | `BOOL BeaconSpawnTemporaryProcess(BOOL bIgnoreToken, BOOL bAlloc, STARTUPINFO *si, PROCESS_INFORMATION *pInfo)` | ❌ | `CreateProcessW(GetSpawnTo(), suspended)`. `bIgnoreToken=FALSE` ⇒ honour current impersonation. |
| 20 | `void BeaconCleanupProcess(PROCESS_INFORMATION *pInfo)` | ❌ | `TerminateProcess + CloseHandle`. |
| 21 | `void BeaconGetSpawnTo(BOOL x86, char *buffer, int length)` | ✅ | static `C:\Windows\System32\rundll32.exe` today; **TODO slice 1.b**: make it configurable per-mission via `Spec.SpawnTo` (default kept). |

Implementation note — injection method needs to be a *per-Spec* knob,
not global. Different BOFs in a single session need different opsec
profiles (recon BOF = `MethodCreateThread`, post-ex elevation BOF =
`MethodEarlyBirdAPC`). Default = `CreateThread`; document the Spec
field clearly.

### A.5 Helpers — Group 5 (3 symbols, ❌ all missing)

| # | Symbol | Status | Resolves to |
|---|---|---|---|
| 22 | `BOOL BeaconIsAdmin(void)` | ❌ | `win/token.IsElevated(caller)`. |
| 23 | `void BeaconGetCustomUserData(char **buffer, int *length)` | ❌ | `Spec.UserData []byte` ⇒ returned via pointer pair. Default empty. |
| 24 | `int toWideChar(char *src, wchar_t *dst, int max)` | ❌ | UTF-8 → UTF-16LE; `golang.org/x/text/encoding/unicode` or a hand-rolled loop (we already have one in `encode/`). |

### A.6 Key-value store — Group 6 (3 symbols, ❌ all missing)

| # | Symbol | Status | Resolves to |
|---|---|---|---|
| 25 | `void BeaconAddValue(const char *key, void *ptr)` | ❌ | concurrent `sync.Map` scoped to the running BOF — values live for the call lifetime, freed at return. |
| 26 | `void *BeaconGetValue(const char *key)` | ❌ | same store; nil if absent. |
| 27 | `void BeaconRemoveValue(const char *key)` | ❌ | `Map.Delete`. |

Implementation note — operators frequently use this to stash an
impersonation token across multiple BOF invocations chained in one
"Beacon command". We **deliberately scope to a single Run**: cross-Run
state goes through the implant's own session state, not Beacon globals.
Doc this loud-and-clear in the loader's `doc.go`.

### A.7 Status digest

- **Today:** 15 of 27 (56 %).
- **After slice 1:** 27 of 27 (100 %), bar the parked `BeaconCallbacks`
  dispatcher (internal table, already wired).

---

## Appendix B — `.gof` (Go Object Format) v1 spec

The custom format for **maldev-private** modules. Designed for:

- Position-independent execution from RWX (or RX after `ACG` fixup).
- Symbol-hashed imports (no symbol strings in the blob).
- ≤5 KB for a non-trivial module (one screen of code + a syscall stub).
- Built from C with `clang -fPIC -nostdlib` or assembler — never from Go
  (Go object files use the Go-native loader path, slice 3).

### B.1 Wire layout

All integers little-endian. Sections concatenated, no alignment beyond
explicit padding fields.

```
offset  size   name             description
─────── ─────  ──────────────── ───────────────────────────────────────
0x00    4      magic            "GOF1" (0x31464F47)
0x04    4      version          0x00010000  (1.0)
0x08    4      flags            bit0 = PIC, bit1 = encrypted, bit2 = AES-CTR
0x0C    4      entry_rva        offset of entrypoint relative to code
0x10    4      code_size
0x14    4      data_size        BSS-style, allocated by the loader, zeroed
0x18    4      nimports
0x1C    4      imports_off      offset of import table from start of blob
0x20    4      code_off         offset of code section from start of blob
0x24    4      checksum         FNV-1a over (magic..code_off + code)
0x28    [...]  reserved         24 bytes, zero
0x40    [...]  imports          nimports × uint32 (FNV-1a of symbol name)
...     [...]  code             code_size bytes, PIC, x86_64
```

Header total = 64 bytes. Imports table = 4 × nimports. Code follows
immediately.

### B.2 Symbol hashing

```c
uint32_t fnv1a(const char *s) {
    uint32_t h = 2166136261U;
    while (*s) { h ^= (uint8_t)*s++; h *= 16777619U; }
    return h;
}
```

Same primitive as the existing `hash/` package — promote the function
if it isn't already exported (per [[feedback_promote_utilities]]).

Operator-side, `gofgen` emits an `imports.h` mapping `name → hash`:

```c
#define MALDEV_PRINT       0xA1B3C4D5
#define MALDEV_INJECT      0xDEADBEEF
#define MALDEV_TOKEN_USE   0xCAFEBABE
extern void (*g_imports[])(void);
#define mPrint   ((void (*)(const char*, ...))g_imports[0])
#define mInject  ((int  (*)(uint32_t, const void*, size_t))g_imports[1])
```

`g_imports[]` is set up by the loader before jumping to `entry_rva`.

### B.3 Host ABI

The hashed symbols resolve into the `MaldevHost` table. v1 surface
(intentionally minimal — extend as concrete needs arise):

| Hash key | Go signature exposed |
|---|---|
| `MALDEV_PRINT`        | `Print(format string, args ...any)` |
| `MALDEV_INJECT`       | `Inject(pid uint32, sc []byte) error` |
| `MALDEV_TOKEN_USE`    | `UseToken(t Token) error` |
| `MALDEV_TOKEN_REVERT` | `RevertToken() error` |
| `MALDEV_IS_ADMIN`     | `IsAdmin() bool` |
| `MALDEV_OUTPUT`       | `Output(kind OutputKind, data []byte)` |
| `MALDEV_CALLER`       | `Caller() *wsyscall.Caller` — module pins its own opsec |
| `MALDEV_SLEEP`        | `Sleep(d time.Duration)` — routed through `evasion/sleepmask` if enabled |

Each entry is exposed as a C function pointer via
`windows.NewCallback` (the same trick `goffloader` uses). Type
adaptation (Go strings ↔ C `char *`) happens in a one-time wrapper
generated at loader init.

### B.4 Encryption (flag bit 1)

When `flags & 0x02` set: the `code` blob is AES-128-CTR encrypted; key
+ nonce live in the `reserved` field of the header at offsets 0x28
(key, 16B) and 0x38 (nonce, 16B with last 8B counter). Loader
decrypts in-place into freshly-allocated RWX, then optionally
`VirtualProtect`s to RX before jumping.

> [!NOTE]
> The encryption key in the header is a per-module **wrap key** —
> useless on its own. The implant unwraps it via a session-derived KDF
> (matches the existing `crypto/` AES-CTR pattern). Spec the unwrap
> in slice 4.b.

### B.5 Building a `.gof` module

Reference toolchain:

```bash
# 1. Compile the module — PIC, no libc, freestanding
clang -target x86_64-pc-windows-msvc -fPIC -nostdlib -O2 \
      -ffunction-sections -fdata-sections \
      -Wl,-entry:bof_entry -c module.c -o module.o

# 2. Link to flat code
lld-link /entry:bof_entry /subsystem:native /noentry /merge:.rdata=.text \
         /out:module.bin module.o

# 3. Convert into .gof — header + import table from imports.json
gofgen pack -code module.bin -imports module.imports.json -out module.gof
```

`gofgen` (new `cmd/gofgen`) is a build-host tool — like `cert-snapshot`,
not part of the operator loadout. It computes FNV-1a hashes, fills the
header, optionally encrypts.

### B.6 Loader sketch (Go-side, ≤300 LOC target)

```go
func LoadGOF(blob []byte, host MaldevHost) (Runnable, error) {
    h, err := parseHeader(blob)              // 64-byte header
    if err != nil { return nil, err }
    imports := blob[h.ImportsOff : h.ImportsOff+4*h.NImports]
    code    := blob[h.CodeOff   : h.CodeOff+h.CodeSize]

    // Allocate RWX, copy (or decrypt) the code section
    page, err := windows.VirtualAlloc(0, uintptr(h.CodeSize+h.DataSize),
        windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_EXECUTE_READWRITE)
    if err != nil { return nil, err }
    if h.Flags&FlagEncrypted != 0 { decryptCTR(page, code, h) } else { copyTo(page, code) }

    // Resolve imports into a g_imports[] array
    table := make([]uintptr, h.NImports)
    for i := uint32(0); i < h.NImports; i++ {
        hash := binary.LittleEndian.Uint32(imports[i*4 : i*4+4])
        cb, ok := host.Resolve(hash)
        if !ok { return nil, fmt.Errorf("gof: unresolved import 0x%08x", hash) }
        table[i] = cb
    }

    // Patch the module's g_imports pointer in .data (well-known offset 0)
    *(*uintptr)(unsafe.Pointer(page + h.CodeSize)) = uintptr(unsafe.Pointer(&table[0]))

    // Jump to entry
    return &gofRunnable{entry: page + uintptr(h.EntryRVA), page: page, size: h.CodeSize+h.DataSize}, nil
}
```

### B.7 Example module (a "hello" `.gof`)

```c
#include "imports.h"

void bof_entry(void) {
    if (mIsAdmin()) {
        mPrint("running elevated\n");
    } else {
        mPrint("running as user\n");
    }
}
```

After build → typically ~600 bytes total (header 64 + imports 8 + code
~530). Operator-side check: `packerscope -inspect module.gof` should
parse it without complaint (`packerscope` gets a `.gof` mode in slice 4).

### B.8 What `.gof` deliberately does **not** do

- **No relocation table.** Modules must be position-independent
  (`-fPIC`). Loader does zero fix-ups beyond import patching.
- **No TLS, no exception handlers.** Modules call `MALDEV_OUTPUT` on
  error paths.
- **No symbol strings.** All resolution by hash; reverse-engineering a
  blob can't recover the host API names without our `imports.json` map.

### B.9 Versioning rule

`magic = "GOF1"` only ever describes the v1 layout. Any breaking change
ships as `GOF2` with a fresh loader behind a separate `bof_gof2` tag.
Loaders are not back-compatible — operators rebuild modules.


