// Package crypto provides passphrase-derived key wrapping for the license
// manager's at-rest secrets. KEK = Argon2id(passphrase, salt). Wrap/Unwrap
// uses ChaCha20-Poly1305 with a random 12-byte nonce per blob.
package crypto

import (
	"crypto/rand"

	"golang.org/x/crypto/argon2"

	"github.com/oioio-space/maldev/cleanup/memory"
)

const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 4
	keyLen       uint32 = 32
)

// KEK is the symmetric key derived from the operator's passphrase. Never
// persist this value — only the wrapped blobs and the salt land on disk.
type KEK struct {
	key [32]byte
}

// DeriveFromPassphrase computes Argon2id(passphrase, salt). Same passphrase
// + same salt always yields the same KEK, so an existing DB can be reopened.
func DeriveFromPassphrase(passphrase string, salt [16]byte) *KEK {
	out := argon2.IDKey([]byte(passphrase), salt[:], argonTime, argonMemory, argonThreads, keyLen)
	var k KEK
	copy(k.key[:], out)
	memory.SecureZero(out)
	return &k
}

// Wipe zeroes the key bytes. Call on clean shutdown so a memory snapshot
// after exit doesn't reveal the KEK.
func (k *KEK) Wipe() {
	memory.SecureZero(k.key[:])
}

// GenerateSalt returns a fresh 16-byte random salt for a new DB.
func GenerateSalt() ([16]byte, error) {
	var s [16]byte
	if _, err := rand.Read(s[:]); err != nil {
		return s, err
	}
	return s, nil
}
