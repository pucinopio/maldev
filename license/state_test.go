package license

import (
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state")
	key := []byte("hmac-key-32-bytes-long-enough....")[:32]
	in := State{
		TrustedFloor:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LastSeenLocal: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}
	if err := writeState(p, key, in); err != nil {
		t.Fatal(err)
	}
	out, err := readState(p, key)
	if err != nil {
		t.Fatal(err)
	}
	if !out.TrustedFloor.Equal(in.TrustedFloor) || !out.LastSeenLocal.Equal(in.LastSeenLocal) {
		t.Fatalf("roundtrip mismatch: %+v vs %+v", in, out)
	}
}

func TestStateRejectsTamperedHMAC(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state")
	key := []byte("hmac-key-32-bytes-long-enough....")[:32]
	_ = writeState(p, key, State{TrustedFloor: time.Now().UTC()})
	raw, _ := os.ReadFile(p)
	raw[len(raw)/2] ^= 0xFF
	_ = os.WriteFile(p, raw, 0o600)
	if _, err := readState(p, key); err == nil {
		t.Fatal("expected HMAC failure")
	}
}

func TestVerifyDetectsClockRollback(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "license-state")

	pub, priv, _ := GenerateKey()
	data, err := Issue(IssueOptions{
		PrivateKey: priv, KeyID: "k1", Subject: "test",
		NotAfter: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	tr := Trusted{Keys: map[string]ed25519.PublicKey{"k1": pub}}

	// First Verify (clock = 2026-06-01).
	clk := &FakeClock{T: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}
	if _, err := Verify(data, tr, WithClock(clk), WithStateFile(statePath), WithMaxClockSkew(time.Minute)); err != nil {
		t.Fatalf("first verify failed: %v", err)
	}

	// Second Verify with clock rolled back to 2026-01-01.
	clk.T = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := Verify(data, tr, WithClock(clk), WithStateFile(statePath), WithMaxClockSkew(time.Minute)); !errors.Is(err, ErrLicenseInvalid) {
		t.Fatalf("rollback should reject, got %v", err)
	}
}
