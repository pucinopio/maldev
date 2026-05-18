//go:build windows && !bof_x86_loader

package bof

// loadX86LoaderDLL is the default-build variant: no DLL bytes are
// linked into the binary. The runtime/bof orchestrator surfaces
// ErrCrossArchX86Unsupported instead of attempting to spawn the
// fork-and-run helper. Build with `-tags=bof_x86_loader` to
// activate the embedded variant defined in
// embed_bof_x86_loader_present_windows.go.
//
// Why a build tag: the loader DLL is a 5 KB i386 mingw32 build
// artefact under runtime/bof/internal/x86loader/. Even though it
// ships committed to the repo, embedding it unconditionally would
// add ~5 KB to every implant whether or not the operator needs
// x86 BOF support. The tag-gated default keeps the cost opt-in,
// matching the pe_noconsolation pattern in runtime/pe.
func loadX86LoaderDLL() ([]byte, error) {
	return nil, ErrCrossArchX86Unsupported
}
