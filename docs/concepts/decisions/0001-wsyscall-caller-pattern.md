# ADR-0001 — The `wsyscall.Caller` pattern

**Status:** accepted
**Date:** 2026-04-02 (origin in `evasion-interfaces` design)

## Context

Every NT* call inside maldev (`NtAllocateVirtualMemory`,
`NtProtectVirtualMemory`, `NtCreateThreadEx`, …) can be issued
via four different methods:

1. WinAPI through `kernel32.dll` (loudest, easiest).
2. NativeAPI through `ntdll.dll` (cleaner but still hooked).
3. **Direct** syscalls (`syscall` instruction with SSN, classic
   Hells/Tartarus/Hallows gate).
4. **Indirect** syscalls (resolve SSN, then `jmp` into a clean
   `ntdll.dll` syscall stub — bypasses hook-on-`syscall`).

Each EDR catches a different subset depending on hook strategy.
Operators want to switch methods per engagement.

If every package hard-codes one method, switching means rebuilding
or forking each package.

## Decision

We will expose every NT* call as a function that **accepts an
optional `*wsyscall.Caller`**.

```go
func PatchScanBuffer(caller *wsyscall.Caller) error
func StealByName(name string, caller *wsyscall.Caller) (windows.Token, error)
```

A `*wsyscall.Caller` is constructed once at process start with the
chosen method and threaded through every package:

```go
caller, _ := wsyscall.New(wsyscall.MethodIndirect)
amsi.PatchAll(caller)
inject.SectionMapInject(targetPID, sc, caller, nil)
```

When `caller == nil` the function falls back to WinAPI — useful
only for debug.

## Consequences

- **Positive.** One construction site, repo-wide behaviour change.
  Operators flip `MethodDirect` ↔ `MethodIndirect` based on
  defender posture without touching technique code.
- **Positive.** Composable: `evasion.ApplyAll([]Technique{…}, caller)`
  threads the same caller into every step of a chain.
- **Positive.** Testable: tests pass a `MethodWinAPI` caller for
  fast local validation.
- **Negative.** Every NT-using signature has the extra parameter.
  Verbose at call sites.
- **Negative.** The `nil` fallback is a footgun — easy to ship to
  production without realising the WinAPI path was taken.

## Alternatives considered

- **Build-tag per method.** Different `_direct.go` / `_indirect.go`
  files compiled in. Rejected: locks the choice at compile time,
  not at runtime.
- **Global caller singleton.** `wsyscall.SetDefault(method)`.
  Rejected: hides the dependency, defeats `evasion.ApplyAll`'s
  composability, no per-task switching.
- **Interface-per-package.** `amsi.AMSIClient` etc. Rejected:
  every package would redefine the same NT* signatures.

## References

- `.dev/superpowers/specs/2026-04-02-evasion-interfaces-design.md`
- `docs/architecture.md` § Caller Pattern
