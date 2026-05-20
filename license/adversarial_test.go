package license

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/rand"
	"testing"
	"time"
)

func TestAdversarial_SingleBitFlipRejected(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})
	blk, _ := pem.Decode(data)
	inner, _ := base64.StdEncoding.DecodeString(string(blk.Bytes))
	i := bytes.Index(inner, []byte(`"sub"`))
	if i < 0 {
		t.Fatal("could not locate sub field")
	}
	inner[i+2] ^= 0x01
	blk.Bytes = []byte(base64.StdEncoding.EncodeToString(inner))
	tampered := pem.EncodeToMemory(blk)
	if _, err := Verify(tampered, trustedFor(pub, "k1")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("tampered license accepted")
	}
}

func TestAdversarial_HugeLicenseRejectedBeforeParse(t *testing.T) {
	blob := bytes.Repeat([]byte("A"), MaxLicenseSize+1)
	if _, err := Verify(blob, Trusted{}); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("oversize license accepted")
	}
}

func TestAdversarial_SwappedKeyIDRejected(t *testing.T) {
	pubA, privA, _ := GenerateKey()
	_, privB, _ := GenerateKey()
	_ = privA // we only need pubA as the trusted key the attacker doesn't possess
	// Issue with privB but claim KeyID "kA".
	data, err := Issue(IssueOptions{PrivateKey: privB, KeyID: "kA", Subject: "x", NotAfter: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	// Verify under Trusted{kA: pubA}.
	if _, err := Verify(data, Trusted{Keys: map[string]ed25519.PublicKey{"kA": pubA}}); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("substituted-key license accepted")
	}
}

func TestAdversarial_RandomByteMutation(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})
	rng := rand.New(rand.NewSource(42))
	accepted := 0
	for i := 0; i < 100; i++ {
		cp := append([]byte(nil), data...)
		start := bytes.Index(cp, []byte("\n")) + 1
		end := bytes.LastIndex(cp, []byte("-----END"))
		if start <= 0 || end <= 0 || end <= start {
			t.Fatal("bad PEM")
		}
		cp[start+rng.Intn(end-start)] ^= byte(1 << uint(rng.Intn(8)))
		if _, err := Verify(cp, trustedFor(pub, "k1")); err == nil {
			accepted++
		}
	}
	if accepted > 0 {
		t.Fatalf("%d/100 mutations accepted — signature/format check too lax", accepted)
	}
}
