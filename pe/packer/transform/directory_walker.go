package transform

import (
	"encoding/binary"
	"fmt"
)

// DirectoryPatchEvent is the sealed sum type yielded by the unified
// data-directory walkers ([DirectoryWalkers]). Variants describe
// the different shapes of "loader-relevant pointer the linker
// baked into a PE":
//
//   - [RVAFileOffEvent] — a uint32 RVA at a known file offset
//     (IMPORT-directory descriptors, RESOURCE-directory leaves).
//   - [BaseRelocEvent] — one BASERELOC entry plus block context
//     (PageRVA header offset, entry index within the block, the
//     target RVA whose absolute pointer needs bumping, and the
//     pointer's byte size 4/8).
//
// Why a sum and not a flat (FileOff, Size) struct: base-reloc
// entries describe an absolute pointer whose **file** location must
// be computed from `e.RVA` via the section table. ShiftImageVA in
// particular resolves the file offset against a pre-shift snapshot
// of the section ranges (the walker can't do that translation in
// general because it doesn't know whether the caller wants OLD or
// NEW coordinates). Yielding the raw block + RVA + size keeps the
// abstraction honest.
//
// Adding a future data-directory walker (e.g., EXCEPTION,
// LOAD_CONFIG, EXPORT) is one new entry in [DirectoryWalkers] and
// (if a new event shape is required) one new variant here. R2 from
// docs/refactor-2026-doc/audit-2026-04-27.md /
// docs/refactor-2026-doc/packer-improvements-2026-05-12.md.
type DirectoryPatchEvent interface {
	isDirectoryPatchEvent()
}

// RVAFileOffEvent describes a uint32 RVA at a concrete file
// position. Yielded by the IMPORT and RESOURCE walkers, where the
// linker bakes RVAs as plain dwords the loader rebases internally
// (NOT covered by the BASERELOC table). To rebase, the caller
// reads the dword, adds the image-shift delta, writes it back.
type RVAFileOffEvent struct {
	// FileOff is the absolute byte offset inside the PE buffer
	// where the uint32 RVA lives.
	FileOff uint32
}

func (RVAFileOffEvent) isDirectoryPatchEvent() {}

// BaseRelocEvent describes one entry in the BASERELOC directory.
// Yielded once per entry, including padding [RelTypeAbsolute]
// entries (PtrSize=0 — the caller skips them).
//
// The shape mirrors [BaseRelocEntry] exactly, plus PtrSize already
// resolved to a byte count so callers don't re-classify by Type.
// Block-level patching (PageRVA dword at BlockOff) is the caller's
// responsibility, gated on `EntryIdx == 0` to fire it exactly once
// per block; the walker yields ascending-order entries so that
// detection is reliable.
type BaseRelocEvent struct {
	// BlockOff is the file offset of the block header's PageRVA
	// dword (a uint32). Bump it by delta once per block —
	// EntryIdx==0 is the canonical detector.
	BlockOff uint32
	// BlockVA is the current PageRVA value (pre-shift coordinates
	// when the walker runs before the caller's mutation pass).
	BlockVA uint32
	// EntryIdx is 0..NumEntries-1 within the block.
	EntryIdx uint32
	// RVA is the target the entry's absolute pointer points at
	// (PageRVA + entry.offset12).
	RVA uint32
	// Type is the raw IMAGE_REL_BASED_* code ([RelTypeAbsolute],
	// [RelTypeHighLow], [RelTypeDir64]). Preserved for callers
	// that need the original classification.
	Type uint16
	// PtrSize is the byte width of the absolute pointer the entry
	// describes — 4 for HIGHLOW, 8 for DIR64, 0 for padding
	// (Absolute). Convenience: callers can switch on this without
	// re-deriving from Type.
	PtrSize uint8
}

func (BaseRelocEvent) isDirectoryPatchEvent() {}

// DirectoryWalker walks one PE data directory and yields each
// patch site as a [DirectoryPatchEvent]. The walker may emit
// multiple variants (e.g., a BASERELOC walker yields only
// [BaseRelocEvent], an IMPORT walker yields only [RVAFileOffEvent]).
//
// Callers SHOULD use [DirectoryWalkers] to enumerate every
// registered walker rather than calling individuals — the loop
// stays open to future walker registrations.
type DirectoryWalker func(pe []byte, cb func(DirectoryPatchEvent) error) error

// DirectoryWalkers is the registry of unified directory walkers
// indexed by IMAGE_DIRECTORY_ENTRY_* index. The map shape lets
// callers iterate every registered walker without naming them
// individually — adding EXCEPTION / LOAD_CONFIG / EXPORT walkers
// is a single new entry.
//
// Underlying primitives ([WalkBaseRelocs], [WalkImportDirectoryRVAs],
// [WalkResourceDirectoryRVAs]) stay exported for callers that need
// the typed callback shape. The walkers here are thin adapters.
var DirectoryWalkers = map[int]DirectoryWalker{
	dirImport:    walkImportPatchEvents,
	dirResource:  walkResourcePatchEvents,
	dirBaseReloc: walkBaseRelocPatchEvents,
}

// walkImportPatchEvents adapts [WalkImportDirectoryRVAs] to the
// unified event shape. Every yield is an [RVAFileOffEvent].
func walkImportPatchEvents(pe []byte, cb func(DirectoryPatchEvent) error) error {
	return WalkImportDirectoryRVAs(pe, func(rvaFileOff uint32) error {
		return cb(RVAFileOffEvent{FileOff: rvaFileOff})
	})
}

// walkResourcePatchEvents adapts [WalkResourceDirectoryRVAs] to the
// unified event shape.
func walkResourcePatchEvents(pe []byte, cb func(DirectoryPatchEvent) error) error {
	return WalkResourceDirectoryRVAs(pe, func(rvaFileOff uint32) error {
		return cb(RVAFileOffEvent{FileOff: rvaFileOff})
	})
}

// walkBaseRelocPatchEvents adapts [WalkBaseRelocs]. PtrSize is
// pre-resolved from the reloc type so callers don't switch on
// Type themselves.
func walkBaseRelocPatchEvents(pe []byte, cb func(DirectoryPatchEvent) error) error {
	return WalkBaseRelocs(pe, func(e BaseRelocEntry) error {
		var ptrSize uint8
		switch e.Type {
		case RelTypeAbsolute:
			ptrSize = 0
		case RelTypeHighLow:
			ptrSize = 4
		case RelTypeDir64:
			ptrSize = 8
		default:
			return fmt.Errorf("transform: unsupported reloc type 0x%x at RVA 0x%x", e.Type, e.RVA)
		}
		return cb(BaseRelocEvent{
			BlockOff: e.BlockOff,
			BlockVA:  e.BlockVA,
			EntryIdx: e.EntryIdx,
			RVA:      e.RVA,
			Type:     e.Type,
			PtrSize:  ptrSize,
		})
	})
}

// ApplyRVAShiftAllDirectories walks every directory registered in
// [DirectoryWalkers] and bumps each embedded RVA / absolute pointer
// by `delta`. Reads structural metadata from `pe` (the pre-shift
// snapshot — section headers still carry the OLD VAs needed to
// resolve descriptor RVAs) and applies the patches to `out` (which
// already carries the NEW section VAs after the caller's structural
// pass). `rvaToFile` resolves a pre-shift RVA to its file offset
// inside `out` (typically a closure over a section-ranges snapshot
// captured before the section-table mutation).
//
// Returns the count of patch sites visited. The split between
// "yield events" (DirectoryWalkers) and "apply delta" (this fn) is
// deliberate — callers that need a different bump semantic (e.g.,
// recording sites for an audit log instead of patching) reuse the
// walkers without inheriting the patch logic.
//
// Used by [ShiftImageVA] to fold its three previously-manual
// directory-fixup passes into one loop.
func ApplyRVAShiftAllDirectories(pe, out []byte, delta uint32, rvaToFile func(uint32) (uint32, error)) (int, error) {
	const rvaWidth = 4 // every RVA / 32-bit pointer width in PE32+ data dirs
	count := 0
	for dirIdx, walker := range DirectoryWalkers {
		walkErr := walker(pe, func(ev DirectoryPatchEvent) error {
			switch e := ev.(type) {
			case RVAFileOffEvent:
				if int(e.FileOff)+rvaWidth > len(out) {
					return fmt.Errorf("RVA patch past EOF (file 0x%x)", e.FileOff)
				}
				cur := binary.LittleEndian.Uint32(out[e.FileOff:])
				if cur == 0 {
					return nil
				}
				binary.LittleEndian.PutUint32(out[e.FileOff:], cur+delta)
				count++
				return nil
			case BaseRelocEvent:
				if e.EntryIdx == 0 {
					if int(e.BlockOff)+rvaWidth > len(out) {
						return fmt.Errorf("BASERELOC block header past EOF (file 0x%x)", e.BlockOff)
					}
					binary.LittleEndian.PutUint32(out[e.BlockOff:], e.BlockVA+delta)
					count++
				}
				if e.PtrSize == 0 {
					return nil // padding entry; no pointer to patch.
				}
				fileOff, ferr := rvaToFile(e.RVA)
				if ferr != nil {
					return fmt.Errorf("reloc at RVA 0x%x: %w", e.RVA, ferr)
				}
				if int(fileOff)+int(e.PtrSize) > len(out) {
					return fmt.Errorf("reloc patch past EOF (RVA 0x%x → file 0x%x)", e.RVA, fileOff)
				}
				switch e.PtrSize {
				case 4:
					val := binary.LittleEndian.Uint32(out[fileOff:])
					binary.LittleEndian.PutUint32(out[fileOff:], val+delta)
				case 8:
					val := binary.LittleEndian.Uint64(out[fileOff:])
					binary.LittleEndian.PutUint64(out[fileOff:], val+uint64(delta))
				}
				count++
				return nil
			default:
				return fmt.Errorf("transform: unknown DirectoryPatchEvent variant %T", ev)
			}
		})
		if walkErr != nil {
			return count, fmt.Errorf("transform: directory %d fixup: %w", dirIdx, walkErr)
		}
	}
	return count, nil
}
