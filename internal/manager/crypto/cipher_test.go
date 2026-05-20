package crypto

import (
	"bytes"
	"errors"
	"testing"
)

func TestWrapUnwrapRoundTrip(t *testing.T) {
	k := DeriveFromPassphrase("p", [16]byte{1})
	plain := []byte("classified config")
	w, err := k.Wrap(plain)
	if err != nil {
		t.Fatal(err)
	}
	got, err := k.Unwrap(w)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("roundtrip mismatch")
	}
}

func TestUnwrapRejectsTampered(t *testing.T) {
	k := DeriveFromPassphrase("p", [16]byte{1})
	w, _ := k.Wrap([]byte("x"))
	w[len(w)-1] ^= 0x01
	if _, err := k.Unwrap(w); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}

func TestUnwrapRejectsWrongKey(t *testing.T) {
	a := DeriveFromPassphrase("p1", [16]byte{1})
	b := DeriveFromPassphrase("p2", [16]byte{1})
	w, _ := a.Wrap([]byte("x"))
	if _, err := b.Unwrap(w); err == nil {
		t.Fatal("wrong KEK accepted")
	}
}

func TestCanaryDetectsWrongPassphrase(t *testing.T) {
	good := DeriveFromPassphrase("p1", [16]byte{1})
	canary, err := NewCanary(good)
	if err != nil {
		t.Fatal(err)
	}
	if !good.VerifyCanary(canary) {
		t.Fatal("good KEK rejected its own canary")
	}
	bad := DeriveFromPassphrase("p2", [16]byte{1})
	if bad.VerifyCanary(canary) {
		t.Fatal("wrong KEK passed canary check")
	}
}

func TestErrIsExported(t *testing.T) {
	k := DeriveFromPassphrase("p", [16]byte{1})
	_, err := k.Unwrap([]byte{1, 2, 3})
	if !errors.Is(err, ErrWrappedFormat) {
		t.Fatalf("err=%v want ErrWrappedFormat", err)
	}
}
