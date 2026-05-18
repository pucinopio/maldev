//go:build windows && bof_x86_loader

package bof

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildIOBuffer_LayoutAndCopy pins the ioOffsets layout. The
// shellcode reads sub-region addresses via the params block; if
// the offsets drift (e.g. someone reorders args before bof) the
// loader will read the wrong bytes and the BOF will misparse its
// own .o. Verifies content + offsets in a single pass.
func TestBuildIOBuffer_LayoutAndCopy(t *testing.T) {
	bof := []byte{0xAA, 0xBB, 0xCC}
	args := []byte{0x01, 0x02}
	ud := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	spawn := []byte("C:\\W\x00")
	const outCap = 128
	const errCap = 64

	buf, off := buildIOBuffer(bof, args, ud, spawn, outCap, errCap)

	// Offsets must accumulate.
	assert.Equal(t, uint32(0), off.bof)
	assert.Equal(t, uint32(3), off.args)
	assert.Equal(t, uint32(5), off.userData)
	assert.Equal(t, uint32(9), off.spawnTo)
	assert.Equal(t, uint32(9+len(spawn)), off.out)
	assert.Equal(t, uint32(9+len(spawn))+outCap, off.err)

	// Content at each sub-region.
	assert.Equal(t, bof, buf[off.bof:off.bof+uint32(len(bof))])
	assert.Equal(t, args, buf[off.args:off.args+uint32(len(args))])
	assert.Equal(t, ud, buf[off.userData:off.userData+uint32(len(ud))])
	assert.Equal(t, spawn, buf[off.spawnTo:off.spawnTo+uint32(len(spawn))])

	// Output / error buffers are zeroed.
	for i := off.out; i < off.out+outCap; i++ {
		require.Equal(t, byte(0), buf[i], "out buffer must be zeroed at %d", i)
	}
	for i := off.err; i < off.err+errCap; i++ {
		require.Equal(t, byte(0), buf[i], "err buffer must be zeroed at %d", i)
	}

	// Total size matches.
	want := uint32(len(bof)+len(args)+len(ud)+len(spawn)) + outCap + errCap
	assert.Equal(t, want, uint32(len(buf)))
}

// TestBuildParamsBlock_FieldOffsets pins every byte offset that
// the C-side loader reads. A wire format drift would surface as
// LOADER_STATUS_ABI_MISMATCH (if magic shifts) or as a silent
// bad-pointer dereference (if any of the *_addr fields shift) —
// the latter is hard to debug in a VM, so we lock the layout
// here at unit-test time.
func TestBuildParamsBlock_FieldOffsets(t *testing.T) {
	const (
		bofA, bofL   = 0x10000000, 0x100
		argsA, argsL = 0x20000000, 0x200
		udA, udL     = 0x30000000, 0x300
		stA          = 0x40000000
		outA, outC   = 0x50000000, 0x500
		errA, errC   = 0x60000000, 0x600
	)
	buf := buildParamsBlock(
		bofA, bofL, argsA, argsL, udA, udL, stA,
		outA, outC, errA, errC, true,
	)
	require.Len(t, buf, loaderParamsSize)

	get := func(off int) uint32 { return binary.LittleEndian.Uint32(buf[off:]) }

	assert.Equal(t, loaderABIMagic, get(0), "magic")
	assert.Equal(t, loaderABIVersion, get(4), "version")
	assert.Equal(t, loaderStatusPending, get(8), "status")
	assert.Equal(t, uint32(0), get(12), "error_code")
	assert.Equal(t, uint32(bofA), get(16), "bof_addr")
	assert.Equal(t, uint32(bofL), get(20), "bof_len")
	assert.Equal(t, uint32(argsA), get(24), "args_addr")
	assert.Equal(t, uint32(argsL), get(28), "args_len")
	assert.Equal(t, uint32(udA), get(32), "user_data_addr")
	assert.Equal(t, uint32(udL), get(36), "user_data_len")
	assert.Equal(t, uint32(stA), get(40), "spawn_to_addr")
	assert.Equal(t, uint32(outA), get(44), "out_addr")
	assert.Equal(t, uint32(outC), get(48), "out_cap")
	assert.Equal(t, uint32(0), get(52), "out_len")
	assert.Equal(t, uint32(errA), get(56), "err_addr")
	assert.Equal(t, uint32(errC), get(60), "err_cap")
	assert.Equal(t, uint32(0), get(64), "err_len")
}

// TestBuildParamsBlock_NoSpawnTo_ZerosTheField verifies the
// hasSpawnTo flag correctly suppresses the spawn_to_addr write.
// The loader checks 0 = "no override" so a stale spawn-to write
// would make BOFs that consult BeaconGetSpawnTo read whatever
// happened to be at offset 40 (likely a parent stack address —
// catastrophic).
func TestBuildParamsBlock_NoSpawnTo_ZerosTheField(t *testing.T) {
	buf := buildParamsBlock(
		0x1000, 1, 0x2000, 2, 0x3000, 3, 0xDEADBEEF, // would-be spawn addr
		0x4000, 100, 0x5000, 50, false,
	)
	got := binary.LittleEndian.Uint32(buf[40:])
	assert.Equal(t, uint32(0), got, "spawn_to_addr must be 0 when hasSpawnTo=false")
}

// TestClassifyLoaderStatus covers every documented status → error
// mapping. Pins both the nil-on-DONE contract and the message
// substrings callers might grep.
func TestClassifyLoaderStatus(t *testing.T) {
	cases := []struct {
		status, code uint32
		wantNil      bool
		wantSubstr   string
	}{
		{loaderStatusDone, 0, true, ""},
		{loaderStatusPending, 0, false, "PENDING"},
		{loaderStatusRunning, 0, false, "RUNNING"},
		{loaderStatusABIMismatch, 0xDEAD, false, "ABI_MISMATCH"},
		{loaderStatusResolveFail, 0, false, "kernel32"},
		{loaderStatusLoadFail, 0xC0FF, false, "COFF load"},
		{loaderStatusBOFCrashed, 0xC0000005, false, "SEH=0xC0000005"},
		{0xFFFF, 0xAA, false, "unknown loader status"},
	}
	for _, c := range cases {
		t.Run(c.wantSubstr, func(t *testing.T) {
			err := classifyLoaderStatus(c.status, c.code)
			if c.wantNil {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantSubstr)
		})
	}
}
