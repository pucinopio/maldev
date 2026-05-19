---
status: design-pending
opened: 2026-05-19
owner: oioio-space
scope: runtime/bof
parent_bundle: v0.156.0 (Bundles A–H shipped)
---

# Bundle I — Route every BOF import through `*wsyscall.Caller`

## Why this is non-trivial (the realisation that paused implementation)

The initial sketch in Bundle H's commit message proposed a 15-byte
per-symbol shim that loads a function ID into `r10` and jumps to a
shared dispatcher; the dispatcher invokes `b.caller.Call(name,
args...)`. Clean asm, ~3-5 hours of work.

**That sketch overlooked a fundamental constraint:**
`*wsyscall.Caller.Call(name, args...)` only resolves `name` against
**ntdll** (`win/syscall/direct_windows.go:136-159`). For the
`MethodDirect` / `MethodIndirect` / `MethodIndirectAsm` paths, the
SSN resolver (HellsGate / HalosGate / Tartarus / HashGate) only
knows about Nt* SSNs — there is no Wow64-style mapping for kernel32
or advapi32 in this package.

A typical CS BOF imports kernel32 / advapi32 wrappers, NOT Nt*
directly:

```
__imp_KERNEL32$VirtualAlloc          // kernel32.dll → NtAllocateVirtualMemory
__imp_ADVAPI32$OpenProcessToken      // advapi32.dll → NtOpenProcessToken
__imp_KERNEL32$CloseHandle           // kernel32.dll → NtClose
__imp_KERNEL32$WriteProcessMemory    // kernel32.dll → NtWriteVirtualMemory
```

Routing only `NTDLL$Nt*` imports through Caller would intercept ~5%
of a realistic BOF's calls. The other 95% would go through the
kernel32 wrapper which **internally calls the Nt* function in
ntdll** — exactly where userland EDR hooks live. The operator
chose `MethodIndirect+HellsGate` precisely to bypass those hooks;
routing only the rare direct-Nt imports defeats the point.

## The honest decision tree

### Option A — limited routing (Nt* imports only)

Replace `__imp_NTDLL$Nt*` slots with `syscall.NewCallback` thunks
that dispatch through `b.caller`. Leave every other import direct.

- **Cost:** ~1 day. Reuses existing Caller. No new mapping.
- **Benefit:** Minimal. Only rare BOFs (the ones that explicitly
  import Nt*) gain anything. The kernel32-wrapper majority is
  unchanged.
- **Honest cover for what `SetCaller` already does?** Yes. Already
  documented (bof-loader.md, SetCaller godoc).

### Option B — wrapper → syscall translation (full routing)

For every kernel32/advapi32/etc. wrapper a BOF imports, maintain a
mapping `{wrapperName → ntName, argShuffle func([]uintptr) []uintptr}`.
The shim translates the wrapper's argument shape into the Nt*
shape, then dispatches.

- **Cost:** 3-5 days. Mapping table is per-function (signature
  knowledge required). For example `VirtualAlloc(addr, size, type,
  protect)` → `NtAllocateVirtualMemory(processHandle=-1, &addr,
  zeroBits=0, &size, type, protect)` — needs synthesised
  out-params on the stack.
- **Coverage:** Real. Every translated wrapper now respects the
  operator's syscall method.
- **Maintenance:** Each new wrapper a BOF references needs a table
  entry. Public corpus probably needs ~40 entries to cover 95% of
  CS-SA-BOF + Outflank + FortyNorth.

### Option C — keep `SetCaller` scope, document, move on

Accept that `SetCaller` covers the three `BeaconInjectProcess`
primitives (the only ones an operator cares about in the CS fork-
and-run model). Document the limitation prominently (already done
in v0.156.0). Operators who want more should pair the BOF with
`evasion/unhook` to clean ntdll itself — solves the problem at the
right layer.

- **Cost:** 0. Already shipped.
- **Coverage:** What CS Beacon itself does. Operationally enough.

## Open questions for design discussion

1. **Is option B's mapping table maintenance worth it?** With the
   x86 cross-process loader and goloader (slice 3) on the roadmap,
   adding a separate 40-entry-and-growing wrapper table is real
   maintenance load. Probably not.

2. **Does any real-world BOF in the public corpus import Nt*
   directly?** Spot check needed — if no, option A has zero
   real-world test surface. Quick `nm` scan of CS-SA + Outflank
   .o files would settle it.

3. **Is the `evasion/unhook` pairing enough?** If an operator
   pre-cleans ntdll before loading the BOF, kernel32 wrappers
   route through clean ntdll automatically — same effect as
   option B with zero new code. Worth verifying in a VM scenario.

4. **Are there OPSEC reasons to specifically route Nt* imports?**
   Maybe a BOF that *avoids* kernel32 wrappers (some Sektor7 /
   Outflank advanced BOFs) gains from option A even if rare.

## Recommended next step

**Empirical first**: pull the CS-SA + Outflank + FortyNorth corpus
(~80 BOFs total), `objdump -h --syms *.o | grep __imp_` and count
how many import Nt* directly vs kernel32 wrappers. The answer
decides between A and C and whether B is worth the engineering.

If the corpus is dominated by kernel32 wrappers (likely): close
the loop with option C, mark Bundle I "wontfix — operator's
ntdll-cleaning is the right layer". If a meaningful tail of BOFs
uses Nt* directly: implement option A as a pure win.

## Out of scope for this bundle

- Building or extending `*wsyscall.Caller` to resolve kernel32 /
  advapi32 names. That's a Caller-package feature, not a BOF-
  loader feature, and is its own discussion.
- Per-BOF callback allocation: `syscall.NewCallback` shares a
  process-global table (~40 B per unique callback), so worst-case
  even option A is cheap. Not a blocker.

## TL;DR for the next session

1. Audit the public BOF corpus (objdump symbols).
2. If `> 5 %` of imports are Nt* → ship option A (~1 day).
3. Else → close Bundle I as wontfix with a doc note pointing
   operators at `evasion/unhook` as the right layer for full
   coverage.
