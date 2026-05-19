//go:build windows

package bof

import "fmt"

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
