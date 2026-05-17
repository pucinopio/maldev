package transform_test

import (
	"encoding/binary"
	"testing"

	"github.com/oioio-space/maldev/pe/packer/transform"
)

// TestDirectoryWalkers_RegistryShape — three entries (Import,
// Resource, BaseReloc), each callable, none nil.
func TestDirectoryWalkers_RegistryShape(t *testing.T) {
	if len(transform.DirectoryWalkers) != 3 {
		t.Errorf("DirectoryWalkers len = %d, want 3", len(transform.DirectoryWalkers))
	}
	for idx, walker := range transform.DirectoryWalkers {
		if walker == nil {
			t.Errorf("DirectoryWalkers[%d] is nil", idx)
		}
	}
}

// TestDirectoryWalkers_BaseReloc_YieldsBaseRelocEvents — the
// base-reloc walker must emit BaseRelocEvent values, with PtrSize
// pre-resolved from Type (Absolute=0, HighLow=4, Dir64=8).
func TestDirectoryWalkers_BaseReloc_YieldsBaseRelocEvents(t *testing.T) {
	pe := peWithRelocs(t)
	walker := transform.DirectoryWalkers[5]
	if walker == nil {
		t.Fatal("DirectoryWalkers[5] (BaseReloc) is nil")
	}
	var dir64, abs int
	err := walker(pe, func(ev transform.DirectoryPatchEvent) error {
		e, ok := ev.(transform.BaseRelocEvent)
		if !ok {
			t.Fatalf("non-BaseRelocEvent yielded: %T", ev)
		}
		switch e.Type {
		case transform.RelTypeDir64:
			if e.PtrSize != 8 {
				t.Errorf("Dir64 PtrSize = %d, want 8", e.PtrSize)
			}
			dir64++
		case transform.RelTypeAbsolute:
			if e.PtrSize != 0 {
				t.Errorf("Absolute PtrSize = %d, want 0", e.PtrSize)
			}
			abs++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walker: %v", err)
	}
	if dir64 != 2 {
		t.Errorf("Dir64 entries = %d, want 2", dir64)
	}
	if abs != 4 {
		t.Errorf("Absolute padding entries = %d, want 4", abs)
	}
}

// TestApplyRVAShiftAllDirectories_BumpsAllSites — feed the helper a
// minimal PE with one BASERELOC block (DIR64 entries) + non-zero
// content at the entry targets, apply a delta, assert every patched
// site bumped by exactly delta.
func TestApplyRVAShiftAllDirectories_BumpsAllSites(t *testing.T) {
	pe := peWithRelocs(t)

	// Route the two DIR64 entry targets to file offsets that DON'T
	// overlap the reloc block itself (which lives at file 0x410..0x428).
	// The synthetic peWithRelocs fixture has entries pointing at
	// page-relative offsets 0x020 / 0x040 (RVAs 0x1020 / 0x1040) —
	// in a real PE those would land in code/data; here we just
	// remap them to a scratch zone at 0x500 / 0x508 so the walker
	// can iterate the block without seeing its own bytes mutated.
	const baseVal uint64 = 0x140001234
	binary.LittleEndian.PutUint64(pe[0x500:], baseVal)
	binary.LittleEndian.PutUint64(pe[0x508:], baseVal)

	const delta uint32 = 0x10000
	rvaToFile := func(rva uint32) (uint32, error) {
		switch rva {
		case 0x1020:
			return 0x500, nil
		case 0x1040:
			return 0x508, nil
		}
		return 0, nil
	}
	patched, err := transform.ApplyRVAShiftAllDirectories(pe, pe, delta, rvaToFile)
	if err != nil {
		t.Fatalf("ApplyRVAShiftAllDirectories: %v", err)
	}
	// Expected sites: 1 block header (PageRVA), 2 DIR64 entries.
	// Padding Absolute entries contribute zero patches.
	if patched != 3 {
		t.Errorf("patched count = %d, want 3", patched)
	}

	// Block header PageRVA was 0x1000 → 0x11000.
	const blockFileOff = 0x410
	if got := binary.LittleEndian.Uint32(pe[blockFileOff:]); got != 0x11000 {
		t.Errorf("PageRVA = %#x, want 0x11000", got)
	}

	// DIR64 targets += delta.
	if got := binary.LittleEndian.Uint64(pe[0x500:]); got != baseVal+uint64(delta) {
		t.Errorf("DIR64 target[0] = %#x, want %#x", got, baseVal+uint64(delta))
	}
	if got := binary.LittleEndian.Uint64(pe[0x508:]); got != baseVal+uint64(delta) {
		t.Errorf("DIR64 target[1] = %#x, want %#x", got, baseVal+uint64(delta))
	}
}

// TestApplyRVAShiftAllDirectories_PropagatesRvaResolverError — the
// rvaToFile closure can fail (e.g. RVA outside any section); the
// helper must wrap that error rather than panic.
func TestApplyRVAShiftAllDirectories_PropagatesRvaResolverError(t *testing.T) {
	pe := peWithRelocs(t)
	const delta uint32 = 0x10000
	resolverErr := errResolverFail{}
	_, err := transform.ApplyRVAShiftAllDirectories(pe, pe, delta, func(uint32) (uint32, error) {
		return 0, resolverErr
	})
	if err == nil {
		t.Fatal("expected resolver error to propagate, got nil")
	}
}

type errResolverFail struct{}

func (errResolverFail) Error() string { return "resolver intentional failure" }
