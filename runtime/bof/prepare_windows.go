//go:build windows

package bof

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

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
