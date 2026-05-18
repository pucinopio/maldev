//go:build windows

package bof

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadLifecycleBOF reads a testdata .o by basename, skipping the
// test cleanly if the fixture is missing (e.g. fresh clone before
// mingw build). Keeps the lifecycle suite self-contained — we
// can't import loadExampleBOF from example_bofs_windows_test.go
// without pulling the testutil.RequireIntrusive gate they use.
func loadLifecycleBOF(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("%s missing: %v (build per testdata/README.md)", path, err)
	}
	return data
}

// TestBOF_Close_Idempotent verifies Close() is safe to call any
// number of times and returns nil after the first successful free.
// Idempotency matters for callers using defer Close() patterns
// where the BOF might already have been closed by Run-style
// dispatch helpers that wrap their own cleanup.
func TestBOF_Close_Idempotent(t *testing.T) {
	b, err := Load(loadLifecycleBOF(t, "hello_beacon.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := b.Execute(nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Errorf("third Close: %v", err)
	}
}

// TestBOF_ExecuteAfterClose proves Execute returns a clean error
// (no panic, no segfault) after Close. The mapped memory is gone;
// running anyway would be a use-after-free.
func TestBOF_ExecuteAfterClose(t *testing.T) {
	b, err := Load(loadLifecycleBOF(t, "hello_beacon.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := b.Execute(nil); err != nil {
		t.Fatalf("initial Execute: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, err = b.Execute(nil)
	if err == nil {
		t.Fatal("Execute after Close should return an error")
	}
	if !strings.Contains(err.Error(), "closed BOF") {
		t.Errorf("error message should mention 'closed BOF', got %q", err)
	}
}

// TestBOF_ExecuteTwice_Default verifies that a BOF can be
// Executed multiple times — the prepare-once design must keep
// the entry address + import table valid across calls. The
// hello_beacon fixture prints a fixed greeting; both calls
// should produce identical output.
func TestBOF_ExecuteTwice_Default(t *testing.T) {
	b, err := Load(loadLifecycleBOF(t, "hello_beacon.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer b.Close()

	first, err := b.Execute(nil)
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	second, err := b.Execute(nil)
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("stateless BOF should produce identical output across calls\nfirst:  %q\nsecond: %q",
			first, second)
	}
	if !strings.Contains(string(first), "hello") {
		t.Errorf("first output should contain 'hello', got %q", first)
	}
}

// TestBOF_SetPersistent_StatelessByDefault is the documenting
// witness: without SetPersistent(true), writable sections are
// restored between Execute calls so a stateless BOF observes
// fresh memory. With our current test corpus we don't have a
// fixture that READS its own .data across calls; the test
// instead asserts the API doesn't crash + the default behaviour
// is observable via successive identical outputs (already
// covered by TestBOF_ExecuteTwice_Default).
//
// Pinning the default-is-false contract here makes future toggles
// of the field default louder than a silent behaviour change.
func TestBOF_SetPersistent_StatelessByDefault(t *testing.T) {
	b, err := Load(loadLifecycleBOF(t, "hello_beacon.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer b.Close()
	if b.persistent {
		t.Error("BOF.persistent default should be false")
	}
	b.SetPersistent(true)
	if !b.persistent {
		t.Error("SetPersistent(true) must flip the flag")
	}
	b.SetPersistent(false)
	if b.persistent {
		t.Error("SetPersistent(false) must clear the flag")
	}
}
