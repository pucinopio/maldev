//go:build windows

package bof

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/oioio-space/maldev/win/api"
	"golang.org/x/sys/windows"
)

// Sacrificial-thread crash isolation.
//
// # Threat model
//
// In the default (inline) Execute mode, the BOF runs on the same
// OS thread as the implant. Any native exception the BOF raises
// — access violation on a wild pointer, stack overflow, illegal
// instruction from a busted relocation — propagates through
// Windows SEH, hits Go's runtime exception handler, and ends in
// TerminateProcess. The implant dies with the BOF.
//
// Sacrificial mode mitigates that by:
//
//  1. Spawning the BOF entry on a dedicated OS thread via
//     CreateThread (start address = a per-call thunk that
//     unpacks argPtr+argLen and tail-calls b.entryAddr).
//  2. Installing a process-wide Vectored Exception Handler at
//     priority 1 (first to run). When the VEH sees a fault whose
//     ExceptionAddress falls inside *any* registered BOF's
//     [execMem, execMem+execMemSize) mapping, it rewrites the
//     faulting CONTEXT.Rip to point at a small exit-stub that
//     calls ExitThread(1). Control returns to the kernel which
//     resumes the thread at the stub — the thread dies cleanly
//     without invoking the default UnhandledExceptionFilter
//     (which would TerminateProcess).
//  3. The host thread waits on the BOF thread handle with the
//     operator-supplied timeout. On timeout, TerminateThread
//     (leaky but last-resort). On exit, the captured fault
//     metadata is surfaced as a Go error.
//
// # Limitations (operator must accept)
//
//   - Token impersonation done by the BOF (BeaconUseToken)
//     affects only the sacrificial thread. The host thread that
//     called Execute keeps its original token; impersonation
//     state does NOT cross back to the calling goroutine. BOFs
//     that chain token changes across calls are not compatible
//     with sacrificial mode.
//
//   - The VEH only catches faults whose ExceptionAddress lies
//     inside the BOF's own mapping. A BOF that passes a NULL
//     pointer to kernel32!HeapAlloc takes the fault inside
//     kernel32, NOT inside the BOF — those still kill the
//     implant under sacrificial mode. Mitigation: defensive
//     BOFs that null-check before native calls.
//
//   - TerminateThread (used on timeout) leaks the thread's stack
//     + any kernel objects it held — Windows-design limitation,
//     unavoidable. Set timeouts generously; this is a last-
//     resort kill, not a routine cancellation primitive.
//
//   - On timeout the per-call args struct (24 B, Go heap) stays
//     referenced via sacrificialMap and is never released.
//     Windows offers no synchronous way to confirm a forcibly-
//     terminated thread has actually stopped executing in our
//     code; releasing the struct could race with a final read
//     by the terminated thread. The leaked structs are tiny and
//     bounded by the number of timeouts the operator triggers —
//     in practice negligible. Close() does NOT reclaim them
//     either (same reason).
//
//   - The VEH is registered once per process via sync.Once and
//     never removed. Tiny perma-handler in the implant's
//     exception chain; cost is one sync.Map lookup per process-
//     wide exception. Documented opsec posture; not removable.

var (
	vehInstallOnce sync.Once
	vehInstallErr  error
	exitStubAddr   uintptr // RX stub: rsp-align + mov rcx,1 + call ExitThread

	// Shared per-process entry trampoline. One 4 KB RX page,
	// initialised lazily on first sacrificial Execute. Replaces
	// the legacy per-call thunk: a *sacArgs struct address is
	// passed to CreateThread via lpParameter (lands in rcx on
	// thread start) and the stub demultiplexes it into rcx/rdx
	// before tail-calling the BOF entry. Owns its own sync.Once
	// so a future eviction strategy (or a unit test) can rebuild
	// it without dragging the VEH install along.
	trampolineInstallOnce sync.Once
	trampolineInstallErr  error
	sharedTrampolineAddr  uintptr

	sacrificialMap sync.Map // map[uint32 tid]*sacFrame — registered for VEH lookup
)

// sacFrame is the per-Execute capsule passed to the shared
// trampoline via CreateThread's lpParameter. The trampoline
// reads three uintptrs at offsets 0/8/16; the bof field at
// offset 24 is invisible to asm — used only by the VEH callback
// (for mapping range checks) and Close (for liveness).
//
// Field order MUST stay in lock-step with the asm in
// installSharedTrampoline; argPtr/argLen/entry MUST stay at
// offsets 0/8/16 respectively.
type sacFrame struct {
	argPtr uintptr
	argLen uintptr
	entry  uintptr
	bof    *BOF
}

// faultRecord captures the exception code + address the VEH saw.
// Stored on *BOF by the VEH; read by the host thread after the
// BOF thread exits. atomic.Store + WaitForSingleObject's
// implicit acquire fence on the host side keep reads coherent.
type faultRecord struct {
	code uint32 // EXCEPTION_ACCESS_VIOLATION = 0xC0000005 etc.
	pc   uintptr
}

// SetSacrificialThread enables crash-isolation mode for this
// *BOF. Subsequent Execute calls spawn the entry on a dedicated
// OS thread; access violations inside the BOF mapping are
// intercepted by a process-wide VEH and converted to a clean
// Go error rather than a process crash.
//
// timeout caps how long the BOF thread is allowed to run.
// Zero disables sacrificial mode (Execute reverts to inline).
// Recommended floor: a few hundred milliseconds — sub-millisecond
// timeouts truncate to zero internally and WaitForSingleObject
// returns immediately. Recommended ceiling: a few hours —
// time.Duration values larger than ~49 days clamp to uint32 max
// (the WaitForSingleObject parameter limit).
//
// See the "Limitations" section in this file's header for the
// honest caveats (token scope, faults outside BOF range,
// TerminateThread + thunk-page leaks on timeout).
//
// Returns ErrAlreadyPrepared if called after the first Execute
// — the mode toggle is part of the prepared-state contract.
func (b *BOF) SetSacrificialThread(timeout time.Duration) error {
	bofMu.Lock()
	defer bofMu.Unlock()
	if b.prepared {
		return ErrAlreadyPrepared
	}
	b.sacrificialTimeout = timeout
	return nil
}

// callEntry dispatches the BOF entry call. In default mode it
// runs inline (current behaviour). In sacrificial mode it runs
// on a dedicated OS thread with VEH-mediated fault isolation.
// Caller must hold bofMu.
func (b *BOF) callEntry(args []byte) error {
	var argPtr, argLen uintptr
	// argPtr stays 0 when args is empty — passing &args[0] on a
	// zero-length slice panics. Native code sees a NULL+0 pair
	// which BOFs handle as "no args".
	if len(args) > 0 {
		argPtr = uintptr(unsafe.Pointer(&args[0]))
		argLen = uintptr(len(args))
	}
	if b.sacrificialTimeout == 0 {
		// Inline path — preserves original Execute semantics.
		func() {
			defer func() {
				if r := recover(); r != nil {
					b.errors.write([]byte(fmt.Sprintf("bof: panic during entry: %v\n", r)))
				}
			}()
			syscallN(b.entryAddr, argPtr, argLen)
		}()
		return nil
	}
	return b.callEntrySacrificial(argPtr, argLen)
}

// callEntrySacrificial runs the entry on a dedicated thread and
// waits with the configured timeout. The per-call sacArgs capsule
// is small (24 B Go heap) and held alive via the sacrificialMap
// entry; on clean return the entry is removed and Go GC reclaims
// it. On timeout the entry is kept (we can't tell whether the
// terminated thread is still dereferencing it) and the capsule
// leaks — but it's tiny, and bounded by the operator's timeouts.
func (b *BOF) callEntrySacrificial(argPtr, argLen uintptr) error {
	if err := installSacrificialVEH(); err != nil {
		return fmt.Errorf("runtime/bof: VEH install: %w", err)
	}
	if err := installSharedTrampoline(); err != nil {
		return fmt.Errorf("runtime/bof: trampoline install: %w", err)
	}

	frame := &sacFrame{argPtr: argPtr, argLen: argLen, entry: b.entryAddr, bof: b}

	const createSuspended uint32 = 0x4
	hThread, _, e := api.ProcCreateThread.Call(
		0,                              // lpThreadAttributes
		0,                              // dwStackSize (default)
		sharedTrampolineAddr,           // lpStartAddress
		uintptr(unsafe.Pointer(frame)), // lpParameter → rcx in trampoline
		uintptr(createSuspended),
		0, // lpThreadId out — we use GetThreadId on the returned handle
	)
	if hThread == 0 {
		// CreateThread failed before any thread could read frame —
		// Go GC will reclaim it as soon as we return.
		return fmt.Errorf("runtime/bof: CreateThread: %w", e)
	}
	defer windows.CloseHandle(windows.Handle(hThread))

	tidR, _, _ := api.ProcGetThreadId.Call(hThread)
	tid := uint32(tidR)

	sacrificialMap.Store(tid, frame)
	b.fault = faultRecord{} // clear from any previous call

	// ResumeThread returns the previous suspend count on success,
	// 0xFFFFFFFF on failure (x/sys/windows surfaces the latter as
	// the err return).
	prevSuspend, err := windows.ResumeThread(windows.Handle(hThread))
	if prevSuspend == ^uint32(0) {
		// Thread was created suspended and never started; safe
		// to drop the registry entry.
		_, _, _ = api.ProcTerminateThread.Call(hThread, 1)
		sacrificialMap.Delete(tid)
		return fmt.Errorf("runtime/bof: ResumeThread: %w", err)
	}

	timeoutMs := durationToMs(b.sacrificialTimeout)
	wait, err := windows.WaitForSingleObject(windows.Handle(hThread), timeoutMs)
	if err != nil {
		// Wait itself failed (rare — invalid handle, etc.). Keep
		// the registry entry; the thread may still be running.
		return fmt.Errorf("runtime/bof: WaitForSingleObject: %w", err)
	}
	if wait == uint32(windows.WAIT_TIMEOUT) {
		_, _, _ = api.ProcTerminateThread.Call(hThread, 1)
		// Capsule leaks — TerminateThread is asynchronous and
		// the kernel offers no way to confirm the thread is
		// fully stopped. Leaving the sacrificialMap entry in
		// place pins the *sacArgs against the GC.
		return fmt.Errorf("runtime/bof: BOF timeout after %v", b.sacrificialTimeout)
	}
	// Thread terminated cleanly (either ran to completion OR
	// faulted into the exit-stub which called ExitThread(1)).
	// Safe to drop the entry; Go GC reclaims the capsule.
	sacrificialMap.Delete(tid)
	// Pin the frame until after Delete: the trampoline's last
	// memory read is fenced by WaitForSingleObject's kernel-side
	// acquire, but Go's escape analyser doesn't see that — keep
	// the local ref live so the compiler can't shorten lifetime.
	runtime.KeepAlive(frame)

	code := atomic.LoadUint32(&b.fault.code)
	if code != 0 {
		pc := atomic.LoadUintptr(&b.fault.pc)
		return fmt.Errorf("runtime/bof: BOF crashed with exception 0x%x at PC 0x%x", code, pc)
	}
	return nil
}

// durationToMs converts a time.Duration to the uint32 milliseconds
// WaitForSingleObject expects. Clamps to uint32 max (≈49.7 days)
// and to a minimum of 1 ms for non-zero positive durations — a
// zero argument would mean "no wait, return immediately", which
// is never what a sacrificial caller intends.
func durationToMs(d time.Duration) uint32 {
	if d <= 0 {
		return 0
	}
	ms := int64(d / time.Millisecond)
	if ms <= 0 {
		return 1
	}
	// INFINITE is 0xFFFFFFFF; cap one below so callers never
	// pass "wait forever" accidentally via a giant Duration.
	const maxWaitMs = int64(0xFFFFFFFE)
	if ms > maxWaitMs {
		return uint32(maxWaitMs)
	}
	return uint32(ms)
}

// installSharedTrampoline lazily builds the single per-process
// RX stub that demultiplexes a *sacArgs into the BOF's two-arg
// ABI. CreateThread lands the lpParameter value in rcx on x64;
// the stub reads argPtr/argLen/entry from that struct and tail-
// calls the entry. Reading entry first (into rax) lets us clobber
// rcx last, since the source struct address lives in rcx.
//
// Has its own sync.Once (independent of the VEH install) so the
// two concerns evolve separately — a future eviction strategy or
// a unit test that wants to rebuild the trampoline does not have
// to reinitialise the exception handler.
func installSharedTrampoline() error {
	trampolineInstallOnce.Do(func() {
		page, err := windows.VirtualAlloc(0, 4096,
			windows.MEM_COMMIT|windows.MEM_RESERVE,
			windows.PAGE_READWRITE)
		if err != nil {
			trampolineInstallErr = fmt.Errorf("VirtualAlloc(trampoline): %w", err)
			return
		}
		// Shared trampoline:
		//   48 8b 41 10            mov rax, [rcx+0x10]   ; entry  (sacArgs.entry,  offset 16)
		//   48 8b 51 08            mov rdx, [rcx+0x08]   ; argLen (sacArgs.argLen, offset  8)
		//   48 8b 09               mov rcx, [rcx]        ; argPtr (sacArgs.argPtr, offset  0) — clobbers source
		//   ff e0                  jmp rax
		stub := []byte{
			0x48, 0x8b, 0x41, 0x10,
			0x48, 0x8b, 0x51, 0x08,
			0x48, 0x8b, 0x09,
			0xff, 0xe0,
		}
		if err := writeRXStub(page, stub); err != nil {
			_ = windows.VirtualFree(page, 0, windows.MEM_RELEASE)
			trampolineInstallErr = fmt.Errorf("VirtualProtect(trampoline): %w", err)
			return
		}
		sharedTrampolineAddr = page
	})
	return trampolineInstallErr
}

// writeRXStub copies code into the page and flips the page to
// PAGE_EXECUTE_READ. Shared helper between installSharedTrampoline
// and the exit-stub installation in installSacrificialVEH so the
// two callsites stay in lock-step on alignment + protection
// semantics.
func writeRXStub(page uintptr, code []byte) error {
	dst := unsafe.Slice((*byte)(unsafe.Pointer(page)), len(code))
	copy(dst, code)
	var old uint32
	return windows.VirtualProtect(page, uintptr(len(code)), windows.PAGE_EXECUTE_READ, &old)
}

// installSacrificialVEH wires the process-wide VEH + exit stub.
// Idempotent via sync.Once. Returns the saved init error if a
// previous install failed; future Execute calls in sacrificial
// mode then surface that error rather than running unprotected.
//
// The VEH and the exit stub are intentionally NEVER removed —
// Windows doesn't expose a clean way to revoke a VEH that may
// have run on a thread we no longer own, and the exit stub's
// page must outlive any thread that has ever been redirected
// to it. Pay the perma-handler cost once per process; treat as
// a fixed opsec artefact.
func installSacrificialVEH() error {
	vehInstallOnce.Do(func() {
		page, err := windows.VirtualAlloc(0, 4096,
			windows.MEM_COMMIT|windows.MEM_RESERVE,
			windows.PAGE_READWRITE)
		if err != nil {
			vehInstallErr = fmt.Errorf("VirtualAlloc(stub): %w", err)
			return
		}
		k32 := windows.NewLazySystemDLL("kernel32.dll")
		exitThread := k32.NewProc("ExitThread")
		if err := exitThread.Find(); err != nil {
			vehInstallErr = fmt.Errorf("ExitThread proc: %w", err)
			return
		}
		etAddr := exitThread.Addr()

		// Exit stub:
		//   and rsp, -16          align rsp to 16 (Win64 ABI for `call`)
		//   sub rsp, 0x28         32B shadow space + 8B re-misalign so
		//                         post-`call` rsp ≡ 0 (mod 16)
		//   mov rcx, 1            uExitCode
		//   mov rax, ExitThread
		//   call rax              ExitThread never returns
		//   int3                  unreachable padding
		//
		// The redirected thread enters here with rsp = whatever
		// instruction faulted. Aligning first is mandatory:
		// ExitThread's prologue uses movaps on aligned stack
		// slots; a misaligned rsp at call time tears.
		var imm [8]byte
		binary.LittleEndian.PutUint64(imm[:], uint64(etAddr))
		buf := []byte{
			0x48, 0x83, 0xe4, 0xf0, // and rsp, -16
			0x48, 0x83, 0xec, 0x28, // sub rsp, 0x28
			0x48, 0xc7, 0xc1, 0x01, 0x00, 0x00, 0x00, // mov rcx, 1
			0x48, 0xb8, // mov rax, imm64
			imm[0], imm[1], imm[2], imm[3], imm[4], imm[5], imm[6], imm[7],
			0xff, 0xd0, 0xcc, // call rax; int3
		}

		if err := writeRXStub(page, buf); err != nil {
			_ = windows.VirtualFree(page, 0, windows.MEM_RELEASE)
			vehInstallErr = fmt.Errorf("VirtualProtect(stub): %w", err)
			return
		}
		exitStubAddr = page

		// Register the VEH at priority 1 (first to run).
		callback := syscall.NewCallback(vehBOFFault)
		r, _, e := api.ProcAddVectoredExceptionHandler.Call(1, callback)
		if r == 0 {
			vehInstallErr = fmt.Errorf("AddVectoredExceptionHandler: %w", e)
			return
		}
	})
	return vehInstallErr
}

// sacrificialBOFActive reports whether *any* registry entry
// points at this *BOF. Called by Close under bofMu to refuse
// freeing the mapping while a sacrificial thread is still
// running. Linear scan, but the registry is tiny (one entry
// per concurrent sacrificial Execute) so the cost is trivial.
func sacrificialBOFActive(b *BOF) bool {
	active := false
	sacrificialMap.Range(func(_, v any) bool {
		if v.(*sacFrame).bof == b {
			active = true
			return false // stop iteration
		}
		return true
	})
	return active
}

// vehBOFFault is the VEH callback. Walks the sacrificial registry
// looking for a *BOF whose mapping contains the fault address;
// if found, rewrites the thread's RIP to point at the exit stub
// + returns EXCEPTION_CONTINUE_EXECUTION so the kernel resumes
// the thread at the stub. Threads not in the registry
// (everything in the host implant) return CONTINUE_SEARCH so
// Go's runtime handler takes the normal path.
//
// info points at an EXCEPTION_POINTERS struct:
//
//	struct EXCEPTION_POINTERS {
//	    EXCEPTION_RECORD *ExceptionRecord;
//	    CONTEXT          *ContextRecord;
//	};
//
// Both fields are pointers; CONTEXT.Rip lives at offset 0xF8.
//
// b.execMem is read without bofMu — safe because a *BOF is in
// the sacrificialMap only between ResumeThread and the deferred
// Delete, a window during which Close cannot run (it would block
// on bofMu held by the host's Execute call). The worst case
// (Close races with the VEH and reads execMem=0) falls cleanly
// through to CONTINUE_SEARCH, which Go's handler then turns
// into a process exit — losing the isolation, not corrupting
// anything.
func vehBOFFault(info uintptr) uintptr {
	const (
		exceptionContinueExecution = ^uintptr(0) // -1
		exceptionContinueSearch    = uintptr(0)
	)
	if info == 0 {
		return exceptionContinueSearch
	}
	recPtr := *(*uintptr)(unsafe.Pointer(info))
	ctxPtr := *(*uintptr)(unsafe.Pointer(info + 8))
	if recPtr == 0 || ctxPtr == 0 {
		return exceptionContinueSearch
	}
	exceptionCode := *(*uint32)(unsafe.Pointer(recPtr))
	exceptionAddr := *(*uintptr)(unsafe.Pointer(recPtr + 16))

	tid := windows.GetCurrentThreadId()
	frameI, ok := sacrificialMap.Load(tid)
	if !ok {
		return exceptionContinueSearch
	}
	b := frameI.(*sacFrame).bof
	if b.execMem == 0 || exceptionAddr < b.execMem || exceptionAddr >= b.execMem+b.execMemSize {
		return exceptionContinueSearch
	}

	// Record + redirect. CONTEXT.Rip is at offset 0xF8 on x64.
	atomic.StoreUint32(&b.fault.code, exceptionCode)
	atomic.StoreUintptr(&b.fault.pc, exceptionAddr)
	*(*uintptr)(unsafe.Pointer(ctxPtr + 0xF8)) = exitStubAddr
	return exceptionContinueExecution
}
