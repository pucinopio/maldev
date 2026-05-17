//go:build windows

package bof

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetectKind covers the magic-byte sniffer for every documented
// case: real COFF prefix → KindCOFF, anything else → KindUnknown,
// degenerate inputs (nil, len<2) → KindUnknown without panic.
func TestDetectKind(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want Kind
	}{
		{"nil", nil, KindUnknown},
		{"empty", []byte{}, KindUnknown},
		{"single byte", []byte{0x64}, KindUnknown},
		{"coff machine x64", []byte{0x64, 0x86, 0, 0}, KindCOFF},
		{"coff machine x86 (unsupported)", []byte{0x4c, 0x01, 0, 0}, KindUnknown},
		{"random gibberish", []byte("hello bytes!"), KindUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, DetectKind(c.in))
		})
	}
}

// TestKind_String round-trips each Kind into its slug. Locks the
// strings so the doc + log output stay stable.
func TestKind_String(t *testing.T) {
	assert.Equal(t, "coff", KindCOFF.String())
	assert.Equal(t, "gomod", KindGoModule.String())
	assert.Equal(t, "gof", KindGOF.String())
	assert.Equal(t, "unknown", KindUnknown.String())
	assert.Equal(t, "unknown", Kind(999).String())
}

// loadFixture is a thin helper for the Run tests below.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err, "read %s", name)
	return b
}

// TestRun_AutoDetectsCOFF passes a real COFF fixture without setting
// Spec.Method; Run must sniff the magic bytes, dispatch to coffLoader,
// and surface the BOF's BeaconPrintf output.
func TestRun_AutoDetectsCOFF(t *testing.T) {
	bytes := loadFixture(t, "hello_beacon.o")
	res, err := Run(context.Background(), Spec{Bytes: bytes})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.NotEmpty(t, res.Output, "BOF should have printed at least one byte")
}

// TestRun_ExplicitMethodCOFF skips magic-byte detection by setting
// Spec.Method=KindCOFF. Same fixture, same expected output.
func TestRun_ExplicitMethodCOFF(t *testing.T) {
	bytes := loadFixture(t, "hello_beacon.o")
	res, err := Run(context.Background(), Spec{Bytes: bytes, Method: KindCOFF})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Output)
}

// TestRun_UnknownMagicErrors covers the "no loader" path: random bytes
// produce a clean error, not a panic or silent zero-output result.
func TestRun_UnknownMagicErrors(t *testing.T) {
	res, err := Run(context.Background(), Spec{Bytes: []byte("not a coff blob")})
	assert.Error(t, err)
	assert.Nil(t, res)
}

// TestRun_UnregisteredKindErrors covers the explicit-method-but-no-
// loader path (slice 2 only has COFF registered; KindGOF must error
// loudly until slice 4 lands).
func TestRun_UnregisteredKindErrors(t *testing.T) {
	res, err := Run(context.Background(), Spec{Bytes: []byte("anything"), Method: KindGOF})
	assert.Error(t, err)
	assert.Nil(t, res)
}

// TestRun_AppliesSpawnTo confirms Spec.SpawnTo flows into the BOF via
// the SetSpawnTo setter. The error_spawnto fixture asks for the path
// via BeaconGetSpawnTo and writes it back, so we can assert the
// configured value reached the BOF.
func TestRun_AppliesSpawnTo(t *testing.T) {
	bytes := loadFixture(t, "error_spawnto.o")
	res, err := Run(context.Background(), Spec{
		Bytes:   bytes,
		SpawnTo: `C:\Windows\System32\notepad.exe`,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Contains(t, string(res.Output), `notepad.exe`,
		"BOF must echo the configured SpawnTo path")
}

// TestRun_AppliesUserData_NoFixture is a placeholder regression guard:
// SetUserData applies cleanly even when the BOF doesn't read the data
// (the assignment is plumbing). We assert no error rather than an
// output property, because no checked-in fixture calls
// BeaconGetCustomUserData. A dedicated fixture is queued in the
// loader revamp plan.
func TestRun_AppliesUserData_NoFixture(t *testing.T) {
	bytes := loadFixture(t, "hello_beacon.o")
	_, err := Run(context.Background(), Spec{
		Bytes:    bytes,
		UserData: []byte("custom-blob"),
	})
	assert.NoError(t, err)
}

// TestRun_NoLoaderFallsThroughKindError is the negative twin of
// TestRun_UnregisteredKindErrors via temporary registry mutation —
// guards against silently falling back to the default loader when an
// explicit Kind is set.
func TestRun_NoLoaderFallsThroughKindError(t *testing.T) {
	// Snapshot + restore the coff loader so this test doesn't pollute
	// the package-level registry.
	loaderRegistryMu.Lock()
	saved := loaderRegistry[KindCOFF]
	delete(loaderRegistry, KindCOFF)
	loaderRegistryMu.Unlock()
	t.Cleanup(func() {
		loaderRegistryMu.Lock()
		loaderRegistry[KindCOFF] = saved
		loaderRegistryMu.Unlock()
	})

	res, err := Run(context.Background(), Spec{
		Bytes:  []byte{0x64, 0x86, 0, 0}, // valid COFF magic
		Method: KindCOFF,
	})
	assert.Error(t, err)
	assert.Nil(t, res)
}
