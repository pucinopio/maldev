package identity

import "testing"

func TestSetReadRoundTrip(t *testing.T) {
	Set([]byte{1, 2, 3, 4})
	if got := Read(); string(got) != "\x01\x02\x03\x04" {
		t.Fatalf("got %x", got)
	}
}

func TestHashIdentityHex(t *testing.T) {
	h := HashIdentity([]byte("abc"))
	if len(h) != 64 {
		t.Fatalf("len=%d", len(h))
	}
}
