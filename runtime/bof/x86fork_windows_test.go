//go:build windows && !bof_x86_loader

package bof

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalX86COFFHeader returns 20 bytes that pass parseCOFFHeader
// shape checks while declaring machine=0x014c (i386). No section
// table, no symbols — enough to exercise the early-return error
// paths in Load and Run without needing a real fixture on disk.
func minimalX86COFFHeader() []byte {
	b := make([]byte, coffHeaderSize)
	binary.LittleEndian.PutUint16(b[0:], 0x014c) // Machine = IMAGE_FILE_MACHINE_I386
	// NumberOfSections=0, the rest zero is fine for this code path.
	return b
}

// TestRun_X86COFF_RoutesToCrossArchError pins the slice 1.d.2
// phase A contract: an x86 COFF is detected, routed to the
// registered coffX86Loader, and surfaces ErrCrossArchX86Unsupported
// wrapped inside bof.Run's "load (coff-x86)" error frame. Operators
// reading the message must see both the kind label and the
// sentinel so they know which build tag to flip on once phase C
// lands.
func TestRun_X86COFF_RoutesToCrossArchError(t *testing.T) {
	res, err := Run(context.Background(), Spec{Bytes: minimalX86COFFHeader()})
	require.Error(t, err)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, ErrCrossArchX86Unsupported,
		"x86 .o must surface the cross-arch sentinel, not a generic 'unknown format' error")
	assert.Contains(t, err.Error(), "coff-x86",
		"error must name the detected kind so operators can grep it")
}

// TestRun_X86COFF_ExplicitMethod_RoutesSameWay confirms explicit
// Spec.Method=KindCOFFx86 lands on the same loader as auto-detection.
// Guards against future regressions where one path skips the registry
// or returns "no loader registered" instead of the rich sentinel.
func TestRun_X86COFF_ExplicitMethod_RoutesSameWay(t *testing.T) {
	res, err := Run(context.Background(), Spec{
		Bytes:  minimalX86COFFHeader(),
		Method: KindCOFFx86,
	})
	require.Error(t, err)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, ErrCrossArchX86Unsupported)
}

// TestLoad_X86COFF_ReturnsCrossArchSentinel pins the in-process
// Load entry point: an x86 .o must not be silently accepted, must
// not surface as the generic "unsupported COFF machine type"
// message (which leaks the raw hex code), and must wrap
// ErrCrossArchX86Unsupported so errors.Is dispatching works in
// caller code.
func TestLoad_X86COFF_ReturnsCrossArchSentinel(t *testing.T) {
	b, err := Load(minimalX86COFFHeader())
	require.Error(t, err)
	assert.Nil(t, b)
	assert.ErrorIs(t, err, ErrCrossArchX86Unsupported)
	assert.True(t, errors.Is(err, ErrCrossArchX86Unsupported),
		"Load must wrap the sentinel verbatim (errors.Is must succeed)")
}
