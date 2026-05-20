// Package identity holds a 32-byte build-time identity registered by the
// consumer binary (typically via //go:embed identity.bin and a call to Set).
// The identity survives binary packing because packers preserve embedded data.
package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

var (
	mu  sync.RWMutex
	val []byte
)

// Set registers the embedded identity bytes. Call once at init.
func Set(b []byte) {
	mu.Lock()
	val = append([]byte(nil), b...)
	mu.Unlock()
}

// Read returns the registered identity bytes (nil if Set has not been called).
func Read() []byte {
	mu.RLock()
	defer mu.RUnlock()
	return append([]byte(nil), val...)
}

// HashIdentity returns the hex sha256 of arbitrary identity bytes.
func HashIdentity(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
