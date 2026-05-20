package license

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestSignPayloadDomainTag(t *testing.T) {
	out := signPayload(tagLicenseV1, []byte("hello"))
	want := append([]byte("maldev-license-v1\x00"), []byte("hello")...)
	if !bytes.Equal(out, want) {
		t.Fatalf("got %x want %x", out, want)
	}
}

func TestHashBytes(t *testing.T) {
	got := sha256Hex([]byte("hello"))
	expect := sha256.Sum256([]byte("hello"))
	if got != hex.EncodeToString(expect[:]) {
		t.Fatalf("hash mismatch")
	}
}
