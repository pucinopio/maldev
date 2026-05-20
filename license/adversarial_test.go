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
	origInner, ok := decodeInnerForTest(data)
	if !ok {
		t.Fatal("could not decode original PEM")
	}

	rng := rand.New(rand.NewSource(42))
	accepted, evaluated := 0, 0
	for i := 0; i < 100; i++ {
		cp := append([]byte(nil), data...)
		start := bytes.Index(cp, []byte("\n")) + 1
		end := bytes.LastIndex(cp, []byte("-----END"))
		if start <= 0 || end <= 0 || end <= start {
			t.Fatal("bad PEM")
		}
		cp[start+rng.Intn(end-start)] ^= byte(1 << uint(rng.Intn(8)))

		// Mutations that hit a PEM newline get absorbed by pem.Decode, and
		// some base64 quantisations are no-ops. These produce the same inner
		// bytes as the original — they are not real attacks, so skip them.
		if newInner, ok := decodeInnerForTest(cp); ok && bytes.Equal(newInner, origInner) {
			continue
		}
		evaluated++
		if _, err := Verify(cp, trustedFor(pub, "k1")); err == nil {
			accepted++
		}
	}
	if accepted > 0 {
		t.Fatalf("%d/%d real mutations accepted — signature/format check too lax", accepted, evaluated)
	}
	if evaluated == 0 {
		t.Fatal("no real mutations evaluated — test loop is not exercising the signature path")
	}
}

func decodeInnerForTest(data []byte) ([]byte, bool) {
	blk, _ := pem.Decode(data)
	if blk == nil || blk.Type != pemLicense {
		return nil, false
	}
	raw, err := base64.StdEncoding.DecodeString(string(blk.Bytes))
	if err != nil {
		return nil, false
	}
	return raw, true
}
