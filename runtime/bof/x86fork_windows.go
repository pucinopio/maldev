//go:build windows

package bof

import "errors"

// ErrCrossArchX86Unsupported is returned when Run or Load is fed a
// 32-bit (i386) COFF object and the implant has no embedded x86 loader
// DLL to route it through.
//
// Phase A (this commit) ships only the detection layer + this clean
// error. Phase B will add the fork-and-run orchestrator: spawn the
// SpawnToX86 host (default `C:\Windows\SysWOW64\rundll32.exe`)
// suspended, write the BOF + an x86 loader DLL into it via
// VirtualAllocEx + WriteProcessMemory, launch with CreateRemoteThread,
// then ReadProcessMemory the captured output. Phase C builds the
// loader DLL itself (mingw32 i686-w64-mingw32-gcc) and ships it gated
// behind the bof_x86_loader build tag — same pattern as
// pe_noconsolation in runtime/pe.
//
// Until phase B+C land, callers that need x86 BOF execution should
// either: (a) build a separate x86 implant that runs the in-process
// loader natively (blocked today on win/syscall/386 missing .s
// files), or (b) wait for the fork-and-run path.
var ErrCrossArchX86Unsupported = errors.New("runtime/bof: x86 COFF detected but no x86 loader embedded (slice 1.d.2 phase B/C — fork-and-run not yet shipped)")

// coffX86Loader is the registry stub for KindCOFFx86. It always
// errors with ErrCrossArchX86Unsupported in this build because the
// orchestrator + embedded x86 loader DLL haven't shipped yet. The
// registration is still useful: it gives Run a clean dispatch path
// that surfaces a specific, actionable error instead of the generic
// "no loader registered for kind coff-x86".
type coffX86Loader struct{}

func (coffX86Loader) Kind() Kind { return KindCOFFx86 }

func (coffX86Loader) Load(_ []byte) (Runnable, error) {
	return nil, ErrCrossArchX86Unsupported
}

func init() {
	registerLoader(coffX86Loader{})
}
