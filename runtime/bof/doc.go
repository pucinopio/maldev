// Package bof loads and executes Beacon Object Files (BOFs) —
// compiled COFF object files (`.o`) — entirely in process memory.
//
// A BOF is a relocatable COFF object that runs in the calling
// process's address space. The loader parses the COFF header,
// lays out every section with raw data into a single
// VirtualAlloc'd region (initially `PAGE_READWRITE`), applies
// relocations (`IMAGE_REL_AMD64_ABSOLUTE` / `_ADDR64` / `_ADDR32` /
// `_ADDR32NB` / `_REL32` / `_REL32_1` … `_REL32_5`), flips the
// executable sections to `PAGE_EXECUTE_READ` via `VirtualProtect`,
// registers `.pdata` with `RtlAddFunctionTable` so SEH unwinds
// work, and finally calls the entry-point symbol resolved from
// the COFF symbol table. External symbols resolve via the
// Beacon API stub table (`__imp_BeaconXxx`) or by dynamic-link
// import name (`__imp_<DLL>$<Func>` → PEB walk + ROR13 export
// match). The same format used by Cobalt Strike's
// inline-execute and Sliver's BOF runner.
//
// # MITRE ATT&CK
//
//   - T1059 (Command and Scripting Interpreter) — in-memory code execution
//   - T1620 (Reflective Code Loading) — COFF loader is a textbook reflective primitive
//
// # Detection level
//
// moderate
//
// The loader allocates `PAGE_READWRITE` then flips to
// `PAGE_EXECUTE_READ` after relocations — no RWX is ever
// exposed. Behavioural EDRs still flag the `VirtualAlloc`
// → `VirtualProtect(EXECUTE)` cadence and execution from
// non-image memory; the payload never touches disk and runs
// inside the caller's process so there is no fresh-process
// telemetry.
//
// # Required privileges
//
// unprivileged. Loader runs entirely in the calling process's
// own address space — `VirtualAlloc(RW)` for the COFF text +
// Beacon-API stub table, then `VirtualProtect` to flip exec
// sections to RX. No cross-process work for an in-process x64
// BOF. The privilege requirements of the loaded BOF itself
// depend on what it does (a `whoami` BOF needs no extra; an
// LSA secrets BOF needs admin). Sub-features add their own
// privilege contracts: `SetExecuteAsToken` needs
// `SeImpersonatePrivilege` (admin or service context);
// `SetCaller` with a direct/indirect syscall method needs no
// extra privileges, but routes around userland hooks an
// operator might rely on.
//
// # Platform
//
// Windows-only. The default build covers x64 BOFs in-process.
// Building with `-tags=bof_x86_loader` adds a cross-process
// x86 path: an embedded 11 KB i386 DLL is reflectively loaded
// into a freshly-spawned `SysWOW64\rundll32.exe` helper and
// drives the 32-bit `.o` from there. ARM64 is not supported —
// would need a parallel relocation table + an ARM64-side
// Beacon API host.
//
// # Example
//
// See [ExampleLoad] in bof_example_test.go.
//
// # See also
//
//   - docs/techniques/runtime/bof-loader.md
//   - [github.com/oioio-space/maldev/runtime/clr] — sibling reflective runtime (.NET)
//   - [github.com/oioio-space/maldev/inject] — alternative for cross-process delivery
//
// [github.com/oioio-space/maldev/runtime/clr]: https://pkg.go.dev/github.com/oioio-space/maldev/runtime/clr
// [github.com/oioio-space/maldev/inject]: https://pkg.go.dev/github.com/oioio-space/maldev/inject
package bof
