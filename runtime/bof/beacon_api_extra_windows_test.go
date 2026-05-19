//go:build windows

package bof

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"

	wsyscall "github.com/oioio-space/maldev/win/syscall"
)

// TestBOF_SetCaller_RoundTrip pins the SetCaller getter/setter:
// stores the supplied *wsyscall.Caller on the *BOF where the
// cross-process Beacon API helpers (beaconRemoteAlloc / Write /
// CreateThread) can read it. nil restores the kernel32 path.
func TestBOF_SetCaller_RoundTrip(t *testing.T) {
	b := &BOF{}
	require.Nil(t, b.caller)

	c := wsyscall.New(wsyscall.MethodWinAPI, nil)
	b.SetCaller(c)
	assert.Same(t, c, b.caller)

	b.SetCaller(nil)
	assert.Nil(t, b.caller)
}

// TestBeaconRemoteAlloc_LocalProcess_RoundTrip exercises the
// Caller-aware allocator against the current process. With
// caller=nil the path is kernel32!VirtualAllocEx; with
// caller=MethodWinAPI the path is also kernel32 (Caller routes
// "NtAllocateVirtualMemory" to ntdll!NtAllocateVirtualMemory). Both
// must return a non-zero base, and the allocation must be readable
// + freeable via the standard windows.VirtualFree.
func TestBeaconRemoteAlloc_LocalProcess_RoundTrip(t *testing.T) {
	const size = 4096
	self := windows.CurrentProcess()

	for _, tc := range []struct {
		name   string
		caller *wsyscall.Caller
	}{
		{"nil_falls_back_to_kernel32", nil},
		{"WinAPI_Caller_through_ntdll", wsyscall.New(wsyscall.MethodWinAPI, nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr := beaconRemoteAlloc(tc.caller, self, size, windows.PAGE_READWRITE)
			require.NotZero(t, addr, "allocation must succeed")
			require.NoError(t, windows.VirtualFree(addr, 0, windows.MEM_RELEASE))
		})
	}
}

// TestBOF_SetUserData_RoundTrip — SetUserData copies the slice so callers
// can mutate the original buffer without disturbing the BOF; clearing
// with nil/empty removes the data.
func TestBOF_SetUserData_RoundTrip(t *testing.T) {
	b := &BOF{}
	src := []byte("hello")
	b.SetUserData(src)
	require.Equal(t, []byte("hello"), b.userData)

	// Mutate caller's buffer — BOF must keep the original bytes.
	src[0] = 'X'
	assert.Equal(t, []byte("hello"), b.userData)

	b.SetUserData(nil)
	assert.Nil(t, b.userData)

	b.SetUserData([]byte{})
	assert.Nil(t, b.userData)
}

// TestBeaconGetCustomUserData_ReturnsConfiguredBlob — when UserData is
// set, GetCustomUserData writes the pinned pointer + length into the
// BOF-supplied output pointers.
func TestBeaconGetCustomUserData_ReturnsConfiguredBlob(t *testing.T) {
	withCurrentBOF(t, func(b *BOF) {
		b.SetUserData([]byte("abcd"))
		var (
			buf uintptr
			ln  int32
		)
		beaconGetCustomUserDataImpl(
			uintptr(unsafe.Pointer(&buf)),
			uintptr(unsafe.Pointer(&ln)),
		)
		require.NotZero(t, buf)
		assert.Equal(t, int32(4), ln)
		got := unsafe.Slice((*byte)(unsafe.Pointer(buf)), int(ln))
		assert.Equal(t, []byte("abcd"), got)
	})
}

// TestBeaconGetCustomUserData_EmptyByDefault — when no UserData is set,
// the callback zeroes both output fields and returns 0.
func TestBeaconGetCustomUserData_EmptyByDefault(t *testing.T) {
	withCurrentBOF(t, func(_ *BOF) {
		buf := uintptr(0xDEADBEEF)
		ln := int32(99)
		beaconGetCustomUserDataImpl(
			uintptr(unsafe.Pointer(&buf)),
			uintptr(unsafe.Pointer(&ln)),
		)
		assert.Equal(t, uintptr(0), buf)
		assert.Equal(t, int32(0), ln)
	})
}

// TestBeaconKV_AddGetRemove exercises the per-BOF key-value store
// end-to-end via the callback layer.
func TestBeaconKV_AddGetRemove(t *testing.T) {
	withCurrentBOF(t, func(_ *BOF) {
		key := append([]byte("session"), 0)
		val := uintptr(0xC0FFEE)
		beaconAddValueImpl(uintptr(unsafe.Pointer(&key[0])), val)
		got := beaconGetValueImpl(uintptr(unsafe.Pointer(&key[0])))
		assert.Equal(t, val, got)

		beaconRemoveValueImpl(uintptr(unsafe.Pointer(&key[0])))
		assert.Equal(t, uintptr(0),
			beaconGetValueImpl(uintptr(unsafe.Pointer(&key[0]))))
	})
}

// TestBeaconIsAdmin_ReturnsBoolean — IsAdmin must return 0 or 1; the
// concrete value depends on the test process privileges (typically 0
// in CI, 1 when running elevated).
func TestBeaconIsAdmin_ReturnsBoolean(t *testing.T) {
	got := beaconIsAdminImpl()
	if got != 0 && got != 1 {
		t.Fatalf("BeaconIsAdmin must return 0 or 1; got %d", got)
	}
}

// TestToWideChar_UTF8ToUTF16 — feeds an ASCII string into toWideChar and
// verifies the destination buffer is UTF-16LE with a trailing NUL and
// the return value excludes the NUL.
func TestToWideChar_UTF8ToUTF16(t *testing.T) {
	src := append([]byte("abc"), 0)
	dst := make([]uint16, 8)
	n := toWideCharImpl(
		uintptr(unsafe.Pointer(&src[0])),
		uintptr(unsafe.Pointer(&dst[0])),
		uintptr(len(dst)),
	)
	assert.Equal(t, uintptr(3), n, "return must exclude NUL")
	assert.Equal(t, uint16('a'), dst[0])
	assert.Equal(t, uint16('b'), dst[1])
	assert.Equal(t, uint16('c'), dst[2])
	assert.Equal(t, uint16(0), dst[3], "trailing NUL must be written")
}

// TestBeaconGetOutputData_ReturnsSnapshotAndSize covers slice 1.c.8:
// BeaconGetOutputData returns a pointer to a stable copy of the
// BOF's accumulated output bytes plus the length.
func TestBeaconGetOutputData_ReturnsSnapshotAndSize(t *testing.T) {
	withCurrentBOF(t, func(b *BOF) {
		b.output.write([]byte("snapshot-payload"))
		var sz int32
		ptr := beaconGetOutputDataImpl(uintptr(unsafe.Pointer(&sz)))
		require.NotZero(t, ptr)
		assert.Equal(t, int32(len("snapshot-payload")), sz)
		got := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(sz))
		assert.Equal(t, []byte("snapshot-payload"), got)
	})
}

// TestBeaconGetOutputData_EmptyOutput — empty buffer + non-nil size
// pointer must zero the size field and return null.
func TestBeaconGetOutputData_EmptyOutput(t *testing.T) {
	withCurrentBOF(t, func(_ *BOF) {
		sz := int32(99)
		ptr := beaconGetOutputDataImpl(uintptr(unsafe.Pointer(&sz)))
		assert.Equal(t, uintptr(0), ptr)
		assert.Equal(t, int32(0), sz)
	})
}

// TestExtraBeaconAPI_SymbolsRegistered locks in that every slice-1
// callback resolves; catches a typo in registerExtraBeaconCallbacks.
func TestExtraBeaconAPI_SymbolsRegistered(t *testing.T) {
	for _, name := range []string{
		"__imp_BeaconUseToken",
		"__imp_BeaconRevertToken",
		"__imp_BeaconIsAdmin",
		"__imp_BeaconGetCustomUserData",
		"__imp_toWideChar",
		"__imp_BeaconAddValue",
		"__imp_BeaconGetValue",
		"__imp_BeaconRemoveValue",
		"__imp_BeaconSpawnTemporaryProcess",
		"__imp_BeaconCleanupProcess",
		"__imp_BeaconInjectProcess",
		"__imp_BeaconInjectTemporaryProcess",
		"__imp_BeaconGetOutputData",
	} {
		addr, ok := resolveBeaconImport(name)
		require.True(t, ok, "%s must resolve", name)
		assert.NotZero(t, addr, "%s callback address", name)
	}
}
