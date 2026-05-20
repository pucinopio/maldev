package seal

import (
	"bytes"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	pub, priv, err := GenerateRecipient()
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("classified config: don't leak")
	sealed, err := Seal(pub, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(priv, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("roundtrip mismatch")
	}
}

func TestOpenRejectsTampered(t *testing.T) {
	pub, priv, _ := GenerateRecipient()
	sealed, _ := Seal(pub, []byte("data"))
	sealed[len(sealed)-1] ^= 0x01
	if _, err := Open(priv, sealed); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}

func TestOpenRejectsWrongKey(t *testing.T) {
	pub, _, _ := GenerateRecipient()
	_, otherPriv, _ := GenerateRecipient()
	sealed, _ := Seal(pub, []byte("data"))
	if _, err := Open(otherPriv, sealed); err == nil {
		t.Fatal("wrong key accepted")
	}
}
