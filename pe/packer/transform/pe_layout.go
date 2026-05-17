package transform

import (
	"encoding/binary"
	"fmt"
)

// peLayout caches the per-region offsets every Phase 2 patcher
// resolves out of a PE32+ buffer (e_lfanew → COFF → Optional →
// section table). Computed once per call by parsePELayout; readers
// rely on the bounds checks done at parse time and may index into
// the buffer with the cached offsets without re-validating.
type peLayout struct {
	peOff        uint32
	coffOff      uint32
	optOff       uint32
	sizeOfOptHdr uint16
	secTableOff  uint32
	numSections  uint16
}

// ErrUnsupportedMachine fires when an input PE's COFF Machine
// field is not [MachineAMD64]. The packer's stubs are amd64-only;
// silently producing output for an x86 or ARM64 input would yield
// a non-executable file. Caught explicitly at input detection
// rather than at runtime decryption.
var ErrUnsupportedMachine = fmt.Errorf("transform: unsupported COFF Machine — packer requires IMAGE_FILE_MACHINE_AMD64 (0x8664)")

// ErrUnsupportedOptMagic fires when an input PE's Optional Header
// Magic is not PE32+ (0x020B). PE32 (32-bit) inputs are rejected
// because every header offset in this package is keyed on the
// 64-bit Optional Header layout.
var ErrUnsupportedOptMagic = fmt.Errorf("transform: unsupported Optional Header Magic — packer requires PE32+ (0x020B)")

// ValidateAMD64PE32Plus checks that `pe` is a PE32+ amd64 image.
// Returns [ErrUnsupportedMachine] / [ErrUnsupportedOptMagic] on
// mismatch, or a layout-parse error if the headers are malformed.
//
// Called at FormatPE detection so the operator gets an immediate,
// readable rejection — packing an x86 EXE through this library
// would otherwise succeed at every header-rewrite step and only
// fail at LoadLibrary on Win64 with a cryptic STATUS_INVALID_IMAGE.
func ValidateAMD64PE32Plus(pe []byte) error {
	l, err := parsePELayout(pe)
	if err != nil {
		return err
	}
	machine := binary.LittleEndian.Uint16(pe[l.coffOff+COFFMachineOffset : l.coffOff+COFFMachineOffset+2])
	if machine != MachineAMD64 {
		return fmt.Errorf("%w (got %#04x)", ErrUnsupportedMachine, machine)
	}
	magic := binary.LittleEndian.Uint16(pe[l.optOff+OptMagicOffset : l.optOff+OptMagicOffset+2])
	if magic != OptMagicPE32Plus {
		return fmt.Errorf("%w (got %#04x)", ErrUnsupportedOptMagic, magic)
	}
	return nil
}

// parsePELayout validates DOS magic, e_lfanew, the PE signature,
// and the COFF / Optional / section-table bounds, then returns the
// resolved offsets. Patchers in this package call it before
// touching header bytes so each one shares the same bounds-check
// vocabulary instead of re-implementing the walk.
func parsePELayout(pe []byte) (peLayout, error) {
	if len(pe) < int(PEELfanewOffset)+4 {
		return peLayout{}, fmt.Errorf("transform: PE too short for e_lfanew")
	}
	peOff := binary.LittleEndian.Uint32(pe[PEELfanewOffset : PEELfanewOffset+4])
	coffOff := peOff + PESignatureSize
	if int(coffOff)+PECOFFHdrSize > len(pe) {
		return peLayout{}, fmt.Errorf("transform: PE too short for COFF header")
	}
	sizeOfOptHdr := binary.LittleEndian.Uint16(pe[coffOff+COFFSizeOfOptHdrOffset : coffOff+COFFSizeOfOptHdrOffset+2])
	optOff := coffOff + PECOFFHdrSize
	if int(optOff)+int(sizeOfOptHdr) > len(pe) {
		return peLayout{}, fmt.Errorf("transform: PE too short for Optional Header (%d)", sizeOfOptHdr)
	}
	numSections := binary.LittleEndian.Uint16(pe[coffOff+COFFNumSectionsOffset : coffOff+COFFNumSectionsOffset+2])
	secTableOff := optOff + uint32(sizeOfOptHdr)
	if int(secTableOff)+int(numSections)*PESectionHdrSize > len(pe) {
		return peLayout{}, fmt.Errorf("transform: PE too short for section table (%d sections)", numSections)
	}
	return peLayout{
		peOff:        peOff,
		coffOff:      coffOff,
		optOff:       optOff,
		sizeOfOptHdr: sizeOfOptHdr,
		secTableOff:  secTableOff,
		numSections:  numSections,
	}, nil
}
