---
status: audit complete — implementation pending operator review
opened: 2026-05-19
owner: oioio-space
scope: repo-wide helper inventory
parent_session: v0.156.0 ship
---

# Helper-function audit — repo-wide sweep

## Methodology

Surveyed every Go package under the module root (~125 packages) for
recurring developer-facing boilerplate. For each area I looked at:

  1. Public API surface (`grep ^func [A-Z]`)
  2. `*_example_test.go` files — the canonical "how a dev uses this"
  3. `examples/*/main.go` — operator workflows
  4. Internal call sites of the same kernel32/ntdll primitive
  5. Cross-package boilerplate patterns

For each candidate helper:

  - Estimated lines saved per call site
  - Number of real call sites today
  - Whether the helper would replace a CORRECT-but-verbose pattern,
    or paper over a design issue better fixed elsewhere

The 80/20 cutoff: **a helper is worth shipping only when its
absence forces 3+ lines of identical boilerplate at 3+ call sites.**

## Honest finding: the repo is mostly tight

Eight years of discipline have produced a surface where most "this
looks repetitive" patterns turn out to be either (a) already
encapsulated, (b) genuinely different per call site, or (c) one-off.

Examples of patterns that LOOK helper-worthy but aren't:

  - "Open a process handle by PID" — `windows.OpenProcess` is already
    a one-liner; access masks differ per call site.
  - "Allocate RW then VirtualProtect RX" — repeats but each site has
    different section layout / size logic.
  - "Build a *wsyscall.Caller + defer Close" — already a 2-liner.
  - "Pack BOF args + Execute" — already handled by RunFromBytes +
    ArgsFromStrings (Bundle H).

## Real candidates (3 found, all P3)

### C1 — `inject.CreateRemoteThreadWithCaller` ✅ **shipped commit `bcfcdf6`**

`AllocRemoteWithCaller` / `WriteRemoteWithCaller` are deferred —
the existing private `allocateAndWriteMemoryRemoteWithCaller`
(combined alloc+write+protect to RX) covers the shellcode-style
use cases inject already serves; exporting the three primitives
separately is a second-order refactor that should land only if a
real consumer needs the granularity. The `CreateRemoteThread`
piece, by contrast, was duplicated across 4 inline sites — that's
the win this commit captured.

### C1 — original scope (kept here for design context)


Flagged by the Bundle B reuse reviewer. The current state:

  - `inject/memory_helpers_windows.go:54` has the combined
    `allocateAndWriteMemoryRemoteWithCaller` (alloc + write + RX
    flip) — unexported.
  - `inject/config_windows.go` + `inject/pipeline_windows.go` +
    `runtime/bof/beacon_api_extra_windows.go` each contain a
    1-shot inline `api.ProcCreateRemoteThread.Call(...)` site.
    Reviewer counted 4 sites across the repo.

**Proposal:** Export from `inject/`:

```go
func AllocRemoteWithCaller(h windows.Handle, size uintptr, protect uint32, caller *wsyscall.Caller) (uintptr, error)
func WriteRemoteWithCaller(h windows.Handle, dst, src, size uintptr, caller *wsyscall.Caller) error
func CreateRemoteThreadWithCaller(h windows.Handle, entry, param uintptr, caller *wsyscall.Caller) (windows.Handle, error)
```

`runtime/bof/beacon_api_extra_windows.go`'s `beaconRemoteAlloc` /
`beaconRemoteWrite` / `beaconRemoteCreateThread` (the Bundle B
internal helpers) become 3-line wrappers calling into these.

  - **Lines saved:** ~30 across 4 sites
  - **Risk:** low (refactor of working code, full test coverage)
  - **Cost:** half a day including tests + tech md update

### C2 — `win/api.AllocExecutable(code []byte) (uintptr, free func(), error)`

Pattern observed in 3 distinct files:

  - `runtime/bof/sacrificial_windows.go:317` — VEH exit stub
  - `runtime/bof/sacrificial_windows.go:262` — shared trampoline
  - `cleanup/memory/wipe_windows.go` (similar shape)

All do `VirtualAlloc(0, len, RW) → copy(dst, code) → VirtualProtect(RX)`
in 4-6 lines.

**Proposal:**

```go
// AllocExecutable maps a fresh page, copies code, flips to PAGE_EXECUTE_READ.
// Returns the base address (executable) and a free func that VirtualFrees
// on call. Caller is responsible for the free.
func AllocExecutable(code []byte) (uintptr, func(), error)
```

  - **Lines saved:** ~15 across 3-4 sites
  - **Risk:** medium (callers want stable addresses across goroutines
    — the helper must not move state; verify writeRXStub semantics
    transfer cleanly)
  - **Cost:** ~3 hours including a test matrix on different code
    sizes + Caller variants

### C3 — `stealthopen` dogfooding across the repo

Not a NEW helper — a sweep to thread an optional `stealthopen.Opener`
through file-reading helpers that currently call `os.ReadFile`
directly. Audit shows **~65 sites** in non-test, non-example code:

  - `credentials/lsassdump` — already has it
  - `pe/parse` — already has it
  - `recon/dllhijack` — already has it
  - Many `cmd/*` tools — application-level, fine as `os.ReadFile`
  - **`runtime/bof/runtime/pe/...`** — multi-file BOF loads may
    benefit, low priority

Realistic surface for the sweep is small (<10 sites) because most
file reads either already use stealthopen or are operator-tooling
binaries where the path-based hook is fine.

  - **Lines changed:** ~30 across 5-10 sites
  - **Risk:** low (parameter addition with nil-default; no behaviour
    change on existing call sites)
  - **Cost:** half a day including tech md + tests

## Where I looked and found nothing worth proposing

  - `crypto/` — already minimal, one-shots are 5 lines max.
  - `encode/` — base64/utf16/rot13 are already one-line wrappers.
  - `hash/` — same.
  - `random/` — one-shot helpers already present.
  - `useragent/` — one function, nothing to compose.
  - `pe/cert`, `pe/masquerade`, `pe/morph`, `pe/strip` — each has a
    single canonical entry point (`Forge`, `Apply`, `Run`, `Sanitize`).
  - `c2/transport`, `c2/shell` — already use builder pattern.
  - `evasion/preset` — already IS the composition helper.
  - `cleanup/*` — each technique is one function.
  - `persistence/*` — each has a single `Install` / `Remove` pair.
  - `process/enum`, `process/session` — single-call APIs.
  - `win/api`, `win/syscall`, `win/token`, `win/impersonate`,
    `win/privilege`, `win/com`, `win/domain`, `win/version` — all
    canonical Win32 mappings, no composition surface to compress.
  - `recon/*` — each technique surfaces a `Detect()` or `List()`
    one-shot.

## Recommendation

  - **C1** is the highest-value, lowest-risk improvement. The work
    is mostly mechanical (extract → adjust call sites → tests).
    Ship as a dedicated commit.
  - **C2** is worth doing if Bundle E's SEH-unwind work expands or
    we add more JIT-style allocations. Today's 3 sites is borderline
    — defer until a fourth site appears, then bundle them all.
  - **C3** is plumbing, not a new helper. Worth a separate sweep
    commit when an operator runs into a hook on one of the
    currently-unwrapped paths.

The methodical answer: **the repo doesn't need ~10 helper
additions**. It needs the discipline that already keeps its surface
minimal. Adding more helpers per session would turn into
maintenance cost without proportional clarity gain.

If you specifically want one shipped this session, **C1** is the
call — gives a real reuse win, no architecture risk.
