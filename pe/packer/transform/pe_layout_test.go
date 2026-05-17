package transform_test

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/oioio-space/maldev/pe/packer/transform"
)

// TestValidateAMD64PE32Plus_AcceptsMinimalPE — the canonical
// minimal PE32+ fixture used everywhere else in the transform
// tests should pass validation cleanly.
func TestValidateAMD64PE32Plus_AcceptsMinimalPE(t *testing.T) {
	pe, err := transform.BuildMinimalPE32Plus([]byte{0xC3})
	if err != nil {
		t.Fatalf("BuildMinimalPE32Plus: %v", err)
	}
	if err := transform.ValidateAMD64PE32Plus(pe); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestValidateAMD64PE32Plus_RejectsX86Machine — flip the Machine
// field to IMAGE_FILE_MACHINE_I386 (0x014C) and expect
// ErrUnsupportedMachine.
func TestValidateAMD64PE32Plus_RejectsX86Machine(t *testing.T) {
	pe, err := transform.BuildMinimalPE32Plus([]byte{0xC3})
	if err != nil {
		t.Fatalf("BuildMinimalPE32Plus: %v", err)
	}
	// Locate the COFF Machine field and overwrite it.
	peOff := binary.LittleEndian.Uint32(pe[transform.PEELfanewOffset:])
	coffOff := peOff + transform.PESignatureSize
	binary.LittleEndian.PutUint16(pe[coffOff+transform.COFFMachineOffset:], 0x014C) // i386
	err = transform.ValidateAMD64PE32Plus(pe)
	if !errors.Is(err, transform.ErrUnsupportedMachine) {
		t.Errorf("expected ErrUnsupportedMachine, got %v", err)
	}
}

// TestValidateAMD64PE32Plus_RejectsPE32Magic — flip Optional
// Header Magic from PE32+ (0x020B) to PE32 (0x010B) and expect
// ErrUnsupportedOptMagic.
func TestValidateAMD64PE32Plus_RejectsPE32Magic(t *testing.T) {
	pe, err := transform.BuildMinimalPE32Plus([]byte{0xC3})
	if err != nil {
		t.Fatalf("BuildMinimalPE32Plus: %v", err)
	}
	peOff := binary.LittleEndian.Uint32(pe[transform.PEELfanewOffset:])
	coffOff := peOff + transform.PESignatureSize
	optOff := coffOff + transform.PECOFFHdrSize
	binary.LittleEndian.PutUint16(pe[optOff+transform.OptMagicOffset:], 0x010B) // PE32
	err = transform.ValidateAMD64PE32Plus(pe)
	if !errors.Is(err, transform.ErrUnsupportedOptMagic) {
		t.Errorf("expected ErrUnsupportedOptMagic, got %v", err)
	}
}

// TestValidateAMD64PE32Plus_RejectsTruncated — short buffer must
// return a layout-parse error (not panic).
func TestValidateAMD64PE32Plus_RejectsTruncated(t *testing.T) {
	if err := transform.ValidateAMD64PE32Plus([]byte{0x4D, 0x5A}); err == nil {
		t.Error("expected error on truncated input, got nil")
	}
}
