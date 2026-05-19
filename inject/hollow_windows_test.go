//go:build windows

package inject

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"

	"github.com/oioio-space/maldev/testutil"
)

// TestHollow_RejectsX86Payload pins the early-out: a 32-bit PE
// payload must surface ErrHollowParse without ever touching the
// target process. Uses C:\Windows\SysWOW64\notepad.exe — the real
// 32-bit notepad that every 64-bit Windows ships, so the test
// runs against a genuine i386 PE32 image rather than synthetic
// bytes.
func TestHollow_RejectsX86Payload(t *testing.T) {
	x86 := loadRealX86Notepad(t)
	_, err := Hollow(HollowConfig{
		Target:  systemTarget(t),
		Payload: x86,
	})
	if !errors.Is(err, ErrHollowParse) {
		t.Fatalf("want ErrHollowParse, got %v", err)
	}
}

// loadRealX86Notepad reads SysWOW64\notepad.exe — the real x86 PE
// shipped by every 64-bit Windows. Skips the test cleanly when
// SysWOW64 isn't present (e.g. a 32-bit-only Windows install or a
// stripped image without WoW64).
func loadRealX86Notepad(t *testing.T) []byte {
	t.Helper()
	const path = `C:\Windows\SysWOW64\notepad.exe`
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("%s not available (WoW64 absent?): %v", path, err)
	}
	return data
}

// TestHollow_GarbagePayloadErrors covers the parse-failure branch
// for non-PE bytes — must surface ErrHollowParse, must not spawn.
func TestHollow_GarbagePayloadErrors(t *testing.T) {
	_, err := Hollow(HollowConfig{
		Target:  systemTarget(t),
		Payload: []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x00, 0x00, 0x00},
	})
	if !errors.Is(err, ErrHollowParse) {
		t.Fatalf("want ErrHollowParse, got %v", err)
	}
}

// TestHollow_BadTargetSurfaces ErrHollowSpawn — exercises the
// terminate-on-failure path: CreateProcess fails, no zombie left
// behind, the error wraps the spawn sentinel.
func TestHollow_BadTargetSurfaces(t *testing.T) {
	x64 := loadFixture(t, "winhello.exe")
	_, err := Hollow(HollowConfig{
		Target:  `C:\Windows\System32\does-not-exist.exe`,
		Payload: x64,
	})
	if !errors.Is(err, ErrHollowSpawn) {
		t.Fatalf("want ErrHollowSpawn, got %v", err)
	}
}

// TestHollow_NotepadEndToEnd is the headline functional test:
// spawn notepad SUSPENDED, hollow with winhello (x64 PE that
// prints "hello" and exits 0), resume, wait, verify clean exit.
//
// Intrusive — gated on MALDEV_INTRUSIVE because it creates a
// real notepad process. Skips cleanly when the gate is off.
func TestHollow_NotepadEndToEnd(t *testing.T) {
	testutil.RequireIntrusive(t)
	payload := loadFixture(t, "winhello.exe")

	res, err := Hollow(HollowConfig{
		Target:  `C:\Windows\System32\notepad.exe`,
		Payload: payload,
	})
	require.NoError(t, err)
	defer windows.CloseHandle(res.Process)
	defer windows.CloseHandle(res.Thread)
	require.NotZero(t, res.PID)

	// Best-effort: terminate the hollowed process at test exit
	// even if ResumeThread / WaitForSingleObject misbehave so we
	// never leak a process across CI runs.
	defer windows.TerminateProcess(res.Process, 0)

	if _, err := windows.ResumeThread(res.Thread); err != nil {
		t.Fatalf("ResumeThread: %v", err)
	}

	// 5 s ceiling — winhello prints + exits sub-second in practice.
	st, err := windows.WaitForSingleObject(res.Process, 5_000)
	require.NoError(t, err)
	require.Equal(t, uint32(windows.WAIT_OBJECT_0), st,
		"hollowed process didn't exit within 5s under the winhello payload")
}

// TestHollow_CallerMatrix exercises the Caller-routed unmap path
// across every (Method, SSN-resolver) combination so a regression
// in NtUnmapViewOfSection under any syscall method surfaces here.
// 14 sub-tests sourced from testutil.CallerResolverMatrix; each
// spawns notepad SUSPENDED, runs the full hollow under that
// Caller, asserts the process exits clean. Intrusive.
func TestHollow_CallerMatrix(t *testing.T) {
	testutil.RequireIntrusive(t)
	payload := loadFixture(t, "winhello.exe")

	for _, cm := range testutil.CallerResolverMatrix(t) {
		t.Run(cm.Name, func(t *testing.T) {
			res, err := Hollow(HollowConfig{
				Target:  `C:\Windows\System32\notepad.exe`,
				Payload: payload,
				Caller:  cm.Caller,
			})
			require.NoError(t, err, "Hollow under %s", cm.Name)
			defer windows.CloseHandle(res.Process)
			defer windows.CloseHandle(res.Thread)
			defer windows.TerminateProcess(res.Process, 0)

			_, err = windows.ResumeThread(res.Thread)
			require.NoError(t, err)

			st, err := windows.WaitForSingleObject(res.Process, 5_000)
			require.NoError(t, err)
			require.Equal(t, uint32(windows.WAIT_OBJECT_0), st,
				"hollowed process didn't exit clean under %s", cm.Name)
		})
	}
}

// loadFixture reads a PE fixture from pe/packer/testdata and
// skips the test cleanly when absent — these files are committed
// to the repo, so a missing fixture means a corrupt checkout.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "pe", "packer", "testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture %s missing: %v", path, err)
	}
	return data
}

// systemTarget returns a target path used for cheap negative-
// path tests where we don't actually want to spawn anything.
// Picks an existing system binary so CreateProcess WILL succeed
// in the parse-failure tests (the early-out beats spawning).
func systemTarget(t *testing.T) string {
	t.Helper()
	// Don't spawn — return the path string for the test that
	// rejects before CreateProcess.
	return `C:\Windows\System32\notepad.exe`
}

