package license

import (
	"bytes"
	"crypto/ed25519"
	"testing"
)

func TestGenerateKeyRoundTrip(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("payload")
	sig := ed25519.Sign(priv, msg)
	if !ed25519.Verify(pub, msg, sig) {
		t.Fatal("generated keypair fails self verify")
	}
}

func TestMarshalParsePrivateKey(t *testing.T) {
	_, priv, _ := GenerateKey()
	pemBytes, err := MarshalPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(pemBytes, []byte("MALDEV PRIVATE KEY")) {
		t.Fatalf("PEM block label missing: %s", pemBytes)
	}
	back, err := ParsePrivateKey(pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, priv) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestMarshalParsePublicKeyWithKID(t *testing.T) {
	pub, _, _ := GenerateKey()
	pemBytes, err := MarshalPublicKey(pub, "k2026-05")
	if err != nil {
		t.Fatal(err)
	}
	backPub, kid, err := ParsePublicKey(pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	if kid != "k2026-05" {
		t.Fatalf("kid lost: %q", kid)
	}
	if !bytes.Equal(backPub, pub) {
		t.Fatal("pub roundtrip mismatch")
	}
}

func TestParseRejectsWrongBlock(t *testing.T) {
	bogus := []byte("-----BEGIN OTHER-----\nAAAA\n-----END OTHER-----\n")
	if _, err := ParsePrivateKey(bogus); err == nil {
		t.Fatal("expected error on wrong PEM type")
	}
}

func TestTrustedLookup(t *testing.T) {
	pub, _, _ := GenerateKey()
	tr := Trusted{Keys: map[string]ed25519.PublicKey{"k1": pub}}
	if _, ok := tr.Lookup("k1"); !ok {
		t.Fatal("expected k1 to be present")
	}
	if _, ok := tr.Lookup("k2"); ok {
		t.Fatal("expected k2 to be absent")
	}
}
