//go:build windows && !pe_noconsolation

package pe

import (
	"errors"
	"testing"
)

// TestRunExecutable_LoaderMissing — under the default build (no
// pe_noconsolation tag), RunExecutable should return
// ErrLoaderMissing and propagate it cleanly, not panic on the nil
// blob. Gated by the inverse tag because with pe_noconsolation
// the loader IS embedded and the same call would dispatch into
// the BOF for real.
func TestRunExecutable_LoaderMissing(t *testing.T) {
	_, err := RunExecutable([]byte{0x4d, 0x5a}, Options{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrLoaderMissing) {
		t.Errorf("want ErrLoaderMissing, got %v", err)
	}
}
