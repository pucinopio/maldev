// Package pe runs full Portable Executable binaries (EXE / DLL)
// in-process by dispatching them through an embedded Fortra
// No-Consolation BOF on top of [runtime/bof].
//
// The package is the runtime-execution counterpart to the
// file-format utilities under [github.com/oioio-space/maldev/pe]:
// where pe/srdi turns a PE into shellcode for cross-process
// injection, runtime/pe loads the PE bytes and runs them in the
// implant's own address space — capturing stdout, returning the
// PE's printed output as a Go string. The wrapper hides the
// 27-field BeaconData marshaling that No-Consolation expects on
// its entry point.
//
// # Build tag
//
// The No-Consolation object file is gated behind the
// `pe_noconsolation` build tag — same discipline as the BYOVD
// drivers under kernel/driver/rtcore64. The default build returns
// ErrLoaderMissing; build the implant with
//
//	go build -tags=pe_noconsolation ./...
//
// after producing the .o via tools/no-consolation-build.sh (which
// compiles fortra/No-Consolation @ pinned commit with
// x86_64-w64-mingw32-gcc and drops the artefact into
// runtime/pe/internal/noconsolation/).
//
// # MITRE ATT&CK
//
//   - T1620 (Reflective Code Loading) — PE loader executes from
//     a manually-mapped region, not the on-disk image
//   - T1059 (Command and Scripting Interpreter) — operator-supplied
//     EXEs and DLLs run inline in the calling process
//
// # Detection level
//
// moderate
//
// Inherits the parent BOF loader's RWX-watcher exposure plus the
// PE-loader-specific telemetry: PEB.Ldr chain mutation when
// LinkToPEB is set, KernelBase reflection for IAT fixup, and
// per-DLL LdrLoadDll calls when LoadAllDeps is true. AMSI and
// behavioural EDRs that hook NtMapViewOfSection observe the
// section unmap-and-remap pattern.
//
// # Required privileges
//
// unprivileged. The PE itself decides what it needs; the loader
// only consumes the implant's existing token.
//
// # Platform
//
// Windows-only. amd64 by default; x86 module path reserved for
// 32-bit implants once the .x86.o is vendored.
//
// # See also
//
//   - docs/techniques/runtime/pe-loader.md
//   - [github.com/oioio-space/maldev/runtime/bof] — underlying COFF loader
//   - [github.com/oioio-space/maldev/runtime/clr] — sibling reflective runtime (.NET)
//   - [github.com/oioio-space/maldev/pe/srdi] — file-format counterpart (PE → shellcode)
//
// [github.com/oioio-space/maldev/runtime/bof]: https://pkg.go.dev/github.com/oioio-space/maldev/runtime/bof
// [github.com/oioio-space/maldev/runtime/clr]: https://pkg.go.dev/github.com/oioio-space/maldev/runtime/clr
// [github.com/oioio-space/maldev/pe/srdi]: https://pkg.go.dev/github.com/oioio-space/maldev/pe/srdi
// [github.com/oioio-space/maldev/pe]: https://pkg.go.dev/github.com/oioio-space/maldev/pe
package pe
