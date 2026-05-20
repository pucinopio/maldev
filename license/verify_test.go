package license

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"
)

func issueFor(t *testing.T, opts IssueOptions) ([]byte, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, _ := GenerateKey()
	opts.PrivateKey = priv
	if opts.KeyID == "" {
		opts.KeyID = "k1"
	}
	if opts.Subject == "" {
		opts.Subject = "test-sub"
	}
	data, err := Issue(opts)
	if err != nil {
		t.Fatal(err)
	}
	return data, pub, priv
}

func trustedFor(pub ed25519.PublicKey, kid string) Trusted {
	return Trusted{Keys: map[string]ed25519.PublicKey{kid: pub}}
}

func TestVerifyOfflineHappyPath(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})
	v, err := Verify(data, trustedFor(pub, "k1"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Subject != "test-sub" {
		t.Fatalf("subject=%q", v.Subject)
	}
	if v.KeyUsed != "k1" {
		t.Fatalf("KeyUsed=%q", v.KeyUsed)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(-time.Hour)})
	if _, err := Verify(data, trustedFor(pub, "k1")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatalf("expected ErrLicenseInvalid, got %v", err)
	}
}

func TestVerifyRejectsNotYetValid(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{NotBefore: time.Now().Add(time.Hour)})
	if _, err := Verify(data, trustedFor(pub, "k1")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("expected rejection")
	}
}

func TestVerifyRejectsUnknownKey(t *testing.T) {
	data, _, _ := issueFor(t, IssueOptions{KeyID: "kZ"})
	otherPub, _, _ := GenerateKey()
	if _, err := Verify(data, trustedFor(otherPub, "k1")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("expected rejection")
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})
	// Mutate a byte well inside the base64 payload to break the signed body.
	data[80] ^= 0x01
	if _, err := Verify(data, trustedFor(pub, "k1")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("tampered license accepted")
	}
}

func TestVerifyAudienceMatch(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter: time.Now().Add(time.Hour),
		Audience: []string{"rshell"},
	})
	if _, err := Verify(data, trustedFor(pub, "k1"), WithAudience("rshell")); err != nil {
		t.Fatalf("expected accept, got %v", err)
	}
	if _, err := Verify(data, trustedFor(pub, "k1"), WithAudience("memscan")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("audience mismatch should reject")
	}
}

func TestVerifyIssuerMatch(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter: time.Now().Add(time.Hour),
		Issuer:   "lab-eu",
	})
	if _, err := Verify(data, trustedFor(pub, "k1"), WithIssuer("lab-eu")); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(data, trustedFor(pub, "k1"), WithIssuer("lab-us")); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatal("issuer mismatch should reject")
	}
}

func TestVerifyClockSkewTolerated(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(-30 * time.Second)})
	if _, err := Verify(data, trustedFor(pub, "k1"), WithMaxClockSkew(time.Minute)); err != nil {
		t.Fatalf("expected tolerance, got %v", err)
	}
}
