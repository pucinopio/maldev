//go:build windows && bof_x86_loader

package bof

import _ "embed"

//go:embed internal/x86loader/bof_x86_loader.x86.bin
var x86LoaderShellcode []byte

// loadX86LoaderShellcode returns the embedded x86 BOF loader as a
// flat PIC shellcode blob. Linked into the binary by the
// `bof_x86_loader` build tag. The orchestrator
// (x86fork_windows.go) writes these bytes into a VirtualAllocEx
// region in a freshly-spawned WoW64 host, then CreateRemoteThread
// targets offset 0 of the region — no LoadLibrary, no disk drop.
//
// The artefact is produced by scripts/build-bof-x86-loader.sh; the
// committed .bin is the source of truth so `go build` works
// without any C toolchain (same pattern as
// runtime/pe/internal/noconsolation/NoConsolation.x64.o).
func loadX86LoaderShellcode() ([]byte, error) {
	return x86LoaderShellcode, nil
}
