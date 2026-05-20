package license

import (
	"bytes"
	"testing"
)

func TestSignPayloadDomainTag(t *testing.T) {
	out := signPayload(tagLicenseV1, []byte("hello"))
	want := append([]byte("maldev-license-v1\x00"), []byte("hello")...)
	if !bytes.Equal(out, want) {
		t.Fatalf("got %x want %x", out, want)
	}
}
