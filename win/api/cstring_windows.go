//go:build windows

package api

import (
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SafeRegionBytes returns the number of bytes that can be safely read
// starting at ptr without crossing into uncommitted memory. Zero when
// ptr lies in a free or reserved region. Uses a single VirtualQuery
// syscall.
//
// The bound is conservative — only the contiguous committed region
// containing ptr is reported. Reads past the returned count may still
// be valid (an adjacent committed region) but must be re-probed.
func SafeRegionBytes(ptr uintptr) uintptr {
	if ptr == 0 {
		return 0
	}
	var mbi windows.MemoryBasicInformation
	if err := windows.VirtualQuery(ptr, &mbi, unsafe.Sizeof(mbi)); err != nil {
		return 0
	}
	if mbi.State != windows.MEM_COMMIT {
		return 0
	}
	end := mbi.BaseAddress + mbi.RegionSize
	if end <= ptr {
		return 0
	}
	return end - ptr
}

// CStringFromPtr reads a NUL-terminated C string from a raw uintptr,
// capping at max bytes so a malformed or non-terminated pointer cannot
// drive the read off the end of mapped memory.
//
// Designed for the common case where Go code receives a uintptr from
// a syscall.NewCallback thunk or a Win32 API and needs to materialise
// the pointed-at string as a Go string. The standard library helper
// windows.BytePtrToString takes a *byte and offers no length cap; this
// helper takes uintptr and bounds the walk, which is the right shape
// for callback-thunk argument handling.
//
// Returns "" when ptr is 0 or when the destination region is not
// committed. The walk is additionally capped by VirtualQuery to the
// committed region containing ptr — defends against pointers that
// look valid but cross into a guard page or freed region. Returns
// up to max bytes when no NUL is found within the bound; callers can
// detect that case by checking len(result) == max.
func CStringFromPtr(ptr uintptr, max int) string {
	if ptr == 0 {
		return ""
	}
	safe := SafeRegionBytes(ptr)
	if safe == 0 {
		return ""
	}
	if uintptr(max) > safe {
		max = int(safe)
	}
	for n := 0; n < max; n++ {
		if *(*byte)(unsafe.Pointer(ptr + uintptr(n))) == 0 {
			return string(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), n))
		}
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), max))
}

// WStringFromPtr decodes a UTF-16LE NUL-terminated string from a raw
// uintptr. max bounds the scan in WIDE characters (each 2 bytes).
// Same SafeRegionBytes contract as CStringFromPtr — the walk is
// capped to the committed region containing ptr, so a bogus or
// crossing-into-guard-page pointer returns "" instead of crashing.
func WStringFromPtr(ptr uintptr, max int) string {
	if ptr == 0 {
		return ""
	}
	safe := SafeRegionBytes(ptr)
	if safe < 2 {
		return ""
	}
	maxBytes := uintptr(max) * 2
	if maxBytes > safe {
		maxBytes = safe &^ 1 // drop trailing odd byte
	}
	maxUnits := int(maxBytes / 2)
	var units []uint16
	for i := 0; i < maxUnits; i++ {
		u := *(*uint16)(unsafe.Pointer(ptr + uintptr(i*2)))
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return string(utf16.Decode(units))
}
