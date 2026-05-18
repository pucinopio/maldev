//go:build windows

package bof

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"

	"golang.org/x/sys/windows"
)

// beaconOutput is a thread-safe buffer for capturing BOF output. Each
// write is appended to the buf AND, if a stream channel is wired, sent
// to it (non-blocking — if the receiver isn't draining, the buffer
// keeps growing and the stream loses the chunk). Streaming lets
// operators consume long-running BOFs in real time via
// (*BOF).ExecuteStream.
type beaconOutput struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	stream chan<- []byte
}

func newBeaconOutput() *beaconOutput {
	return &beaconOutput{}
}

func (o *beaconOutput) printf(format string, args ...interface{}) {
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintf(&o.buf, format, args...)
}

func (o *beaconOutput) write(data []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buf.Write(data)
	if o.stream != nil {
		// Snapshot the bytes — the BOF may reuse the source buffer.
		chunk := make([]byte, len(data))
		copy(chunk, data)
		select {
		case o.stream <- chunk:
		default:
			// Receiver not draining — drop the chunk so we don't
			// stall the BOF on a slow consumer. The full output is
			// still recoverable via Bytes() after Execute returns.
		}
	}
}

func (o *beaconOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.String()
}

func (o *beaconOutput) Bytes() []byte {
	o.mu.Lock()
	defer o.mu.Unlock()
	b := o.buf.Bytes()
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// Args packs arguments into the format expected by the BOF entry point.
// BOF loaders expect a flat buffer where each argument is prefixed with
// its length — this mirrors the Cobalt Strike BeaconDataParse convention.
//
// Internally the struct keeps a list of byte chunks instead of a
// bytes.Buffer. AddBytes can then reference the caller's slice
// directly without copying; the single concatenation pass happens
// at Pack time. Matters when the payload carries large binary
// blobs (e.g. runtime/pe packs a multi-MB PE inline) — the prior
// bytes.Buffer-based design copied the blob once into the buffer
// and again into Pack's output, tripling peak memory.
type Args struct {
	chunks [][]byte
	size   int
}

// NewArgs allocates an empty argument packer.
func NewArgs() *Args {
	return &Args{}
}

// append registers a chunk in the cumulative buffer. Centralises
// the size bookkeeping so every Add* helper stays one-liner-ish.
func (a *Args) append(b []byte) {
	a.chunks = append(a.chunks, b)
	a.size += len(b)
}

// Wire format: little-endian to match the CS canonical (TrustedSec
// COFFLoader / Outflank read length prefixes via native-int memcpy,
// which is LE on x64). The consumer side in beacon_api_windows.go
// uses binary.LittleEndian for the same reason — keep this in sync.

// AddInt appends a 32-bit signed integer in little-endian byte order.
func (a *Args) AddInt(v int32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))
	a.append(b)
}

// AddShort appends a 16-bit signed integer in little-endian byte order.
func (a *Args) AddShort(v int16) {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(v))
	a.append(b)
}

// AddString appends a null-terminated string with a 4-byte little-endian
// length prefix. The length includes the null terminator.
func (a *Args) AddString(s string) {
	body := make([]byte, 4+len(s)+1)
	binary.LittleEndian.PutUint32(body[:4], uint32(len(s)+1))
	copy(body[4:], s)
	// body[4+len(s)] is already zero from make — that's the NUL.
	a.append(body)
}

// AddWideString appends a UTF-16LE-encoded NUL-terminated string with a
// 4-byte little-endian byte-length prefix. The length is the count of
// raw bytes (= 2 × number of wide chars including the trailing NUL),
// matching the BeaconDataExtract consumer contract: every Extract
// reads `chunkLen` BYTES regardless of whether the payload is ASCII
// or UTF-16. BOFs receiving a wchar_t* via the unpacked buffer get a
// directly-castable UTF-16LE blob.
func (a *Args) AddWideString(s string) {
	utf16 := windows.StringToUTF16(s) // includes trailing NUL
	body := make([]byte, 4+len(utf16)*2)
	binary.LittleEndian.PutUint32(body[:4], uint32(len(utf16)*2))
	for i, u := range utf16 {
		binary.LittleEndian.PutUint16(body[4+i*2:], u)
	}
	a.append(body)
}

// AddBytes appends a byte slice with a 4-byte little-endian length
// prefix. The data slice is referenced, not copied — callers
// must keep it stable until Pack runs. Saves a buffer-side copy
// of multi-MB blobs (e.g. PE bytes packed by runtime/pe).
func (a *Args) AddBytes(data []byte) {
	hdr := make([]byte, 4)
	binary.LittleEndian.PutUint32(hdr, uint32(len(data)))
	a.append(hdr)
	a.append(data)
}

// Pack returns the serialised argument buffer ready for
// BOF.Execute. Each call materialises a fresh slice — callers
// can safely mutate the returned bytes without affecting
// subsequent Pack calls, matching the original Args contract.
//
// For a payload that carried a multi-MB AddBytes blob, the
// chunk-list design means peak memory now is
// `len(blob) (caller) + len(packed) (output) = 2x blob` rather
// than the previous 3x (caller + bytes.Buffer + Pack output).
func (a *Args) Pack() []byte {
	out := make([]byte, a.size)
	off := 0
	for _, c := range a.chunks {
		copy(out[off:], c)
		off += len(c)
	}
	return out
}
