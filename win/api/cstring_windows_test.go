//go:build windows

package api

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func TestCStringFromPtr_Basic(t *testing.T) {
	src := []byte("hello\x00ignored")
	got := CStringFromPtr(uintptr(unsafe.Pointer(&src[0])), 64)
	assert.Equal(t, "hello", got)
}

func TestCStringFromPtr_NilReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", CStringFromPtr(0, 64))
}

func TestCStringFromPtr_NoTerminatorCappedAtMax(t *testing.T) {
	// 5 bytes, no NUL, max=3 — must return the first 3 and stop.
	src := []byte{'a', 'b', 'c', 'd', 'e'}
	got := CStringFromPtr(uintptr(unsafe.Pointer(&src[0])), 3)
	assert.Equal(t, "abc", got)
}

func TestCStringFromPtr_EmptyString(t *testing.T) {
	// First byte is NUL — empty string.
	src := []byte{0, 'x', 'y'}
	got := CStringFromPtr(uintptr(unsafe.Pointer(&src[0])), 16)
	assert.Equal(t, "", got)
}

// TestCStringFromPtr_UnmappedReturnsEmpty pins the safety-probe
// contract: a pointer that lands in free / reserved memory must not
// cause a read fault. We pick an address known to be unmapped on
// x64 Windows (0x0000000FFFFF0000 — well above heap, below
// kernel-reserved 0x80000000_00000000) and verify the helper
// returns "" via the SafeRegionBytes guard.
func TestCStringFromPtr_UnmappedReturnsEmpty(t *testing.T) {
	const unmapped uintptr = 0x0000000FFFFF0000
	got := CStringFromPtr(unmapped, 64)
	assert.Equal(t, "", got)
}

func TestSafeRegionBytes_NilReturnsZero(t *testing.T) {
	assert.Zero(t, SafeRegionBytes(0))
}

func TestSafeRegionBytes_MappedReturnsPositive(t *testing.T) {
	src := []byte("any committed slice")
	n := SafeRegionBytes(uintptr(unsafe.Pointer(&src[0])))
	assert.NotZero(t, n, "Go heap allocation should be committed")
}

func TestSafeRegionBytes_UnmappedReturnsZero(t *testing.T) {
	const unmapped uintptr = 0x0000000FFFFF0000
	assert.Zero(t, SafeRegionBytes(unmapped))
}

func TestWStringFromPtr_Basic(t *testing.T) {
	// "hi" in UTF-16LE + NUL terminator + trailing garbage.
	src := []uint16{'h', 'i', 0, 0xDEAD}
	got := WStringFromPtr(uintptr(unsafe.Pointer(&src[0])), 64)
	assert.Equal(t, "hi", got)
}

func TestWStringFromPtr_NilReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", WStringFromPtr(0, 64))
}

func TestWStringFromPtr_UnmappedReturnsEmpty(t *testing.T) {
	const unmapped uintptr = 0x0000000FFFFF0000
	assert.Equal(t, "", WStringFromPtr(unmapped, 64))
}

func TestWStringFromPtr_NoTerminatorCappedAtMax(t *testing.T) {
	src := []uint16{'a', 'b', 'c', 'd', 'e'}
	got := WStringFromPtr(uintptr(unsafe.Pointer(&src[0])), 3)
	assert.Equal(t, "abc", got)
}
