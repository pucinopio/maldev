//go:build windows && bof_x86_loader

package bof

import (
	"context"
	"encoding/binary"
	"os"
	"testing"
	"time"

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
		{"VirtualAlloc", 0x91AFCA54},
		{"VirtualProtect", 0x7946C61B},
		{"GetProcessHeap", 0xA80EECAE},
		{"HeapAlloc", 0x2500383C},
		{"HeapFree", 0x10C32616},
		{"RtlMoveMemory", 0xCF14E85B},
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

// TestX86BOF_Execute_NoopFixture is the cross-process E2E for
// slice 1.d step 1.b: the loader's COFF parser + relocation
// engine + entry call dispatch are exercised against a real
// (but trivial) i386 BOF that has no Beacon API imports and no
// cross-section relocations.
//
// The fixture testdata/noop.x86.o is `void go(char*, int) {}`
// compiled with i686-w64-mingw32-gcc — 16 bytes of .text plus
// empty .data/.bss. A passing test means the shellcode found
// the entry symbol "_go", called it cdecl-correctly, and
// returned to ExitThread without crashing.
//
// Skipped on hosts without SysWOW64\rundll32.exe (non-Windows
// or Windows-on-ARM without WoW64).
func TestX86BOF_Execute_NoopFixture(t *testing.T) {
	if _, err := os.Stat(defaultX86Host); err != nil {
		t.Skipf("WoW64 host %s missing: %v", defaultX86Host, err)
	}
	bof, err := os.ReadFile("testdata/noop.x86.o")
	require.NoError(t, err, "noop.x86.o fixture missing")

	res, err := Run(context.Background(), Spec{
		Bytes: bof,
		Args:  []byte{},
	})
	require.NoError(t, err, "expected loader to surface LOADER_STATUS_DONE")
	require.NotNil(t, res)
	assert.Empty(t, res.Output, "noop fixture writes no output")
	assert.Empty(t, res.Errors, "noop fixture writes no errors")
}

// TestX86BOF_Execute_Timeout exercises the WaitForSingleObject
// timeout path. A 1ms wait against the real cross-process spawn
// is essentially guaranteed to fire — the orchestrator must kill
// the host and surface a "loader timeout" error.
func TestX86BOF_Execute_Timeout(t *testing.T) {
	if _, err := os.Stat(defaultX86Host); err != nil {
		t.Skipf("WoW64 host %s missing: %v", defaultX86Host, err)
	}
	bof, err := os.ReadFile("testdata/noop.x86.o")
	require.NoError(t, err)
	r, err := coffX86Loader{}.Load(bof)
	require.NoError(t, err)
	x := r.(*x86BOF)
	x.SetTimeout(1 * time.Millisecond)
	_, err = x.Execute(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loader timeout")
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
