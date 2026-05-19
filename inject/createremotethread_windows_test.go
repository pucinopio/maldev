//go:build windows

package inject

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"

	"github.com/oioio-space/maldev/testutil"
)

// TestCreateRemoteThreadWithCaller_LocalProcess verifies the nil-Caller
// fallback path on the current-process pseudo-handle. The "remote"
// process is self; the thread runs a single 0xC3 (ret) instruction
// then exits. Asserts the thread handle is non-zero, the wait
// completes within a sane window, and no resource leaks.
//
// Self-targeting is the cheapest way to exercise the kernel32
// CreateRemoteThread path without a sacrificial helper process —
// the function doesn't care that the handle is the pseudo-self handle.
func TestCreateRemoteThreadWithCaller_LocalProcess(t *testing.T) {
	addr, free := allocRetCode(t)
	defer free()

	h, err := CreateRemoteThreadWithCaller(windows.CurrentProcess(), addr, 0, nil)
	require.NoError(t, err)
	require.NotZero(t, h)
	defer windows.CloseHandle(h)

	// Wait for the ret-only thread to finish — should be near-instant.
	st, err := windows.WaitForSingleObject(h, 5_000)
	require.NoError(t, err)
	require.Equal(t, uint32(windows.WAIT_OBJECT_0), st)
}

// TestCreateRemoteThreadWithCaller_Matrix sweeps every meaningful
// (wsyscall.Method, SSN-resolver) combination through the helper on
// the current process. 14 sub-tests sourced from
// testutil.CallerResolverMatrix — same row set the runtime/bof
// matrices use, so a regression in any single cell shows up across
// every inject + bof consumer at once.
//
// Each sub-test spawns a ret-only thread under that Caller, waits
// for it to exit, and closes the handle. Coverage:
//
//   - 2 hook-free paths: WinAPI + nil resolver, NativeAPI + nil resolver
//   - 12 syscall paths: Direct / Indirect / IndirectAsm × HellsGate /
//     HalosGate / Tartarus / HashGate
func TestCreateRemoteThreadWithCaller_Matrix(t *testing.T) {
	addr, free := allocRetCode(t)
	defer free()

	for _, cm := range testutil.CallerResolverMatrix(t) {
		t.Run(cm.Name, func(t *testing.T) {
			h, err := CreateRemoteThreadWithCaller(windows.CurrentProcess(), addr, 0, cm.Caller)
			require.NoError(t, err, "CreateRemoteThreadWithCaller under %s", cm.Name)
			require.NotZero(t, h)
			defer windows.CloseHandle(h)

			st, err := windows.WaitForSingleObject(h, 5_000)
			require.NoError(t, err)
			require.Equal(t, uint32(windows.WAIT_OBJECT_0), st,
				"thread didn't reach signaled state under %s", cm.Name)
		})
	}
}

// allocRetCode reserves an RX page containing a single 0xC3 (ret)
// instruction and returns the entry address + a free closure. The
// page is allocated PAGE_READWRITE, the opcode is written, then the
// page is flipped to PAGE_EXECUTE_READ — the standard pattern used
// across runtime/bof + evasion sub-packages, kept inline here so the
// test stays self-contained.
func allocRetCode(t *testing.T) (uintptr, func()) {
	t.Helper()
	const size = 0x1000
	addr, err := windows.VirtualAlloc(0, size,
		windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	require.NoError(t, err)
	*(*byte)(unsafe.Pointer(addr)) = 0xC3 // ret
	var old uint32
	require.NoError(t, windows.VirtualProtect(addr, size, windows.PAGE_EXECUTE_READ, &old))
	return addr, func() { _ = windows.VirtualFree(addr, 0, windows.MEM_RELEASE) }
}
