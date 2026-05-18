//go:build windows && bof_x86_loader

package bof

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestX86Loader_Embedded_NotEmpty confirms the bof_x86_loader
// build-tag actually linked the DLL bytes in. A regression here
// (empty embed slice) would silently fall back to
// ErrCrossArchX86Unsupported on every Execute attempt.
func TestX86Loader_Embedded_NotEmpty(t *testing.T) {
	dll, err := loadX86LoaderDLL()
	require.NoError(t, err)
	require.NotEmpty(t, dll, "embed slot must contain the loader DLL bytes")

	// Quick PE32 magic sniff so the bytes look like a real DLL,
	// not e.g. an accidentally-embedded text file.
	require.GreaterOrEqual(t, len(dll), 0x40, "DLL too small to be a real PE")
	assert.Equal(t, byte('M'), dll[0])
	assert.Equal(t, byte('Z'), dll[1])
}

// TestX86Loader_Load_ReturnsRunnable pins the contract: with the
// embed tag on, coffX86Loader.Load returns a non-nil Runnable
// instead of the sentinel. The Runnable is later exercised by the
// VM-only E2E test that actually spawns rundll32.
func TestX86Loader_Load_ReturnsRunnable(t *testing.T) {
	loader := coffX86Loader{}
	bofBytes := minimalX86COFFHeader_present()

	r, err := loader.Load(bofBytes)
	require.NoError(t, err)
	require.NotNil(t, r)
}

// TestClassifyX86Exit covers every known mapping in the
// exit-code-to-error switch. Ensures a future enum addition in
// abi.h that lands without a Go-side update at least surfaces as
// "unknown code" rather than silently being interpreted as DONE.
func TestClassifyX86Exit(t *testing.T) {
	cases := []struct {
		code      uint32
		wantNil   bool
		wantMatch string
	}{
		{x86ExitDone, true, ""},
		{x86ExitBadProtocol, false, "BOF_EXIT_BAD_PROTOCOL"},
		{x86ExitBadVersion, false, "BOF_EXIT_BAD_VERSION"},
		{x86ExitOpenBOFFailed, false, "read the BOF .o file"},
		{x86ExitOpenArgsFailed, false, "read the args file"},
		{x86ExitOpenOutFailed, false, "write the output file"},
		{x86ExitOpenErrFailed, false, "write the error file"},
		{x86ExitLoadFailed, false, "BOF_EXIT_LOAD_FAILED"},
		{x86ExitBOFCrashed, false, "BOF_EXIT_BOF_CRASHED"},
		{0xDEAD, false, "0xDEAD"},
	}
	for _, c := range cases {
		t.Run(c.wantMatch, func(t *testing.T) {
			err := classifyX86Exit(c.code)
			if c.wantNil {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantMatch)
		})
	}
}

// TestRandSuffix_Distinct guards against the crypto/rand fallback
// degrading to a constant value (which would collide concurrent
// helpers and corrupt their temp dirs). 32 calls in a row should
// never produce a duplicate.
func TestRandSuffix_Distinct(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 32; i++ {
		s := randSuffix()
		assert.NotEmpty(t, s)
		_, dup := seen[s]
		assert.False(t, dup, "duplicate randSuffix value: %s", s)
		seen[s] = struct{}{}
	}
}

// TestX86BOF_Execute_EndToEnd is the VM-side smoke test. It runs
// the orchestrator against the actual SysWOW64\rundll32.exe + the
// embedded loader skeleton, with a tiny fake .o byte stream (since
// the skeleton DOESN'T parse COFF yet — phase C step 1).
//
// The skeleton's contract is: read the bof file, write the banner
// "[x86 BOF loader skeleton — phase C step 0]\nbof_len=N args_len=M\n"
// to the output file, exit BOF_EXIT_DONE. So we feed it a
// 32-byte fake .o, expect a non-empty output containing "bof_len=32",
// and verify the orchestrator round-trips correctly.
//
// Gated behind MALDEV_X86_RUNDLL32=1 so host-side `go test`
// without the WoW64 layer (Wine, fresh Win ARM64 etc.) doesn't
// fail on the spawn. The VM driver sets the env var.
func TestX86BOF_Execute_EndToEnd(t *testing.T) {
	if os.Getenv("MALDEV_X86_RUNDLL32") != "1" {
		t.Skip("set MALDEV_X86_RUNDLL32=1 to exercise the SysWOW64\\rundll32 path")
	}
	if _, err := os.Stat(defaultX86Host); err != nil {
		t.Skipf("WoW64 host %s missing: %v", defaultX86Host, err)
	}

	fakeBOF := make([]byte, 32)
	for i := range fakeBOF {
		fakeBOF[i] = byte(i)
	}

	res, err := Run(context.Background(), Spec{
		Bytes: append([]byte{0x4c, 0x01}, fakeBOF[2:]...), // i386 magic + filler
		Args:  []byte("hello"),
	})

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Contains(t, string(res.Output), "phase C step 0")
	assert.Contains(t, string(res.Output), "bof_len=32")
	assert.Contains(t, string(res.Output), "args_len=5")
}

// TestX86BOF_Execute_TimeoutKillsHost pins the timeout behaviour:
// when rundll32 hangs (here simulated by setting an absurdly short
// 1ms timeout against a real spawn), the orchestrator surfaces a
// "rundll32 timeout" error and the helper is killed.
//
// Also gated on MALDEV_X86_RUNDLL32 — needs the WoW64 layer.
func TestX86BOF_Execute_TimeoutKillsHost(t *testing.T) {
	if os.Getenv("MALDEV_X86_RUNDLL32") != "1" {
		t.Skip("set MALDEV_X86_RUNDLL32=1 to exercise the SysWOW64\\rundll32 path")
	}

	r, err := coffX86Loader{}.Load(minimalX86COFFHeader_present())
	require.NoError(t, err)
	x := r.(*x86BOF)
	x.SetTimeout(1 * time.Millisecond)

	_, err = x.Execute(nil)
	require.Error(t, err)
	assert.True(t,
		strings.HasPrefix(err.Error(), "runtime/bof/x86: rundll32 timeout"),
		"want a timeout error, got: %v", err)
}

// minimalX86COFFHeader_present mirrors the helper in the
// !bof_x86_loader test file. Duplicated rather than shared because
// the two test files are mutually exclusive via build tags.
func minimalX86COFFHeader_present() []byte {
	b := make([]byte, 32)
	b[0] = 0x4c
	b[1] = 0x01
	return b
}
