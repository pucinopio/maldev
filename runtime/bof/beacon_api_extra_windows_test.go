//go:build windows

package bof

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	} {
		addr, ok := resolveBeaconImport(name)
		require.True(t, ok, "%s must resolve", name)
		assert.NotZero(t, addr, "%s callback address", name)
	}
}
