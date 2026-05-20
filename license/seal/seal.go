// Package seal encrypts opaque payloads to a recipient identified by an
// X25519 public key. Used for License.SealedPayload — the license is signed
// publicly but the sealed segment is readable only by the holder of the
// recipient X25519 private key.
package seal

import (
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

const ephPubLen = 32

// GenerateRecipient returns a fresh X25519 keypair (pub, priv).
func GenerateRecipient() ([]byte, []byte, error) {
	priv := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, priv); err != nil {
		return nil, nil, err
	}
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}
	return pub, priv, nil
}

// Seal encrypts plaintext to recipientPub using an ephemeral X25519 key exchange
// and XChaCha20-Poly1305 AEAD. The ephemeral public key is authenticated as AAD
// so the ciphertext is bound to the specific key exchange.
func Seal(recipientPub, plaintext []byte) ([]byte, error) {
	if len(recipientPub) != 32 {
		return nil, errors.New("seal: recipientPub must be 32 bytes")
	}
	ephPriv := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, ephPriv); err != nil {
		return nil, err
	}
	ephPub, err := curve25519.X25519(ephPriv, curve25519.Basepoint)
	if err != nil {
		return nil, err
	}
	shared, err := curve25519.X25519(ephPriv, recipientPub)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.NewX(shared)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := aead.Seal(nil, nonce, plaintext, ephPub)
	out := make([]byte, 0, ephPubLen+len(nonce)+len(ct))
	out = append(out, ephPub...)
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

// Open decrypts a sealed payload using the recipient's X25519 private key.
// Returns an error if the payload is malformed, tampered, or sealed to a
// different key.
func Open(recipientPriv, sealed []byte) ([]byte, error) {
	if len(recipientPriv) != 32 || len(sealed) < ephPubLen+24+16 {
		return nil, errors.New("seal: malformed sealed payload")
	}
	ephPub := sealed[:ephPubLen]
	shared, err := curve25519.X25519(recipientPriv, ephPub)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.NewX(shared)
	if err != nil {
		return nil, err
	}
	nonceSize := aead.NonceSize()
	nonce := sealed[ephPubLen : ephPubLen+nonceSize]
	ct := sealed[ephPubLen+nonceSize:]
	return aead.Open(nil, nonce, ct, ephPub)
}
