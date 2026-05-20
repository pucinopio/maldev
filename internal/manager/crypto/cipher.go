package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

// ErrWrappedFormat is returned when Unwrap receives bytes that aren't a
// well-formed wrap: [12-byte nonce][ciphertext][16-byte tag].
var ErrWrappedFormat = errors.New("crypto: wrapped blob has bad length")

const nonceLen = 12

// Wrap encrypts plain under the KEK with a fresh random nonce. Format on
// disk is nonce || ciphertext || tag.
func (k *KEK) Wrap(plain []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(k.key[:])
	if err != nil {
		return nil, fmt.Errorf("crypto: build aead: %w", err)
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, nonceLen+len(plain)+aead.Overhead())
	out = append(out, nonce...)
	return aead.Seal(out, nonce, plain, nil), nil
}

// Unwrap reverses Wrap. Returns ErrWrappedFormat for blobs that are too
// short to even contain a nonce + tag.
func (k *KEK) Unwrap(wrapped []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(k.key[:])
	if err != nil {
		return nil, err
	}
	if len(wrapped) < nonceLen+aead.Overhead() {
		return nil, ErrWrappedFormat
	}
	nonce := wrapped[:nonceLen]
	ct := wrapped[nonceLen:]
	return aead.Open(nil, nonce, ct, nil)
}

// NewCanary wraps a 32-byte random payload under the KEK. Store it once at
// DB creation; verify with KEK.VerifyCanary on every boot.
func NewCanary(k *KEK) ([]byte, error) {
	plain := make([]byte, 32)
	if _, err := rand.Read(plain); err != nil {
		return nil, err
	}
	return k.Wrap(plain)
}

// VerifyCanary returns true iff the KEK can decrypt the canary. A false
// result is the definitive "wrong passphrase" signal at boot.
func (k *KEK) VerifyCanary(canary []byte) bool {
	_, err := k.Unwrap(canary)
	return err == nil
}
