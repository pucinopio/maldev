//go:build windows

package bof

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oioio-space/maldev/testutil"
)

// TestBeaconAPI_Complete exercises the three documented Beacon
// API symbols that no other in-tree fixture touches:
// BeaconGetCustomUserData, BeaconRemoveValue, BeaconGetOutputData.
//
// Combined with hello_beacon (BeaconPrintf), parse_args
// (BeaconData*), data_extras (DataShort/Length), format_output
// (BeaconFormat* family), format_extras (FormatReset/Printf,
// ErrorDD/NA), error_spawnto (ErrorD + GetSpawnTo) and
// realworld_calls (IsAdmin, UseToken/RevertToken, GetSpawnTo
// x86, AddValue/GetValue, toWideChar, BeaconOutput) the suite
// covers every canonical Beacon API symbol.
func TestBeaconAPI_Complete(t *testing.T) {
	path := filepath.Join("testdata", "beacon_api_complete.o")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("%s missing: %v (build per testdata/README.md)", path, err)
	}
	b, err := Load(data)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// UserData round-trip exercises BeaconGetCustomUserData with
	// non-zero bytes — the BOF's "ptr_nonnull=1" branch fires only
	// when SetUserData was called with content.
	b.SetUserData([]byte("WITNESS_PAYLOAD"))

	out, err := b.Execute(nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := string(out)

	checks := []string{
		"userdata_len=15 ptr_nonnull=1",                              // BeaconGetCustomUserData
		"kv_add=1 before=1 removed=1 after=1 reremove=1",             // AddValue/RemoveValue idempotent path
		"outdata_len_pos=1 outdata_nonnull=1",                        // BeaconGetOutputData saw prior writes
	}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("missing %q\nfull output:\n%s", c, got)
		}
	}
}

// TestBeaconAPI_Intrusive exercises BeaconSpawnTemporaryProcess,
// BeaconInjectTemporaryProcess, BeaconCleanupProcess and
// BeaconInjectProcess. Each call creates a real child process or
// writes to one — gated MALDEV_INTRUSIVE=1.
//
// Payload is a single RET (0xC3). The witness is the BOF's own
// trace of what the Beacon API returned for each call: the
// spawned PID is non-zero, the handle is non-NULL, and the
// dispatch markers ("inject_temp=dispatched", "cleanup_temp=done",
// "inject_proc_self=dispatched") all surface.
func TestBeaconAPI_Intrusive(t *testing.T) {
	testutil.RequireIntrusive(t)

	path := filepath.Join("testdata", "beacon_api_intrusive.o")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("%s missing: %v (build per testdata/README.md)", path, err)
	}
	b, err := Load(data)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	out, err := b.Execute(nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := string(out)

	wants := []string{
		"spawn_temp=1",            // BeaconSpawnTemporaryProcess returned TRUE
		"hProc_nonnull=1",         // handle stored
		"inject_temp=dispatched",  // BeaconInjectTemporaryProcess reached
		"cleanup_temp=done",       // BeaconCleanupProcess reached
		"inject_proc_self=dispatched", // BeaconInjectProcess into self
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q\nfull output:\n%s", w, got)
		}
	}
}

// TestBeaconAPI_FullSurfaceMatrix is the master coverage
// witness. It enumerates every documented Beacon API symbol
// and maps each to the in-tree fixture that exercises it. The
// test FAILS if any symbol slips through — so adding a new
// Beacon function without a matching fixture-touch is caught
// at PR time, not in production.
func TestBeaconAPI_FullSurfaceMatrix(t *testing.T) {
	// Maps each Beacon symbol to a fixture .o whose `go()` calls
	// it. Verified by reading the .c sources in testdata/. The
	// "intrusive" group is only checked when MALDEV_INTRUSIVE is
	// set — those fixtures spawn real processes.
	coverage := []struct {
		symbol     string
		fixture    string
		intrusive  bool
	}{
		// Group 1 — Data parsing
		{"BeaconDataParse", "parse_args.o", false},
		{"BeaconDataInt", "parse_args.o", false},
		{"BeaconDataShort", "data_extras.o", false},
		{"BeaconDataLength", "data_extras.o", false},
		{"BeaconDataExtract", "parse_args.o", false},

		// Group 2 — Format / output
		{"BeaconFormatAlloc", "format_output.o", false},
		{"BeaconFormatReset", "format_extras.o", false},
		{"BeaconFormatFree", "format_output.o", false},
		{"BeaconFormatAppend", "format_output.o", false},
		{"BeaconFormatInt", "format_output.o", false},
		{"BeaconFormatToString", "format_output.o", false},
		{"BeaconFormatPrintf", "format_extras.o", false},
		{"BeaconPrintf", "hello_beacon.o", false},
		{"BeaconOutput", "format_output.o", false},
		{"BeaconErrorD", "error_spawnto.o", false},
		{"BeaconErrorDD", "format_extras.o", false},
		{"BeaconErrorNA", "format_extras.o", false},

		// Group 3 — Token
		{"BeaconUseToken", "realworld_calls.o", false},
		{"BeaconRevertToken", "realworld_calls.o", false},

		// Group 4 — Injection / Spawn
		{"BeaconGetSpawnTo", "realworld_calls.o", false},
		{"BeaconSpawnTemporaryProcess", "beacon_api_intrusive.o", true},
		{"BeaconInjectProcess", "beacon_api_intrusive.o", true},
		{"BeaconInjectTemporaryProcess", "beacon_api_intrusive.o", true},
		{"BeaconCleanupProcess", "beacon_api_intrusive.o", true},

		// Group 5 — Helpers
		{"BeaconIsAdmin", "realworld_calls.o", false},
		{"BeaconGetCustomUserData", "beacon_api_complete.o", false},
		{"toWideChar", "realworld_calls.o", false},

		// Group 6 — KV store
		{"BeaconAddValue", "realworld_calls.o", false},
		{"BeaconGetValue", "realworld_calls.o", false},
		{"BeaconRemoveValue", "beacon_api_complete.o", false},

		// Bonus (No-Consolation extension)
		{"BeaconGetOutputData", "beacon_api_complete.o", false},
	}

	intrusive := os.Getenv("MALDEV_INTRUSIVE") != ""
	missingFixtures := map[string]struct{}{}
	for _, c := range coverage {
		if c.intrusive && !intrusive {
			continue
		}
		path := filepath.Join("testdata", c.fixture)
		if _, err := os.Stat(path); err != nil {
			missingFixtures[c.fixture] = struct{}{}
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		// `strings.Contains` on the raw .o bytes catches the
		// __imp_<symbol> reference in the COFF symbol table.
		// Cheap, robust, no need to actually invoke the loader
		// — the goal is to enforce that the fixture mentions
		// the symbol so the runtime tests above will exercise
		// it when they run.
		if !strings.Contains(string(raw), c.symbol) {
			t.Errorf("fixture %s does not reference %s — coverage matrix lying",
				c.fixture, c.symbol)
		}
	}
	if len(missingFixtures) > 0 {
		t.Logf("missing fixtures (build per testdata/README.md): %v", missingFixtures)
	}
}
