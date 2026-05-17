//go:build windows

package bof

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

// cStr returns a NUL-terminated copy of s for use with cStringFromPtr.
// The slice is kept alive by the caller; tests should retain the
// returned []byte for the lifetime of the uintptr they derive.
func cStr(s string) []byte { return append([]byte(s), 0) }
func cPtr(b []byte) uintptr { return uintptr(unsafe.Pointer(&b[0])) }

// TestExpandCFormat_PercentS covers the string-pointer conversion: the
// pointer is dereferenced as a NUL-terminated C string and inlined
// into the output. NULL pointer renders as "(null)".
func TestExpandCFormat_PercentS(t *testing.T) {
	name := cStr("alice")
	got := expandCFormat("user=%s", []uintptr{cPtr(name)})
	assert.Equal(t, "user=alice", string(got))

	gotNull := expandCFormat("user=%s", []uintptr{0})
	assert.Equal(t, "user=(null)", string(gotNull))
}

// TestExpandCFormat_PercentD locks the int32 default + %lld 64-bit
// behaviour.
func TestExpandCFormat_PercentD(t *testing.T) {
	got := expandCFormat("pid=%d", []uintptr{1234})
	assert.Equal(t, "pid=1234", string(got))

	// Negative: lowest 32 bits represent -1.
	got = expandCFormat("rc=%d", []uintptr{^uintptr(0)})
	assert.Equal(t, "rc=-1", string(got))

	// %lld picks the full 64-bit width.
	got = expandCFormat("v=%lld", []uintptr{0x1_0000_0000})
	assert.Equal(t, "v=4294967296", string(got))
}

// TestExpandCFormat_PercentU covers unsigned 32 / 64.
func TestExpandCFormat_PercentU(t *testing.T) {
	got := expandCFormat("n=%u", []uintptr{42})
	assert.Equal(t, "n=42", string(got))

	got = expandCFormat("n=%llu", []uintptr{0x1_0000_0001})
	assert.Equal(t, "n=4294967297", string(got))
}

// TestExpandCFormat_PercentX covers hex lowercase / uppercase, 32 / 64.
func TestExpandCFormat_PercentX(t *testing.T) {
	got := expandCFormat("%x", []uintptr{0xCAFEBABE})
	assert.Equal(t, "cafebabe", string(got))

	got = expandCFormat("%X", []uintptr{0xCAFEBABE})
	assert.Equal(t, "CAFEBABE", string(got))

	// 64-bit via %I64x (Microsoft length modifier).
	got = expandCFormat("addr=%I64x", []uintptr{0x7FF6_0000_1234})
	assert.Equal(t, "addr=7ff600001234", string(got))
}

// TestExpandCFormat_PercentP locks the "0x"-prefixed pointer render.
func TestExpandCFormat_PercentP(t *testing.T) {
	got := expandCFormat("at %p", []uintptr{0x1234})
	assert.Equal(t, "at 0x1234", string(got))
}

// TestExpandCFormat_PercentPercent — literal percent.
func TestExpandCFormat_PercentPercent(t *testing.T) {
	got := expandCFormat("100%% done", nil)
	assert.Equal(t, "100% done", string(got))
}

// TestExpandCFormat_Mixed exercises the goldenpath: multiple conversions
// in one format string, mixed verbs.
func TestExpandCFormat_Mixed(t *testing.T) {
	name := cStr("svc")
	got := expandCFormat("name=%s pid=%d code=0x%x",
		[]uintptr{cPtr(name), 7000, 0xC0000005})
	assert.Equal(t, "name=svc pid=7000 code=0xc0000005", string(got))
}

// TestExpandCFormat_MissingArgsReadZero — when the BOF supplies fewer
// args than conversions, missing ones render as zero rather than
// reading uninitialised memory.
func TestExpandCFormat_MissingArgsReadZero(t *testing.T) {
	got := expandCFormat("a=%d b=%d c=%d", []uintptr{11, 22})
	assert.Equal(t, "a=11 b=22 c=0", string(got))
}

// TestExpandCFormat_UnknownVerbEmittedLiterally — `%q` is not a Beacon
// verb. The expander emits `%q` verbatim and does NOT consume the
// arg, so the next valid conversion stays aligned with operator intent.
func TestExpandCFormat_UnknownVerbEmittedLiterally(t *testing.T) {
	got := expandCFormat("%q-%d", []uintptr{42})
	assert.Equal(t, "%q-42", string(got))
}

// TestBeaconPrintfImpl_ExpandsVarargs drives the public callback with a
// realistic format + 3 args via the captured-uintptr surface.
func TestBeaconPrintfImpl_ExpandsVarargs(t *testing.T) {
	withCurrentBOF(t, func(b *BOF) {
		fmtBuf := cStr("user=%s pid=%d code=0x%x")
		name := cStr("alice")
		beaconPrintfImpl(0, cPtr(fmtBuf), cPtr(name), 1234, 0xDEAD, 0, 0, 0)
		assert.Equal(t, "user=alice pid=1234 code=0xdead", string(b.output.Bytes()))
	})
}

// TestBeaconFormatPrintfImpl_ExpandsVarargs round-trips through the
// formatp buffer: pre-allocate, format-printf, ToString, verify the
// rendered content lives in the buffer.
func TestBeaconFormatPrintfImpl_ExpandsVarargs(t *testing.T) {
	withCurrentBOF(t, func(_ *BOF) {
		var fp formatp
		fpPtr := uintptr(unsafe.Pointer(&fp))
		beaconFormatAllocImpl(fpPtr, 256)
		defer beaconFormatFreeImpl(fpPtr)

		fmtBuf := cStr("svc=%s rc=%d")
		name := cStr("svchost")
		beaconFormatPrintfImpl(fpPtr, cPtr(fmtBuf), cPtr(name), 42, 0, 0, 0, 0)

		var sz int32
		bufPtr := beaconFormatToStringImpl(fpPtr, uintptr(unsafe.Pointer(&sz)))
		got := unsafe.Slice((*byte)(unsafe.Pointer(bufPtr)), int(sz))
		assert.Equal(t, "svc=svchost rc=42", string(got))
	})
}

// TestBOF_SetSpawnToX86_RoundTrip pins the new setter + the dispatch
// shape: GetSpawnTo(TRUE) returns the x86 path, GetSpawnTo(FALSE)
// returns the x64 path. Clearing with "" zeroes the pinned slice.
func TestBOF_SetSpawnToX86_RoundTrip(t *testing.T) {
	withCurrentBOF(t, func(b *BOF) {
		b.SetSpawnTo(`C:\Windows\System32\rundll32.exe`)
		b.SetSpawnToX86(`C:\Windows\SysWOW64\rundll32.exe`)

		// x86=FALSE → x64 path
		ptr64 := beaconGetSpawnToImpl(0, 0)
		assert.Contains(t, cStringFromPtr(ptr64, 256), "System32")

		// x86=TRUE → x86 path
		ptr86 := beaconGetSpawnToImpl(1, 0)
		assert.Contains(t, cStringFromPtr(ptr86, 256), "SysWOW64")

		// Clear x86 → x86 dispatch falls back to the x64 path
		// (compat shim for BOFs built against legacy
		// `char *BeaconGetSpawnTo(void)` that pass garbage in the first
		// register).
		b.SetSpawnToX86("")
		assert.Equal(t, ptr64, beaconGetSpawnToImpl(1, 0),
			"x86 path cleared → fallback to x64 path")

		// Both empty → null pointer.
		b.SetSpawnTo("")
		assert.Equal(t, uintptr(0), beaconGetSpawnToImpl(1, 0))
		assert.Equal(t, uintptr(0), beaconGetSpawnToImpl(0, 0))
	})
}
