package packer_test

import (
	"bytes"
	"debug/pe"
	"os"
	"path/filepath"
	"strings"
	"testing"

	packerpkg "github.com/oioio-space/maldev/pe/packer"
)

// TestPackBinary_DefaultStubSectionName_IsRandomized pins the
// default behaviour: with zero-value options the appended stub
// section name is NOT ".mldv" — Phase 2-A randomisation is the
// default since Item #3 of packer-actions-2026-05-12.
func TestPackBinary_DefaultStubSectionName_IsRandomized(t *testing.T) {
	input := winhelloFixture(t)
	out, _, err := packerpkg.PackBinary(input, packerpkg.PackBinaryOptions{
		Format:       packerpkg.FormatWindowsExe,
		Stage1Rounds: 3,
		Seed:         42,
	})
	if err != nil {
		t.Fatalf("PackBinary: %v", err)
	}
	got := lastSectionName(t, out)
	if got == ".mldv" {
		t.Errorf("default stub section name = %q — randomisation regression (was flipped ON by default 2026-05-16)", got)
	}
	if !strings.HasPrefix(got, ".") {
		t.Errorf("default stub section name = %q, want '.' prefix (MSVC convention)", got)
	}
}

// TestPackBinary_KeepDefaultStubSectionName_OptsIntoMldv covers
// the opt-out path: operators who need byte-reproducible
// differential output explicitly request the historic ".mldv".
func TestPackBinary_KeepDefaultStubSectionName_OptsIntoMldv(t *testing.T) {
	input := winhelloFixture(t)
	out, _, err := packerpkg.PackBinary(input, packerpkg.PackBinaryOptions{
		Format:                     packerpkg.FormatWindowsExe,
		Stage1Rounds:               3,
		Seed:                       42,
		KeepDefaultStubSectionName: true,
	})
	if err != nil {
		t.Fatalf("PackBinary: %v", err)
	}
	if got := lastSectionName(t, out); got != ".mldv" {
		t.Errorf("KeepDefaultStubSectionName=true should yield %q, got %q", ".mldv", got)
	}
}

// TestPackBinary_RandomStubSectionName_Differs verifies that under
// the default (randomisation on), two different seeds produce
// different section names — sanity check that the RNG is wired.
func TestPackBinary_RandomStubSectionName_Differs(t *testing.T) {
	input := winhelloFixture(t)

	out1, _, err := packerpkg.PackBinary(input, packerpkg.PackBinaryOptions{
		Format:       packerpkg.FormatWindowsExe,
		Stage1Rounds: 3,
		Seed:         42,
	})
	if err != nil {
		t.Fatalf("PackBinary seed=42: %v", err)
	}
	out2, _, err := packerpkg.PackBinary(input, packerpkg.PackBinaryOptions{
		Format:       packerpkg.FormatWindowsExe,
		Stage1Rounds: 3,
		Seed:         1337,
	})
	if err != nil {
		t.Fatalf("PackBinary seed=1337: %v", err)
	}

	name1, name2 := lastSectionName(t, out1), lastSectionName(t, out2)
	if name1 == ".mldv" || name2 == ".mldv" {
		t.Errorf("randomised names should not equal default %q (got %q, %q)", ".mldv", name1, name2)
	}
	if name1 == name2 {
		t.Errorf("seeds 42 and 1337 produced identical section names %q — RNG not seeded properly?", name1)
	}
}

// TestPackBinary_RandomStubSectionName_DeterministicGivenSeed
// confirms the same seed produces the same section name — the
// reproducible-build property operators rely on for deterministic
// batch packs.
func TestPackBinary_RandomStubSectionName_DeterministicGivenSeed(t *testing.T) {
	input := winhelloFixture(t)
	opts := packerpkg.PackBinaryOptions{
		Format:       packerpkg.FormatWindowsExe,
		Stage1Rounds: 3,
		Seed:         999,
	}
	a, _, err := packerpkg.PackBinary(input, opts)
	if err != nil {
		t.Fatalf("PackBinary A: %v", err)
	}
	b, _, err := packerpkg.PackBinary(input, opts)
	if err != nil {
		t.Fatalf("PackBinary B: %v", err)
	}
	if lastSectionName(t, a) != lastSectionName(t, b) {
		t.Errorf("same seed produced different section names: %q vs %q",
			lastSectionName(t, a), lastSectionName(t, b))
	}
}

// winhelloFixture returns the bytes of testdata/winhello.exe,
// or skips the test when the fixture is missing (script-built
// from a Windows VM, not committed to the repo).
func winhelloFixture(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("testdata", "winhello.exe")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("Windows fixture missing (%v); run scripts/build-winhello.sh", err)
	}
	return b
}

// lastSectionName returns the section name of the last entry in
// the PE section table — the one PackBinary just appended.
func lastSectionName(t *testing.T, peBytes []byte) string {
	t.Helper()
	f, err := pe.NewFile(bytes.NewReader(peBytes))
	if err != nil {
		t.Fatalf("debug/pe rejected packed output: %v", err)
	}
	defer f.Close()
	if len(f.Sections) == 0 {
		t.Fatal("packed output has zero sections")
	}
	return f.Sections[len(f.Sections)-1].Name
}
