//go:build windows

package bof

import "encoding/binary"

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
	imageRelAMD64Rel32      = 0x0004
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
