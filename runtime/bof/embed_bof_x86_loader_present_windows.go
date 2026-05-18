//go:build windows && bof_x86_loader

package bof

import _ "embed"

//go:embed internal/x86loader/bof_x86_loader.x86.dll
var x86LoaderDLL []byte

// loadX86LoaderDLL returns the embedded WoW64 BOF loader DLL bytes.
// Linked into the binary by the `bof_x86_loader` build tag. The
// orchestrator (x86fork_windows.go) writes these bytes to a temp
// file before spawning rundll32 (SysWOW64) against them.
//
// The artefact is produced by scripts/build-bof-x86-loader.sh; the
// committed .dll is the source of truth (same pattern as
// runtime/pe/internal/noconsolation/NoConsolation.x64.o).
func loadX86LoaderDLL() ([]byte, error) {
	return x86LoaderDLL, nil
}
