---
status: open
opened: 2026-05-17
scope: runtime/bof
counterpart: praetorian-inc/goffloader @ main (cloned 2026-05-17)
references:
  - https://github.com/praetorian-inc/goffloader
  - /tmp/goffloader (local shallow clone)
---

# maldev `runtime/bof` vs goffloader — gap analysis

Audit motivated by the explicit ask "**je veux un loader de BOF pur Go
comme goffloader, mais complet**". This doc walks the two packages
side-by-side, calls out where each leads, and feeds a concrete
improvement plan at the bottom.

## Codebase footprint

| Metric | maldev `runtime/bof` | goffloader |
|---|---|---|
| Total LOC (windows) | ~2 700 (impl + tests) | ~810 (impl) |
| Test files | 7, ~1 500 LOC | 0 first-party — examples only |
| Implementation files | 6 | 4 (`coff`, `lighthouse`, `pe`, `memory`) |
| Third-party deps | `golang.org/x/sys/windows`, `stretchr/testify` | `RIscRIpt/pecoff`, `x/sys/windows` |
| Coff parser | hand-rolled (`bof_windows.go`) | external `RIscRIpt/pecoff` |
| Public entrypoint | `Load` + `(*BOF).Execute` + `Run(ctx, Spec)` | `coff.Load([]byte, []byte) (string, error)` |

## Beacon API surface — symbol-by-symbol

Legend: ✅ functional · ⚠️ stub-only / no-op · ❌ unresolved (load-time error)

| Symbol | maldev | goffloader | Notes |
|---|---|---|---|
| `BeaconDataParse` | ✅ | ✅ | both correct; goffloader's strips a 4-byte length prefix at parse, maldev relies on the caller-supplied `size` |
| `BeaconDataInt` | ✅ | ✅ | identical wire format |
| `BeaconDataShort` | ✅ | ✅ | identical |
| `BeaconDataLength` | ✅ | ✅ | identical |
| `BeaconDataExtract` | ✅ | ✅ | maldev returns in-place pointer; goffloader copies into a fresh slice (goffloader keeps it alive across calls — small leak vs in-place trade-off) |
| `BeaconOutput` | ✅ (sync buffer) | ✅ (async chan) | architecture difference — see "Output model" below |
| `BeaconPrintf` | ✅ varargs expanded (slice 1.b) | ✅ varargs expanded, **up to 10 args**, **wide-string %s heuristic** | goffloader leads on arg count + %s smartness |
| `BeaconFormatAlloc` / `Reset` / `Free` / `Append` / `Int` / `ToString` | ✅ | ⚠️ `fallthrough` to `default` ⇒ falls back to LoadLibrary path that **always fails** | maldev leads — goffloader effectively missing the whole format-buffer family |
| `BeaconFormatPrintf` | ✅ varargs expanded | ⚠️ same as above |  |
| `BeaconErrorD` / `ErrorDD` / `ErrorNA` | ✅ separate errors buffer | ❌ not in switch; falls to default | maldev leads |
| `BeaconUseToken` | ✅ ImpersonateLoggedOnUser w/ LockOSThread | ❌ fallthrough → default → unresolved | maldev leads |
| `BeaconRevertToken` | ✅ RevertToSelf | ❌ same | maldev leads |
| `BeaconIsAdmin` | ✅ GetCurrentProcessToken.IsElevated | ❌ same | maldev leads |
| `BeaconGetSpawnTo` | ✅ x86/x64 dispatch + legacy fallback | ❌ same | maldev leads |
| `BeaconSpawnTemporaryProcess` | ✅ CreateProcess suspended | ❌ same | maldev leads |
| `BeaconInjectProcess` | ✅ VirtualAllocEx + WPM + CRT (with `lpParameter` arg delivery — slice 1.b) | ❌ same | maldev leads |
| `BeaconInjectTemporaryProcess` | ✅ spawn+inject+resume | ❌ same | maldev leads |
| `BeaconCleanupProcess` | ✅ TerminateProcess + CloseHandle | ❌ same | maldev leads |
| `BeaconGetCustomUserData` | ✅ per-BOF blob via `SetUserData` | ❌ same | maldev leads |
| `BeaconAddValue` / `GetValue` / `RemoveValue` | ✅ per-Run mutex-guarded map | ✅ **process-wide** global map | architecture difference — see "KV scope" |
| `toWideChar` | ✅ UTF-8 → UTF-16LE | ❌ fallthrough → default → unresolved | maldev leads |
| `BeaconGetOutputData` | ❌ not implemented | ❌ same | both miss it — used by some PE wrappers |

**Net**: maldev implements **27 / 28** canonical symbols (only
`BeaconGetOutputData` missing). goffloader implements **8 / 28** with
working bodies, **~7** in a `fallthrough` block that effectively does
nothing.

## Where goffloader leads

These are concrete wins worth porting.

### G1. Source-string obfuscation against YARA

goffloader writes every Beacon symbol name as a `string([]rune{...})`
literal so "BeaconOutput" never appears as a contiguous ASCII string
in the compiled binary. Same trick for the package name
("lighthouse", not "beacon"). YARA rules looking for
`"BeaconPrintf"`, `"BeaconDataParse"` etc. against the implant
binary's `.rdata` find nothing. Trivial to port.

### G2. Async output channel

goffloader returns a `chan<- interface{}` from its callback factory;
the BOF's `BeaconOutput` / `BeaconPrintf` push to the channel as the
BOF runs. Consumers can read in real-time. maldev buffers everything
and only surfaces it on `Execute` return — fine for short BOFs, bad
for streaming long-running ones (network recon BOFs that print as
they iterate).

### G3. Vararg capture up to 10

goffloader's printf thunk signature is `func(int, uintptr × 11)` —
10 trailing variadic slots. maldev currently does 6. A few CS BOFs
in the public corpus (e.g. complex `BeaconPrintf` in
SharpHound-like enumeration outputs) hit 7+. Easy to bump.

### G4. Wide-string %s heuristic

goffloader's printf tries `ReadCStringFromPtr` first; if the result
is shorter than 5 chars without a NUL, retries as UTF-16. Many BOFs
that interact with Win32 wide APIs pass wchar_t* to %s — the C
contract is undefined but operators expect it to "just work".
maldev currently writes raw bytes (so `L"hello"` becomes
`"h\0e\0l\0l\0o\0"`).

### G5. Embedded PE loader

Separate `pe/` package wraps `No-Consolation` BOF to run full
unmanaged Windows PE executables in-process. The user-facing API is
just `pe.RunExecutable(bytes, []string{args...}) (string, error)`.
maldev has the `pe/srdi` shellcode wrapper but not a "drop in
hello.exe and get its stdout back" UX. Genuine feature gap.

### G6. MEM_TOP_DOWN allocation

goffloader passes `MEM_COMMIT|MEM_RESERVE|MEM_TOP_DOWN` to
`VirtualAlloc` — places the BOF in high-address space which (a) is
slightly harder for some signature scanners that key on low-RVA
heuristics, (b) reduces collision with the host's heap. maldev uses
default (low) addresses. Trivial flag flip.

### G7. PackArgs operator helpers

goffloader exposes `PackArgs([]string)` consuming type-prefixed
strings (`"i42"`, `"zhello"`, `"Zwide"`, `"bDEADBEEF"`, etc.) —
operator-friendly for one-shot CLI usage. maldev has the more
expressive `(*Args).AddInt`/`AddString`/etc. but no high-level CLI
shim. Useful for a `cmd/bof-runner` flag interface that today
forces operators to pre-pack args.

### G8. Panic recovery in entrypoint goroutine

goffloader's `invokeMethod` wraps the BOF call in
`defer func() { if r := recover(); ... }` and pushes the panic +
stack trace to the output channel. maldev's `syscallN` jumps
straight into native code; a Go-side panic during arg marshaling
would kill the host. Add a recover at the entry trampoline.

### G9. Comments-as-anti-signature seeding

goffloader sprinkles deliberate-but-useless `fmt.Sprintf("...")`
calls through `processRelocation` and `Load`. The comment is
explicit:

> NOTE: There are random fmt.Sprintfs sprinkled through the code -
> these are intentional and seem to break static Go malware
> signatures. LEAVE THEM IN PLACE.

Practical YARA-evasion folklore. maldev's code is clean and likely
re-signaturable as soon as anyone bothers. We can either adopt the
same noise OR achieve the same via `garble` literal-obfuscation at
build time. Build-time wins on cleanliness.

## Where maldev leads

These are areas where porting *from* maldev *to* goffloader would
be the move — for ourselves, just observe they're moats we should
keep.

### M1. Full Beacon API surface

19 symbol implementations goffloader doesn't have (see table). For
a BOF that uses `BeaconUseToken` to impersonate a captured handle,
or `BeaconInjectProcess` to fork-and-run, goffloader returns an
unresolved-symbol error at load. maldev runs it.

### M2. PEB-walk dynamic-link import resolution

goffloader calls `LoadLibrary` + `GetProcAddress` for every BOF
import (`__imp_KERNEL32$LoadLibraryA` etc.) — both calls leave an
API trail visible to ETW/AMSI. maldev uses ROR13-hashed lookup via
PEB walk (`api.ResolveByHash`) for the canonical dollar-form, with
a curated fallback for mingw-w64 bare form. Defender sees no
`LoadLibrary` / `GetProcAddress` activity for BOF imports.

### M3. Per-Execute state isolation

maldev resets the per-BOF KV store + output + errors buffers at the
start of each `Execute`. goffloader's `keyStore` is a package-level
global `map[string]uintptr` that persists across BOF runs and across
parallel callers without mutex protection. A BOF that
`BeaconAddValue`s a transient pointer can poison the next BOF's
`BeaconGetValue` — exploitable bug-class.

### M4. Thread-token correctness

maldev `LockOSThread`s the goroutine for the BOF call duration so
`BeaconUseToken`'s `ImpersonateLoggedOnUser` flows through to
subsequent Win32 calls. Defensive `RevertToSelf` on Execute exit.
goffloader has neither (its `BeaconUseToken` isn't implemented at
all — moot for now, but the impersonation discipline matters once
that's filled in).

### M5. Plug-in loader framework

`Kind` / `Loader` / `Run(ctx, Spec)` opens the door for future
module formats (Go-native modules, `.gof`, WASM) behind one entry
point with magic-byte auto-detection. goffloader is monolithic.

### M6. Pluggable spawn-to with x86/x64 dispatch + legacy fallback

`SetSpawnTo` + `SetSpawnToX86` mirrors the modern CS contract and
keeps compat with BOFs built against the no-arg form. goffloader
has no spawn-to surface at all.

### M7. Test discipline

maldev: 76 tests across unit + behavioural (real CreateRemoteThread
against notepad) tiers, all passing on the Win10 VM.
goffloader: no first-party tests; the only validation is the
`whois_bof.go` / `hello_pe.go` examples.

### M8. Error routing

maldev keeps a separate errors buffer for `BeaconErrorD/DD/NA`;
operators can route the two streams differently. goffloader merges
everything into one channel.

### M9. NX-aware section protection (post-load)

maldev currently maps everything `PAGE_EXECUTE_READWRITE`.
goffloader allocates `PAGE_READWRITE`, applies relocations, then
flips text sections to `PAGE_EXECUTE_READ` via `VirtualProtect`.
goffloader's posture is actually better here — see improvement plan
slice 1.c.4.

## Output model — sync buffer vs async channel

A material architecture difference worth calling out.

| Aspect | maldev (sync buffer) | goffloader (chan) |
|---|---|---|
| API surface | `Execute([]byte) ([]byte, error)` | `Load([]byte, []byte) (string, error)` (blocks until close) |
| Latency on long BOFs | output appears only on return | streams in real-time |
| Concurrency safety | trivial — output is owned by the run | needs careful close + drain semantics |
| Backpressure | none (memory-bound) | implicit via channel block |
| Cancellation | no — Execute runs to completion | possible via context-aware reader |

Plan slice 1.c proposes an **optional** async path additionally
(`(*BOF).ExecuteStream(ctx, args, out chan<- []byte) error`) that
preserves the simple sync API as the default. Don't break what works.

## KV scope — per-Run vs process-wide

maldev: per-BOF, mutex-guarded `sync.Map`, dropped between
`Execute` calls. goffloader: global `map[string]uintptr` with no
mutex, never garbage-collected.

The CS contract is ambiguous; real BOFs use `BeaconAddValue` for two
patterns:

1. **Stash within a single BOF run** — pass a token across nested
   helper functions inside the BOF. Per-Run scope handles this
   correctly.
2. **Cross-Beacon-task state** — operator chains multiple BOFs
   under one Beacon session and wants to share state. The CS-server
   side stitches this together at the Beacon level, not via the BOF
   API. Even goffloader can't honour this cleanly (process-wide
   doesn't equal session-wide).

Per-Run is the correct default. We'll document and stay there.

---

# Improvement plan — slice 1.c

> Slice 1.c is the "goffloader-parity-and-then-some" finisher.
> Estimated effort: 2-3 days. Tracked under
> `.dev/refactor-2026/bof-loader-revamp-plan.md` slice 1.c when this
> doc is committed.

## Adopt from goffloader (must-have)

| # | Item | Effort | Notes |
|---|---|---|---|
| 1.c.1 | String-obfuscate Beacon import names (`string([]rune{...})` for the registration map keys) | 0.5h | Trivial, no API change. Defeats naive YARA. |
| 1.c.2 | Bump vararg capture to 10 in `beaconPrintfImpl` / `beaconFormatPrintfImpl` | 0.5h | Match goffloader. Update tests. |
| 1.c.3 | Wide-string `%s` heuristic in `expandCFormat` | 1h | If C-string < 5 chars no NUL → retry as UTF-16. Add test fixture. |
| 1.c.4 | RW→RX section flip via `VirtualProtect` after relocation, instead of RWX everywhere | 2h | Loader change in `bof_windows.go`. Defeats RWX-watcher EDR rules. Adds an op_per_section but it's quick. |
| 1.c.5 | `MEM_TOP_DOWN` flag on `VirtualAlloc` | 5min | Reduces low-RVA heuristic hit. |
| 1.c.6 | Panic recovery wrapper around `syscallN(entryAddr, ...)` | 1h | `defer func() { recover() → b.errors }`. Keeps the host alive on a busted BOF. |

## Original improvements (not in goffloader either)

| # | Item | Effort | Notes |
|---|---|---|---|
| 1.c.7 | **Async output streaming API** — new `(*BOF).ExecuteStream(ctx, args, out chan<- []byte) error` keeping the sync `Execute` unchanged | 4h | Best of both worlds; pulls in `select` on ctx.Done() so callers can cancel. |
| 1.c.8 | `BeaconGetOutputData` — bonus symbol both packages miss | 1h | Used by `No-Consolation` PE wrapper. Trivial pointer return. |
| 1.c.9 | **PE loader** (separate `runtime/pe` package) | 1-2 days | Embeds `No-Consolation` BOF, exposes `RunExecutable(bytes, args)`. Calls back into `runtime/bof.Run`. Genuine new feature, fills the gap M5 highlights. |
| 1.c.10 | `cmd/bof-runner` operator CLI — type-prefixed args (`-arg i42 -arg zhello -arg Zwide`) | 2h | goffloader-style operator productivity. Keep `Args.Add*` underneath. |
| 1.c.11 | YARA-resistance via `garble` literal obfuscation in `make release` | 1h | Less invasive than goffloader's hand-sprinkled `fmt.Sprintf`s; achieves the same on a build-time switch. |

## Stretch (post-slice-1.c)

- `garble`-built release pipeline for the BOF runner CLI specifically.
- Native syscall path (Indirect Caller via `*wsyscall.Caller`) for
  the four kernel32 calls inside our injection stubs — opsec
  improvement that goffloader doesn't have.
- Cross-version Go-stdlib helpers checked at build time so a CS BOF
  that calls `__imp_strlen` against msvcrt resolves cleanly.

---

# Decision: slice 1.c locks the "goffloader-but-complete" claim

After slice 1.c lands, maldev's runtime/bof:

- Implements **28 / 28** documented Beacon symbols vs goffloader's ~8.
- Mirrors goffloader's three concrete wins (string obfuscation,
  async output option, wide-%s) and exceeds on test discipline,
  state isolation, and native API resolution.
- Adds **one** original feature beyond either: pluggable loader
  framework (slice 2), PE loader as a sibling package
  (slice 1.c.9).

The "comme goffloader mais complet" objective is satisfied at that
point. No new revamp slice required.
