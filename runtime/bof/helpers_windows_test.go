//go:build windows

package bof

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/maldev/evasion/stealthopen"
	"github.com/oioio-space/maldev/testutil"
)

// TestArgsFromStrings verifies the multi-string shortcut produces
// the same wire bytes as NewArgs + AddString in a loop. Each string
// must materialise as a 4-byte LE length prefix (len+1 including the
// trailing NUL) followed by the bytes + a NUL byte.
func TestArgsFromStrings(t *testing.T) {
	got := ArgsFromStrings("alpha", "bravo-bravo")

	// Manual reference: same shape NewArgs would produce.
	var want bytes.Buffer
	for _, s := range []string{"alpha", "bravo-bravo"} {
		var hdr [4]byte
		binary.LittleEndian.PutUint32(hdr[:], uint32(len(s)+1))
		want.Write(hdr[:])
		want.WriteString(s)
		want.WriteByte(0)
	}
	if !bytes.Equal(got, want.Bytes()) {
		t.Errorf("packed bytes differ\n got=% x\nwant=% x", got, want.Bytes())
	}

	// Empty call returns a 0-byte slice (NewArgs().Pack() returns
	// the zero-length output buffer — no header bytes for "no args").
	if empty := ArgsFromStrings(); len(empty) != 0 {
		t.Errorf("ArgsFromStrings() with no args: want empty, got %d bytes", len(empty))
	}
}

// TestRunFromBytes pins the one-shot Load + Execute + Close contract
// against the hello_beacon fixture: must succeed end-to-end, output
// non-empty, no leak (deferred Close runs internally).
func TestRunFromBytes(t *testing.T) {
	data := loadTestBOF(t, "hello_beacon.o")
	out, err := RunFromBytes(data, nil)
	if err != nil {
		t.Fatalf("RunFromBytes: %v", err)
	}
	if len(out) == 0 {
		t.Errorf("expected non-empty output, got 0 bytes")
	}
}

// TestRunFromBytes_InvalidCOFFErrors verifies the error path: bad
// input must surface a Load error rather than panic or silently
// no-op.
func TestRunFromBytes_InvalidCOFFErrors(t *testing.T) {
	_, err := RunFromBytes([]byte{0xDE, 0xAD, 0xBE, 0xEF}, nil)
	if err == nil {
		t.Fatal("expected Load error on garbage input")
	}
}

// TestRunFromFile exercises the disk read path with a known-good
// fixture. Skips cleanly when the fixture is absent (fresh clone
// before mingw build) — same gating as the other lifecycle tests.
func TestRunFromFile(t *testing.T) {
	path := filepath.Join("testdata", "hello_beacon.o")
	// Skip rather than fail when the fixture isn't built — keeps
	// the suite green on a fresh clone.
	if data := loadTestBOF(t, "hello_beacon.o"); len(data) == 0 {
		t.Skip("hello_beacon.o not available")
	}
	// nil opener → stealthopen.OpenRead falls back to os.Open.
	out, err := RunFromFile(nil, path, nil)
	if err != nil {
		t.Fatalf("RunFromFile %s: %v", path, err)
	}
	if len(out) == 0 {
		t.Errorf("expected non-empty output")
	}
}

// TestRunFromFile_WithSpyOpener verifies the Opener parameter is
// honoured: an operator-supplied opener routes the read through its
// own path. Using testutil.SpyOpener — which records every Open and
// delegates to os.Open — is the cheapest way to prove the wiring
// without requiring a real stealth backend.
func TestRunFromFile_WithSpyOpener(t *testing.T) {
	path := filepath.Join("testdata", "hello_beacon.o")
	if data := loadTestBOF(t, "hello_beacon.o"); len(data) == 0 {
		t.Skip("hello_beacon.o not available")
	}
	spy := &testutil.SpyOpener{}
	out, err := RunFromFile(spy, path, nil)
	if err != nil {
		t.Fatalf("RunFromFile %s: %v", path, err)
	}
	if len(out) == 0 {
		t.Errorf("expected non-empty output")
	}
	paths := spy.Paths()
	if len(paths) != 1 || paths[0] != path {
		t.Errorf("opener calls = %v, want [%q]", paths, path)
	}
}

// TestRunFromFile_OpenerMatrix sweeps every stealthopen.Opener
// implementation the repo currently ships and asserts each one
// successfully reads + runs the BOF end-to-end through RunFromFile.
//
// One sub-test per opener — that's the "test everything one by one"
// contract: a regression in the new MultiStealth path doesn't get
// hidden by a green nil-opener case.
//
//   - nil          → stealthopen.OpenRead falls back to os.Open
//   - &Standard{}  → explicit os.Open
//   - *Stealth     → NTFS Object-ID captured at construction; Open
//                    ignores the path arg
//   - *MultiStealth → per-path Object-ID cache; first call pays the
//                    path-based-hook cost, subsequent calls don't
func TestRunFromFile_OpenerMatrix(t *testing.T) {
	path := filepath.Join("testdata", "hello_beacon.o")
	if data := loadTestBOF(t, "hello_beacon.o"); len(data) == 0 {
		t.Skip("hello_beacon.o not available")
	}

	// Build a *Stealth ahead of time so a NewStealth failure (file
	// not on NTFS, FSCTL_GET_OBJECT_ID denied) skips just the
	// Stealth sub-test rather than failing the whole table.
	stealth, stealthErr := stealthopen.NewStealth(path)

	for _, tc := range []struct {
		name   string
		opener stealthopen.Opener
		skip   error
	}{
		{"nil_default_fallback", nil, nil},
		{"explicit_Standard", &stealthopen.Standard{}, nil},
		{"Stealth_object_id", stealth, stealthErr},
		{"MultiStealth_cached", &stealthopen.MultiStealth{}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip != nil {
				t.Skipf("opener unavailable: %v", tc.skip)
			}
			out, err := RunFromFile(tc.opener, path, nil)
			if err != nil {
				t.Fatalf("RunFromFile: %v", err)
			}
			if len(out) == 0 {
				t.Errorf("expected non-empty output")
			}
		})
	}
}

// TestRunFromFile_MissingPathErrors verifies the read-side error
// is propagated with the offending path in the message.
func TestRunFromFile_MissingPathErrors(t *testing.T) {
	_, err := RunFromFile(nil, "does/not/exist.o", nil)
	if err == nil {
		t.Fatal("expected error on missing path")
	}
	if !strings.Contains(err.Error(), "does/not/exist.o") &&
		!strings.Contains(err.Error(), "does\\not\\exist.o") {
		t.Errorf("error %q should mention the offending path", err)
	}
}

// TestRunSafe verifies the sacrificial wrapper succeeds on a normal
// BOF — i.e. the SetSacrificialThread call is properly threaded
// through the helper and the entry runs to completion under the
// dedicated thread with VEH-mediated isolation.
func TestRunSafe(t *testing.T) {
	data := loadTestBOF(t, "hello_beacon.o")
	out, err := RunSafe(data, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("RunSafe: %v", err)
	}
	if len(out) == 0 {
		t.Errorf("expected non-empty output")
	}
}
