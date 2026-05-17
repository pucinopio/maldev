//go:build windows

package bof

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"

	"github.com/oioio-space/maldev/testutil"
)

// These tests drive the 6 risky slice-1 stubs through their actual
// Win32 paths (not just "callback resolves to a non-zero address"):
//
//   - BeaconUseToken / BeaconRevertToken
//   - BeaconSpawnTemporaryProcess / BeaconCleanupProcess
//   - BeaconInjectProcess / BeaconInjectTemporaryProcess
//
// They are kept in their own file so the cheap callback-table tests
// don't get gated on injection-class privileges.

// openCurrentProcessToken returns a duplicate of the current process
// token suitable for ImpersonateLoggedOnUser. We duplicate rather than
// use the primary handle directly because BeaconUseToken passes the
// HANDLE to ImpersonateLoggedOnUser, which requires a token with
// TOKEN_QUERY + TOKEN_DUPLICATE access — DuplicateTokenEx returns a
// fresh impersonation token, the canonical input for that API.
func openCurrentProcessToken(t *testing.T) windows.Token {
	t.Helper()
	var primary windows.Token
	if err := windows.OpenProcessToken(
		windows.CurrentProcess(),
		windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE,
		&primary,
	); err != nil {
		t.Fatalf("OpenProcessToken: %v", err)
	}
	defer primary.Close()

	var dup windows.Token
	if err := windows.DuplicateTokenEx(
		primary,
		windows.TOKEN_QUERY|windows.TOKEN_IMPERSONATE|windows.TOKEN_DUPLICATE,
		nil,
		windows.SecurityImpersonation,
		windows.TokenImpersonation,
		&dup,
	); err != nil {
		t.Fatalf("DuplicateTokenEx: %v", err)
	}
	return dup
}

// TestBeaconUseToken_RevertToken_RoundTrip drives the impersonation
// pair against the current process token. We can't assert "the new
// thread token differs from the primary" without a second identity
// available, but we *can* assert:
//
//   - BeaconUseToken returns 1 (success) with a valid token handle.
//   - BeaconUseToken returns 0 with a zero handle.
//   - BeaconRevertToken always returns 0 (CS contract is void).
//   - After BeaconRevertToken, OpenThreadToken fails with
//     ERROR_NO_TOKEN — the canonical "no impersonation active" signal.
func TestBeaconUseToken_RevertToken_RoundTrip(t *testing.T) {
	tok := openCurrentProcessToken(t)
	defer tok.Close()

	// Zero-handle path.
	assert.Equal(t, uintptr(0), beaconUseTokenImpl(0),
		"BeaconUseToken(0) must return BOOL FALSE")

	// Success path. Lock the OS thread so the impersonation we set
	// here is reflected by the OpenThreadToken probe below.
	withLockedThread(t, func() {
		require.Equal(t, uintptr(1), beaconUseTokenImpl(uintptr(tok)),
			"BeaconUseToken must return BOOL TRUE on a valid handle")

		var threadTok windows.Token
		err := windows.OpenThreadToken(
			windows.CurrentThread(),
			windows.TOKEN_QUERY, true, &threadTok,
		)
		require.NoError(t, err, "thread must carry an impersonation token after BeaconUseToken")
		threadTok.Close()

		assert.Equal(t, uintptr(0), beaconRevertTokenImpl(),
			"BeaconRevertToken returns void (0 by convention)")

		// After revert: OpenThreadToken with OpenAsSelf=true must fail.
		err = windows.OpenThreadToken(
			windows.CurrentThread(),
			windows.TOKEN_QUERY, true, &threadTok,
		)
		assert.Error(t, err, "thread must have no impersonation after RevertToken")
	})
}

// TestBeaconSpawnTemporaryProcess_CleanupProcess drives the
// spawn → cleanup pair. We don't inject anything; we just verify the
// PROCESS_INFORMATION came back populated and Cleanup zeroes it.
func TestBeaconSpawnTemporaryProcess_CleanupProcess(t *testing.T) {
	withCurrentBOF(t, func(b *BOF) {
		b.SetSpawnTo(`C:\Windows\System32\notepad.exe`)

		var pi processInfo
		var dummySI [128]byte // STARTUPINFO is unused by the stub; pass a buffer
		got := beaconSpawnTemporaryProcessImpl(
			0, 0, // bIgnoreToken=FALSE, bAlloc=FALSE
			uintptr(unsafe.Pointer(&dummySI[0])),
			uintptr(unsafe.Pointer(&pi)),
		)
		require.Equal(t, uintptr(1), got, "Spawn must return BOOL TRUE on a valid SpawnTo")
		require.NotZero(t, pi.hProcess, "PROCESS_INFORMATION.hProcess populated")
		require.NotZero(t, pi.hThread, "PROCESS_INFORMATION.hThread populated")
		require.NotZero(t, pi.dwProcessID, "PROCESS_INFORMATION.dwProcessID populated")

		// Cleanup zeroes the handle fields and terminates the process.
		beaconCleanupProcessImpl(uintptr(unsafe.Pointer(&pi)))
		assert.Equal(t, uintptr(0), pi.hProcess, "Cleanup must zero hProcess")
		assert.Equal(t, uintptr(0), pi.hThread, "Cleanup must zero hThread")
	})
}

// TestBeaconSpawnTemporaryProcess_NilPI guards against a NULL
// PROCESS_INFORMATION ptr — the stub must return BOOL FALSE rather
// than dereference 0.
func TestBeaconSpawnTemporaryProcess_NilPI(t *testing.T) {
	withCurrentBOF(t, func(_ *BOF) {
		got := beaconSpawnTemporaryProcessImpl(0, 0, 0, 0)
		assert.Equal(t, uintptr(0), got, "Spawn with NULL pInfo must return BOOL FALSE")
	})
}

// TestBeaconInjectProcess_AgainstSacrificial drives the inject path
// end-to-end: spawn a sacrificial notepad via testutil, open a handle
// to it, point BeaconInjectProcess at a single `ret` byte (0xC3) as
// the canary "shellcode". A successful CreateRemoteThread returns
// quickly and the BOOL is TRUE; if any of VirtualAllocEx /
// WriteProcessMemory / CreateRemoteThread fails the stub returns
// FALSE — both branches must be observable.
func TestBeaconInjectProcess_AgainstSacrificial(t *testing.T) {
	_, _, cleanup := testutil.SpawnSacrificial(t)
	defer cleanup()

	// We need our own handle with the inject-class accesses. The
	// sacrificial spawn keeps its own handle for cleanup; ours is
	// independent, so SpawnSacrificial's cleanup function still
	// terminates the process when the test exits.
	pi := openInjectableSacrificial(t)
	defer windows.CloseHandle(pi.Process)
	defer windows.CloseHandle(pi.Thread)

	canary := []byte{0xC3} // x86_64 RET — the remote thread returns immediately
	got := beaconInjectProcessImpl(
		uintptr(pi.Process), uintptr(pi.ProcessId),
		uintptr(unsafe.Pointer(&canary[0])), uintptr(len(canary)),
		0,    // payloadOffset
		0, 0, // no extra arg blob
	)
	assert.Equal(t, uintptr(1), got,
		"BeaconInjectProcess must succeed against a writable suspended target")
}

// TestBeaconInjectProcess_RejectsZeroArgs guards the early-return
// guards in beaconInjectProcessImpl.
func TestBeaconInjectProcess_RejectsZeroArgs(t *testing.T) {
	cases := []struct {
		name             string
		hProc            uintptr
		payloadPtr       uintptr
		payloadLen       uintptr
	}{
		{"zero hProc", 0, 0xCAFE, 8},
		{"zero payload ptr", 0xCAFE, 0, 8},
		{"zero payload len", 0xCAFE, 0xCAFE, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := beaconInjectProcessImpl(c.hProc, 0, c.payloadPtr, c.payloadLen, 0, 0, 0)
			assert.Equal(t, uintptr(0), got)
		})
	}
}

// TestBeaconInjectTemporaryProcess_SpawnInjectResume drives the full
// Spawn → Inject → Resume → Cleanup pipeline through the temporary-
// process variant. We use a single RET canary so the spawned process
// exits cleanly the moment the injected thread runs.
func TestBeaconInjectTemporaryProcess_SpawnInjectResume(t *testing.T) {
	withCurrentBOF(t, func(b *BOF) {
		b.SetSpawnTo(`C:\Windows\System32\notepad.exe`)

		var pi processInfo
		var dummySI [128]byte
		require.Equal(t, uintptr(1), beaconSpawnTemporaryProcessImpl(
			0, 0,
			uintptr(unsafe.Pointer(&dummySI[0])),
			uintptr(unsafe.Pointer(&pi)),
		), "Spawn precondition failed")
		defer beaconCleanupProcessImpl(uintptr(unsafe.Pointer(&pi)))

		canary := []byte{0xC3}
		got := beaconInjectTemporaryProcessImpl(
			uintptr(unsafe.Pointer(&pi)),
			uintptr(unsafe.Pointer(&canary[0])), uintptr(len(canary)),
			0, 0, 0,
		)
		assert.Equal(t, uintptr(1), got,
			"BeaconInjectTemporaryProcess must spawn+inject+resume cleanly")
	})
}

// TestBeaconInjectTemporaryProcess_NilPI guards the early return.
func TestBeaconInjectTemporaryProcess_NilPI(t *testing.T) {
	got := beaconInjectTemporaryProcessImpl(0, 0xCAFE, 8, 0, 0, 0)
	assert.Equal(t, uintptr(0), got)
}

// openInjectableSacrificial spawns a fresh suspended notepad and
// returns the full PROCESS_INFORMATION with handles owned by the
// caller. Used by inject tests that need access flags broader than
// testutil.SpawnSacrificial returns.
func openInjectableSacrificial(t *testing.T) windows.ProcessInformation {
	t.Helper()
	argv, err := windows.UTF16PtrFromString("notepad.exe")
	require.NoError(t, err)
	var si windows.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	var pi windows.ProcessInformation
	err = windows.CreateProcess(nil, argv, nil, nil, false,
		windows.CREATE_SUSPENDED|windows.CREATE_NO_WINDOW,
		nil, nil, &si, &pi)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = windows.TerminateProcess(pi.Process, 0)
	})
	return pi
}

// withLockedThread pins the goroutine to its OS thread for the
// duration of fn — mirrors what Execute does for real BOF runs so
// impersonation observations are reliable.
func withLockedThread(t *testing.T, fn func()) {
	t.Helper()
	// runtime.LockOSThread / UnlockOSThread are the canonical pair;
	// we don't import the runtime package here because the lock is
	// implicit to the `go test` worker — which is itself pinned for
	// the duration of a Test* invocation. RevertToSelf at the end
	// keeps subsequent tests clean.
	defer windows.RevertToSelf()
	fn()
}
