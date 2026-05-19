//go:build windows

package bof

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/oioio-space/maldev/testutil"
)

// loadTestBOF reads a testdata .o by basename, skipping cleanly if
// the fixture is missing (e.g. fresh clone before mingw build).
// Accepts testing.TB so both tests (T) and benchmarks (B) share one
// helper. Lives here (not in a dedicated _test.go) so other suites in
// this package — bench, lifecycle, sacrificial — pick it up via the
// package-internal scope without importing testutil.RequireIntrusive.
func loadTestBOF(tb testing.TB, name string) []byte {
	tb.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Skipf("%s missing: %v (build per testdata/README.md)", path, err)
	}
	return data
}

// loadLifecycleBOF preserves the historical name used across this
// file's tests. Forwarder to loadTestBOF.
func loadLifecycleBOF(t *testing.T, name string) []byte {
	t.Helper()
	return loadTestBOF(t, name)
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
	if err := b.SetPersistent(true); err != nil {
		t.Fatalf("SetPersistent(true) before Execute should succeed: %v", err)
	}
	if !b.persistent {
		t.Error("SetPersistent(true) must flip the flag")
	}
	if err := b.SetPersistent(false); err != nil {
		t.Fatalf("SetPersistent(false) before Execute should succeed: %v", err)
	}
	if b.persistent {
		t.Error("SetPersistent(false) must clear the flag")
	}
}

// TestBOF_SetPersistent_AfterPrepareErrors pins the
// ErrAlreadyPrepared contract: once prepare() ran (via the first
// Execute), the writable-section snapshots are fixed and toggling
// persistence would only affect future restores. Returning an
// error rather than silently no-op'ing makes the caller's
// mistake loud.
func TestBOF_SetPersistent_AfterPrepareErrors(t *testing.T) {
	b, err := Load(loadLifecycleBOF(t, "hello_beacon.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer b.Close()
	if _, err := b.Execute(nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if err := b.SetPersistent(true); err != ErrAlreadyPrepared {
		t.Errorf("SetPersistent after Execute: want ErrAlreadyPrepared, got %v", err)
	}
}

// TestBOF_SetSacrificialThread_Default exercises the same
// before-/after-prepare contract for the sacrificial-thread
// knob. Default value must be zero (inline mode); the setter
// must accept a duration before Execute and reject after.
func TestBOF_SetSacrificialThread_Default(t *testing.T) {
	b, err := Load(loadLifecycleBOF(t, "hello_beacon.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer b.Close()
	if b.sacrificialTimeout != 0 {
		t.Error("BOF.sacrificialTimeout default should be 0 (inline mode)")
	}
	if err := b.SetSacrificialThread(2 * time.Second); err != nil {
		t.Fatalf("SetSacrificialThread before Execute should succeed: %v", err)
	}
	if b.sacrificialTimeout != 2*time.Second {
		t.Errorf("set timeout not stored: got %v", b.sacrificialTimeout)
	}
	// Zero re-disables.
	if err := b.SetSacrificialThread(0); err != nil {
		t.Fatalf("SetSacrificialThread(0) should succeed: %v", err)
	}
	if b.sacrificialTimeout != 0 {
		t.Errorf("zero should disable, got %v", b.sacrificialTimeout)
	}
}

// TestBOF_SetSacrificialThread_AfterPrepareErrors mirrors the
// SetPersistent post-prepare guard: changing isolation mode
// after the BOF has been Execute'd at least once would leave
// the mapping in an inconsistent half-prepared state.
func TestBOF_SetSacrificialThread_AfterPrepareErrors(t *testing.T) {
	b, err := Load(loadLifecycleBOF(t, "hello_beacon.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer b.Close()
	if _, err := b.Execute(nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if err := b.SetSacrificialThread(time.Second); err != ErrAlreadyPrepared {
		t.Errorf("SetSacrificialThread after Execute: want ErrAlreadyPrepared, got %v", err)
	}
}

// TestBOF_SacrificialThread_CrashIsolated is the headline
// witness: a BOF that deliberately dereferences NULL inside
// its own mapping must NOT terminate the implant when
// sacrificial mode is on. The VEH installed by
// installSacrificialVEH intercepts the AV, rewrites the
// faulting thread's RIP to the ExitThread(1) stub, and the
// host Execute call returns a clean error documenting the
// exception code + PC.
//
// Gated MALDEV_INTRUSIVE because:
//   - it installs a process-wide VEH (well-behaved, but a side
//     effect we don't want in casual `go test ./...`)
//   - the crasher fixture deliberately writes to 0x0 — a flag
//     for AV inspection if a curious EDR is watching test runs.
func TestBOF_SacrificialThread_CrashIsolated(t *testing.T) {
	testutil.RequireIntrusive(t)

	b, err := Load(loadLifecycleBOF(t, "crasher.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer b.Close()
	if err := b.SetSacrificialThread(5 * time.Second); err != nil {
		t.Fatalf("SetSacrificialThread: %v", err)
	}
	out, err := b.Execute(nil)
	if err == nil {
		t.Fatalf("Execute should return a crash error, got nil (out=%q)", out)
	}
	if !strings.Contains(err.Error(), "BOF crashed") {
		t.Errorf("error message should mention 'BOF crashed', got %q", err)
	}
	// The exception code 0xC0000005 is ACCESS_VIOLATION — the
	// only one a NULL write can produce on x64 Windows.
	if !strings.Contains(err.Error(), "0xc0000005") {
		t.Errorf("error message should mention exception 0xc0000005, got %q", err)
	}
	// The "about to crash" line ran before the AV — the BOF
	// reached BeaconPrintf at least once. Witness that output
	// capture survived the crash + the thread teardown.
	if !strings.Contains(string(out), "about to crash") {
		t.Errorf("output should contain the pre-crash BeaconPrintf line, got %q", out)
	}
	if strings.Contains(string(out), "should never reach here") {
		t.Errorf("post-crash line leaked into output: %q", out)
	}

	// Implant is alive: any further Go work succeeds.
	if _, err := os.Hostname(); err != nil {
		t.Errorf("host Go runtime alive after BOF crash? got %v", err)
	}
}

// TestBOF_SacrificialThread_HappyPath verifies that a normal
// BOF run produces the same observable output when executed via
// the sacrificial-thread path. Uses hello_beacon.o which prints
// a fixed greeting via BeaconPrintf — output must match the
// inline-mode reference exactly.
//
// Pinning the hello-path under sacrificial mode protects us
// against the surface drift that a broken thread / wait /
// thunk routine would cause (e.g. silent thread death before
// reaching BeaconPrintf).
func TestBOF_SacrificialThread_HappyPath(t *testing.T) {
	// Inline reference.
	bRef, err := Load(loadLifecycleBOF(t, "hello_beacon.o"))
	if err != nil {
		t.Fatalf("ref Load: %v", err)
	}
	defer bRef.Close()
	refOut, err := bRef.Execute(nil)
	if err != nil {
		t.Fatalf("ref Execute: %v", err)
	}

	// Sacrificial.
	b, err := Load(loadLifecycleBOF(t, "hello_beacon.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer b.Close()
	if err := b.SetSacrificialThread(5 * time.Second); err != nil {
		t.Fatalf("SetSacrificialThread: %v", err)
	}
	out, err := b.Execute(nil)
	if err != nil {
		t.Fatalf("sacrificial Execute: %v", err)
	}
	if string(out) != string(refOut) {
		t.Errorf("sacrificial output differs from inline\ninline:       %q\nsacrificial:  %q",
			refOut, out)
	}
}

// TestBOF_SacrificialThread_SharedTrampolineDistinctArgs pins the
// contract for the per-process shared trampoline: many successive
// sacrificial Execute calls — each with a different argument
// buffer — must each see exactly their own args. A bug in the
// per-call *sacArgs capsule layout (or premature GC of a previous
// capsule) would cause one iteration to observe another's
// argPtr/argLen pair and either crash or echo a stale string.
//
// Uses parse_args.o which BeaconPrintf-echoes the length-prefixed
// string from the args buffer; comparing against the freshly
// packed input per iteration catches both kinds of regression.
//
// Constraint: parse_args.c forwards the extracted string as the
// fmt argument to BeaconPrintf, so inputs containing '%' would be
// consumed by expandCFormat. The fixtures below are intentionally
// '%'-free.
func TestBOF_SacrificialThread_SharedTrampolineDistinctArgs(t *testing.T) {
	b, err := Load(loadLifecycleBOF(t, "parse_args.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer b.Close()
	if err := b.SetSacrificialThread(5 * time.Second); err != nil {
		t.Fatalf("SetSacrificialThread: %v", err)
	}

	inputs := []string{
		"alpha", "bravo-bravo", "charlie 12345",
		"delta\x09tab", "echo", "foxtrot-medium-length-string",
		"golf", "hotel-hotel-hotel", "india", "juliet-final",
	}
	for i, s := range inputs {
		args := NewArgs()
		args.AddInt(int32(i))
		args.AddString(s)
		out, err := b.Execute(args.Pack())
		if err != nil {
			t.Fatalf("iter %d (%q): Execute: %v", i, s, err)
		}
		if !strings.Contains(string(out), s) {
			t.Errorf("iter %d: output %q does not contain %q", i, out, s)
		}
	}
}

// TestBOF_SetExecuteAsToken_RoundTrip pins the token getter/setter
// pair. Round-trip with a non-zero windows.Token (a sentinel value
// is fine — the setter does not dereference, only stores), then
// clear back to zero. Wire-up is exercised by
// TestBOF_SacrificialThread_WithCurrentProcessToken below.
func TestBOF_SetExecuteAsToken_RoundTrip(t *testing.T) {
	b := &BOF{}
	if b.executeAsToken != 0 {
		t.Fatal("fresh *BOF must default to zero token")
	}
	b.SetExecuteAsToken(windows.Token(0xDEADBEEF))
	if b.executeAsToken != 0xDEADBEEF {
		t.Errorf("setter did not store; got 0x%X", b.executeAsToken)
	}
	b.SetExecuteAsToken(0)
	if b.executeAsToken != 0 {
		t.Errorf("zero did not clear; got 0x%X", b.executeAsToken)
	}
}

// TestBOF_SacrificialThread_WithCurrentProcessToken proves the
// SetThreadToken plumbing in callEntrySacrificial: with a real
// impersonation-grade token (the current process's primary token
// duplicated to impersonation-level) the sacrificial Execute must
// succeed end-to-end.
//
// Gated on IsElevated: `SetThreadToken` requires SeImpersonate-
// Privilege, which Windows grants by default only to Admin /
// LocalSystem / Service contexts. Non-admin processes can not
// self-impersonate via this API even with their own token. The
// round-trip test above covers the setter; this test exercises
// the actual SetThreadToken wire-up.
//
// Not a behavioural impersonation test — the BOF here is just
// hello_beacon (no token-aware code). Proving the BOF saw a
// different identity would require a fixture that calls
// OpenThreadToken / GetTokenInformation and prints — heavier
// than this commit's scope.
func TestBOF_SacrificialThread_WithCurrentProcessToken(t *testing.T) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("SetThreadToken requires SeImpersonatePrivilege (run elevated)")
	}

	var primary windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(),
		windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &primary); err != nil {
		t.Fatalf("OpenProcessToken: %v", err)
	}
	defer windows.CloseHandle(windows.Handle(primary))

	var impersonation windows.Token
	// desiredAccess=0 inherits the source token's rights. We opened
	// primary with TOKEN_DUPLICATE|TOKEN_QUERY which is NOT enough
	// for SetThreadToken — explicitly request TOKEN_IMPERSONATE +
	// TOKEN_QUERY on the duplicate.
	if err := windows.DuplicateTokenEx(primary,
		windows.TOKEN_IMPERSONATE|windows.TOKEN_QUERY,
		nil,
		windows.SecurityImpersonation, windows.TokenImpersonation,
		&impersonation); err != nil {
		t.Fatalf("DuplicateTokenEx: %v", err)
	}
	defer windows.CloseHandle(windows.Handle(impersonation))

	b, err := Load(loadLifecycleBOF(t, "hello_beacon.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer b.Close()
	if err := b.SetSacrificialThread(5 * time.Second); err != nil {
		t.Fatalf("SetSacrificialThread: %v", err)
	}
	b.SetExecuteAsToken(impersonation)

	out, err := b.Execute(nil)
	if err != nil {
		t.Fatalf("Execute under SetThreadToken: %v", err)
	}
	if len(out) == 0 {
		t.Errorf("expected hello_beacon output, got empty")
	}
}

// TestBOF_SacrificialThread_TokenAccessDeniedSurfaces verifies the
// error path: when the operator supplies a token the host can't
// apply (non-admin process trying to SetThreadToken on its own
// thread, missing SeImpersonatePrivilege), the sacrificial Execute
// must surface a non-nil error containing "SetThreadToken" and tear
// down the thread cleanly instead of leaking it.
//
// Only runs when NOT elevated — when elevated, SetThreadToken
// succeeds and this test no longer reproduces the access-denied
// path. The complement of WithCurrentProcessToken above.
func TestBOF_SacrificialThread_TokenAccessDeniedSurfaces(t *testing.T) {
	if windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("error-surface test only meaningful when SeImpersonatePrivilege is absent")
	}

	var primary windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(),
		windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &primary); err != nil {
		t.Fatalf("OpenProcessToken: %v", err)
	}
	defer windows.CloseHandle(windows.Handle(primary))

	var impersonation windows.Token
	// desiredAccess=0 inherits the source token's rights. We opened
	// primary with TOKEN_DUPLICATE|TOKEN_QUERY which is NOT enough
	// for SetThreadToken — explicitly request TOKEN_IMPERSONATE +
	// TOKEN_QUERY on the duplicate.
	if err := windows.DuplicateTokenEx(primary,
		windows.TOKEN_IMPERSONATE|windows.TOKEN_QUERY,
		nil,
		windows.SecurityImpersonation, windows.TokenImpersonation,
		&impersonation); err != nil {
		t.Fatalf("DuplicateTokenEx: %v", err)
	}
	defer windows.CloseHandle(windows.Handle(impersonation))

	b, err := Load(loadLifecycleBOF(t, "hello_beacon.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer b.Close()
	if err := b.SetSacrificialThread(5 * time.Second); err != nil {
		t.Fatalf("SetSacrificialThread: %v", err)
	}
	b.SetExecuteAsToken(impersonation)

	_, err = b.Execute(nil)
	if err == nil {
		t.Fatal("expected SetThreadToken error path; got nil — is the test running elevated?")
	}
	if !strings.Contains(err.Error(), "SetThreadToken") {
		t.Errorf("error %q does not mention SetThreadToken", err)
	}
}

// TestBOF_PDataRegistered pins the SEH-unwind registration in
// prepare: a BOF whose COFF carries a non-empty .pdata section
// (mingw-w64 default for any function call with a stack frame)
// must end up with b.pdataTable / b.pdataCount populated so the
// kernel can resolve RUNTIME_FUNCTION entries during stack
// unwinds — exceptions thrown inside the BOF would otherwise
// abort instead of unwinding into the BOF's handler frames.
//
// realworld_calls.o is the canonical fixture: 12-byte .pdata =
// exactly one RUNTIME_FUNCTION. Close must clear the fields and
// call RtlDeleteFunctionTable before VirtualFree.
func TestBOF_PDataRegistered(t *testing.T) {
	b, err := Load(loadLifecycleBOF(t, "realworld_calls.o"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := b.Execute(nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if b.pdataTable == nil {
		t.Fatal("pdataTable nil after Execute — .pdata not registered with RtlAddFunctionTable")
	}
	if b.pdataCount == 0 {
		t.Errorf("pdataCount = 0, want >= 1")
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if b.pdataTable != nil || b.pdataCount != 0 {
		t.Errorf("Close did not clear pdata fields: table=%p count=%d", b.pdataTable, b.pdataCount)
	}
}
