package license

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"

	"github.com/oioio-space/maldev/license/internal/fileutil"
)

// File-based helpers. Every cryptographic primitive in this package also has
// a bytes-in / bytes-out variant (MarshalPrivateKey, ParsePrivateKey, Issue,
// Verify, …) — these helpers exist to remove the os.ReadFile/os.WriteFile
// boilerplate when you already know you want a file on disk.

// SavePrivateKey writes priv as a PEM "MALDEV PRIVATE KEY" block to path.
// The file is written atomically with mode 0o600 (owner read/write only).
// Parent directories are created with mode 0o700 if absent.
func SavePrivateKey(path string, priv ed25519.PrivateKey) error {
	pemBytes, err := MarshalPrivateKey(priv)
	if err != nil {
		return err
	}
	if err := fileutil.AtomicWrite(path, ".key-*.tmp", pemBytes); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// LoadPrivateKey reads and parses a PEM private key from path.
func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("license: load private key: %w", err)
	}
	return ParsePrivateKey(data)
}

// SavePublicKey writes pub as a PEM "MALDEV PUBLIC KEY" block to path. The
// KeyID (kid) is stored in the PEM header for later retrieval via
// LoadPublicKey. Use kid="" to omit.
func SavePublicKey(path string, pub ed25519.PublicKey, kid string) error {
	pemBytes, err := MarshalPublicKey(pub, kid)
	if err != nil {
		return err
	}
	return fileutil.AtomicWrite(path, ".pub-*.tmp", pemBytes)
}

// LoadPublicKey reads a PEM public key from path and returns the key plus the
// KeyID stored in the PEM header (empty if not present).
func LoadPublicKey(path string) (ed25519.PublicKey, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("license: load public key: %w", err)
	}
	return ParsePublicKey(data)
}

// GenerateAndSave generates a fresh Ed25519 keypair, saves both halves under
// dir as "issuer.key" (private, 0o600) and "issuer.pub" (public, 0o644), and
// tags the public key with the given KeyID.
//
// Returns the in-memory copies so callers can use them immediately without a
// second filesystem round trip.
func GenerateAndSave(dir, kid string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := GenerateKey()
	if err != nil {
		return nil, nil, err
	}
	if err := SavePrivateKey(filepath.Join(dir, "issuer.key"), priv); err != nil {
		return nil, nil, err
	}
	if err := SavePublicKey(filepath.Join(dir, "issuer.pub"), pub, kid); err != nil {
		return nil, nil, err
	}
	return pub, priv, nil
}

// SaveLicense writes a PEM-armored license to path with mode 0o644.
func SaveLicense(path string, data []byte) error {
	return fileutil.AtomicWrite(path, ".lic-*.tmp", data)
}

// LoadLicense reads a PEM-armored license from path. Returns the raw bytes
// suitable for passing to Verify.
func LoadLicense(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("license: load license: %w", err)
	}
	return data, nil
}

// VerifyFile is a convenience wrapper that reads a license from path and
// passes it to Verify with the provided options.
func VerifyFile(path string, trusted Trusted, opts ...VerifyOption) (*Verified, error) {
	data, err := LoadLicense(path)
	if err != nil {
		return nil, err
	}
	return Verify(data, trusted, opts...)
}
