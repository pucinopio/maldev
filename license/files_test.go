package license

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadPrivateKey(t *testing.T) {
	_, priv, _ := GenerateKey()
	p := filepath.Join(t.TempDir(), "issuer.key")
	if err := SavePrivateKey(p, priv); err != nil {
		t.Fatal(err)
	}
	back, err := LoadPrivateKey(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, priv) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestSavePrivateKeyMode(t *testing.T) {
	if os.Getenv("CI") == "" && os.PathSeparator == '\\' {
		t.Skip("file mode bits not meaningful on Windows host suite")
	}
	_, priv, _ := GenerateKey()
	p := filepath.Join(t.TempDir(), "issuer.key")
	if err := SavePrivateKey(p, priv); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("mode=%o want 0600", mode)
	}
}

func TestSaveLoadPublicKey(t *testing.T) {
	pub, _, _ := GenerateKey()
	p := filepath.Join(t.TempDir(), "issuer.pub")
	if err := SavePublicKey(p, pub, "k2026-05"); err != nil {
		t.Fatal(err)
	}
	back, kid, err := LoadPublicKey(p)
	if err != nil {
		t.Fatal(err)
	}
	if kid != "k2026-05" {
		t.Fatalf("kid=%q", kid)
	}
	if !bytes.Equal(back, pub) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestGenerateAndSave(t *testing.T) {
	dir := t.TempDir()
	pub, priv, err := GenerateAndSave(dir, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != ed25519.PublicKeySize || len(priv) != ed25519.PrivateKeySize {
		t.Fatal("invalid key sizes")
	}
	if _, err := os.Stat(filepath.Join(dir, "issuer.key")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "issuer.pub")); err != nil {
		t.Fatal(err)
	}
	loadedPriv, err := LoadPrivateKey(filepath.Join(dir, "issuer.key"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loadedPriv, priv) {
		t.Fatal("private key disk roundtrip mismatch")
	}
}

func TestSaveLoadLicenseAndVerifyFile(t *testing.T) {
	pub, priv, _ := GenerateKey()
	data, err := Issue(IssueOptions{
		PrivateKey: priv, KeyID: "k1", Subject: "alice",
		NotAfter: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "alice.license")
	if err := SaveLicense(p, data); err != nil {
		t.Fatal(err)
	}
	v, err := VerifyFile(p, Trusted{Keys: map[string]ed25519.PublicKey{"k1": pub}})
	if err != nil {
		t.Fatal(err)
	}
	if v.Subject != "alice" {
		t.Fatalf("subject=%q", v.Subject)
	}
}
