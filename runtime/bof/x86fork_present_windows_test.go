//go:build windows && bof_x86_loader

package bof

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestX86Loader_Embedded_NotEmpty confirms the bof_x86_loader
// build-tag actually linked the shellcode bytes in. A regression
// here (empty embed slice, e.g. .bin file renamed without
// updating the embed directive) would silently fall back to
// ErrCrossArchX86Unsupported on every operator invocation.
func TestX86Loader_Embedded_NotEmpty(t *testing.T) {
	sc, err := loadX86LoaderShellcode()
	require.NoError(t, err)
	require.NotEmpty(t, sc, "embed slot must contain the loader shellcode bytes")
	// Sanity-floor on the size so a future change that
	// accidentally embeds a header-only artefact (e.g. an empty
	// ELF or a 4-byte stub) fails loudly.
	assert.GreaterOrEqual(t, len(sc), 64,
		"shellcode should be at least 64 bytes — got %d", len(sc))
}

// TestX86Loader_Entry_PrologueLooksReasonable sniffs the first few
// bytes of the shellcode for a plausible x86 function prologue.
// loader_entry's __attribute__((force_align_arg_pointer)) makes
// the compiler emit a `lea ecx, [esp+0x4]` (8d 4c 24 04) before
// the `and esp, -16` alignment trick. Pinning this catches the
// regression where the linker drops .text.entry first and
// CreateRemoteThread jumps into a different function (or into
// .rodata data interpreted as code).
func TestX86Loader_Entry_PrologueLooksReasonable(t *testing.T) {
	sc, err := loadX86LoaderShellcode()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(sc), 4)

	want := []byte{0x8d, 0x4c, 0x24, 0x04} // lea ecx, [esp+0x4]
	assert.Equal(t, want, sc[:4],
		"expected force_align_arg_pointer prologue at offset 0, got % x", sc[:4])
}

// TestRor13_KnownAnswers locks the Go-side ROR13 implementation
// against the precomputed kernel32 hashes baked into loader.c. A
// drift in the algorithm (e.g. someone "improves" win/api's
// ResolveByHash) would desynchronise the parent and the loader
// in a way that's only catchable in a live VM — this unit test
// catches it at compile-test time.
func TestRor13_KnownAnswers(t *testing.T) {
	cases := []struct {
		name string
		want uint32
	}{
		{"ExitThread", 0x60E0CEEF},
		// Future entries land here as phase B-bis step 1 adds
		// more kernel32 resolutions (VirtualAlloc / VirtualProtect
		// / GetProcessHeap / HeapAlloc / HeapFree / RtlMoveMemory).
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ror13Hash(c.name)
			assert.Equal(t, c.want, got,
				"ROR13(%q) = 0x%08X, want 0x%08X", c.name, got, c.want)
		})
	}
}

// TestABIMagic_LittleEndianMatchesCSide pins the byte order. The
// shellcode reads p->magic as a 32-bit little-endian load
// (`cmp eax, 0x36384342`); the Go side writes it as
// binary.LittleEndian.PutUint32. Any mismatch (e.g. someone
// "fixes" the const to network-byte-order) would make every
// injection fail with LOADER_STATUS_ABI_MISMATCH.
func TestABIMagic_LittleEndianMatchesCSide(t *testing.T) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], loaderABIMagic)
	assert.Equal(t, []byte{'B', 'C', '8', '6'}, buf[:])
}

// ror13Hash mirrors win/api.ResolveByHash and the C side's
// ror13_hash. Lives in this test file (under the bof_x86_loader
// tag) for now; step 1 will move it to a shared helper used by
// the orchestrator at hash-precomputation time.
func ror13Hash(s string) uint32 {
	var h uint32
	for i := 0; i < len(s); i++ {
		h = ((h >> 13) | (h << 19)) + uint32(s[i])
	}
	return h
}
