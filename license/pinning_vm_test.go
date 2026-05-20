//go:build vmtest

package license

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oioio-space/maldev/license/identity"
)

// TestBinaryPinning_HashFileStable asserts HashFile is stable across two
// reads of the same path. Sanity test, low-risk on any VM.
func TestBinaryPinning_HashFileStable(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bin")
	if err := os.WriteFile(tmp, []byte("payload-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := HashFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("HashFile not stable: %s vs %s", a, b)
	}
}

// TestIdentityPinning_RoundTrip exercises the full identity-pinning Verify
// path. Independent of the packer; the spec also calls for a packer-based VM
// variant that is deferred until cmd/packer integration in a later task.
func TestIdentityPinning_RoundTrip(t *testing.T) {
	seed := []byte("identity-seed-32-bytes-of-data..")
	identity.Set(seed)
	pub, priv, _ := GenerateKey()
	data, err := Issue(IssueOptions{
		PrivateKey:     priv,
		KeyID:          "k1",
		Subject:        "vm-test",
		NotAfter:       time.Now().Add(time.Hour),
		IdentitySHA256: identity.HashIdentity(seed),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(data, Trusted{Keys: map[string]ed25519.PublicKey{"k1": pub}},
		WithBinaryPinning(),
	); err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
}
