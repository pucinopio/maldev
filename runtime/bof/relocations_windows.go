//go:build windows

package bof

import (
	"encoding/binary"
	"fmt"
)

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
