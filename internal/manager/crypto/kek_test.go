package crypto

import (
	"bytes"
	"testing"
)

func TestDeriveDeterministic(t *testing.T) {
	salt := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	a := DeriveFromPassphrase("hunter2", salt)
	b := DeriveFromPassphrase("hunter2", salt)
	if !bytes.Equal(a.key[:], b.key[:]) {
		t.Fatal("KEK derivation not deterministic")
	}
}

func TestDeriveDifferentSalt(t *testing.T) {
	a := DeriveFromPassphrase("hunter2", [16]byte{1})
	b := DeriveFromPassphrase("hunter2", [16]byte{2})
	if bytes.Equal(a.key[:], b.key[:]) {
		t.Fatal("different salt should give different key")
	}
}

func TestWipeZeroes(t *testing.T) {
	k := DeriveFromPassphrase("p", [16]byte{0})
	k.Wipe()
	for i, b := range k.key {
		if b != 0 {
			t.Fatalf("key[%d]=%d after Wipe", i, b)
		}
	}
}

func TestGenerateSalt(t *testing.T) {
	a, err := GenerateSalt()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := GenerateSalt()
	if a == b {
		t.Fatal("salt collision (probability ~ 2^-128)")
	}
}
