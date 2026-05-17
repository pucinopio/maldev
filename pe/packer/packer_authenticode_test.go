package packer_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	packerpkg "github.com/oioio-space/maldev/pe/packer"
	"github.com/oioio-space/maldev/pe/packer/transform"
)

// stampSecurityDirectoryEntry writes a non-zero SECURITY data-
// directory pointer into pe so tests can prove a downstream
// strip / preserve actually did something.
func stampSecurityDirectoryEntry(t *testing.T, pe []byte) {
	t.Helper()
	peOff := binary.LittleEndian.Uint32(pe[transform.PEELfanewOffset:])
	coffOff := peOff + transform.PESignatureSize
	optOff := coffOff + transform.PECOFFHdrSize
	secEntry := optOff + transform.OptDataDirsStart + 4*transform.OptDataDirEntrySize
	binary.LittleEndian.PutUint32(pe[secEntry:], 0x12345678)
	binary.LittleEndian.PutUint32(pe[secEntry+4:], 0x00001234)
}

// readSecurityDirectoryEntry returns the (VA, Size) pair from
// DataDirectory[SECURITY] of pe.
func readSecurityDirectoryEntry(t *testing.T, pe []byte) (uint32, uint32) {
	t.Helper()
	peOff := binary.LittleEndian.Uint32(pe[transform.PEELfanewOffset:])
	coffOff := peOff + transform.PESignatureSize
	optOff := coffOff + transform.PECOFFHdrSize
	secEntry := optOff + transform.OptDataDirsStart + 4*transform.OptDataDirEntrySize
	va := binary.LittleEndian.Uint32(pe[secEntry:])
	sz := binary.LittleEndian.Uint32(pe[secEntry+4:])
	return va, sz
}

// TestPackBinary_StripsAuthenticodeByDefault verifies the
// default-on cert-strip: a stamped non-zero SECURITY directory
// is zeroed in the packed output.
func TestPackBinary_StripsAuthenticodeByDefault(t *testing.T) {
	in, err := transform.BuildMinimalPE32Plus(bytes.Repeat([]byte{0xC3}, 0x100))
	if err != nil {
		t.Fatalf("BuildMinimalPE32Plus: %v", err)
	}
	stampSecurityDirectoryEntry(t, in)

	out, _, err := packerpkg.PackBinary(in, packerpkg.PackBinaryOptions{
		Format:       packerpkg.FormatWindowsExe,
		Stage1Rounds: 3,
		Seed:         42,
	})
	if err != nil {
		t.Fatalf("PackBinary: %v", err)
	}

	va, sz := readSecurityDirectoryEntry(t, out)
	if va != 0 || sz != 0 {
		t.Errorf("DataDirectory[SECURITY] = (%#x, %#x), want (0, 0) — strip default broken", va, sz)
	}
}

// TestPackBinary_PreserveAuthenticodeDirectory keeps the
// (now-tampered) cert pointer when the operator opts in. Item #8.
func TestPackBinary_PreserveAuthenticodeDirectory(t *testing.T) {
	in, err := transform.BuildMinimalPE32Plus(bytes.Repeat([]byte{0xC3}, 0x100))
	if err != nil {
		t.Fatalf("BuildMinimalPE32Plus: %v", err)
	}
	stampSecurityDirectoryEntry(t, in)

	out, _, err := packerpkg.PackBinary(in, packerpkg.PackBinaryOptions{
		Format:                        packerpkg.FormatWindowsExe,
		Stage1Rounds:                  3,
		Seed:                          42,
		PreserveAuthenticodeDirectory: true,
	})
	if err != nil {
		t.Fatalf("PackBinary: %v", err)
	}

	va, sz := readSecurityDirectoryEntry(t, out)
	if va != 0x12345678 || sz != 0x00001234 {
		t.Errorf("DataDirectory[SECURITY] = (%#x, %#x), want (0x12345678, 0x1234) — preserve opt-out broken", va, sz)
	}
}
