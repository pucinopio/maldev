//go:build windows && !bof_x86_loader

package bof

// loadX86LoaderShellcode is the default-build variant: no shellcode
// bytes are linked into the binary. The runtime/bof orchestrator
// surfaces ErrCrossArchX86Unsupported instead of attempting the
// cross-process injection. Build with `-tags=bof_x86_loader` to
// activate the embedded variant defined in the present-tag file.
//
// Why a build tag: the shellcode adds ~320 bytes to every implant
// whether or not the operator needs x86 BOF support. The tag-gated
// default keeps the cost opt-in, matching the pe_noconsolation
// pattern in runtime/pe.
func loadX86LoaderShellcode() ([]byte, error) {
	return nil, ErrCrossArchX86Unsupported
}
