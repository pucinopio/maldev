//go:build windows

package bof

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRealWorldBOF_ExercisesFullSurface drives realworld_calls.o — a
// clean-room equivalent of a typical TrustedSec-SA-style enumeration
// BOF — through Run(ctx, Spec) to validate the slice 1 + 1.b + 1.c
// surface end-to-end against an externally-compiled COFF object.
//
// The BOF calls into:
//   - BeaconPrintf with multi-arg %s / %d / %x format strings (slice 1.b)
//   - BeaconOutput for raw bytes
//   - BeaconIsAdmin (slice 1)
//   - BeaconUseToken + BeaconRevertToken (slice 1)
//   - BeaconGetSpawnTo (slice 1.b — x86 flag fallback)
//   - BeaconAddValue + BeaconGetValue (slice 1)
//   - toWideChar (slice 1)
//   - KERNEL32$GetComputerNameA / GetCurrentProcessId / GetTickCount
//   - ADVAPI32$OpenProcessToken / KERNEL32$CloseHandle
//
// Asserts the canonical markers each callback path is expected to
// emit, plus the raw 0xDE 0xAD 0xBE 0xEF output channel survives
// untouched.
func TestRealWorldBOF_ExercisesFullSurface(t *testing.T) {
	bytes, err := os.ReadFile(filepath.Join("testdata", "realworld_calls.o"))
	require.NoError(t, err)

	res, err := Run(context.Background(), Spec{
		Bytes:   bytes,
		SpawnTo: `C:\Windows\System32\notepad.exe`,
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	out := string(res.Output)
	t.Logf("BOF output: %q", out)

	// Multi-arg printf line: host=<name> pid=<int> admin=<0|1> ticks=0x<hex>
	assert.Contains(t, out, "host=", "GetComputerNameA result must appear")
	assert.Contains(t, out, "pid=", "GetCurrentProcessId must appear")
	assert.Contains(t, out, "admin=", "BeaconIsAdmin must appear")
	assert.Contains(t, out, "ticks=0x", "GetTickCount must appear with hex prefix")

	// Token roundtrip success line.
	assert.Contains(t, out, "impersonate=ok",
		"BeaconUseToken + BeaconRevertToken must succeed against the current process token")

	// SpawnTo configured path appears.
	assert.Contains(t, out, "spawnto=", "BeaconGetSpawnTo must surface configured path")
	assert.Contains(t, out, "notepad.exe", "configured SpawnTo path must round-trip")

	// KV roundtrip — sentinel value preserved.
	assert.Contains(t, out, "kv=ok value=0xc0ffee",
		"BeaconAddValue + BeaconGetValue must preserve the sentinel through the per-Run map")

	// Wide-char conversion: returns char count (7 for "widearg") and
	// the wide-string is re-decoded via the printf %s wide-heuristic.
	assert.Contains(t, out, "widelen=7", "toWideChar must report 7 wide units for 'widearg'")
	assert.Contains(t, out, "wide=widearg", "wide-string %s heuristic must decode UTF-16 BOF arg")

	// Raw output channel — non-printable bytes survive.
	assert.True(t,
		strings.Contains(out, "\xde\xad\xbe\xef") ||
			strings.Contains(out, string([]byte{0xDE, 0xAD, 0xBE, 0xEF})),
		"BeaconOutput must pass raw bytes untouched")
}
