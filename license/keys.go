package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
)

// GenerateKey returns a fresh Ed25519 keypair from crypto/rand.
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("license: generate key: %w", err)
	}
	return pub, priv, nil
}

// Trusted is the set of public keys a Verify call accepts, keyed by KeyID.
// Permits rotation: keep old keys until all licences they signed expire.
type Trusted struct {
	Keys map[string]ed25519.PublicKey
}

func (t Trusted) Lookup(kid string) (ed25519.PublicKey, bool) {
	if t.Keys == nil {
		return nil, false
	}
	k, ok := t.Keys[kid]
	return k, ok
}

// SingleKey builds a Trusted.Keys map for the common case of one trusted key.
// Equivalent to: map[string]ed25519.PublicKey{kid: pub}.
func SingleKey(kid string, pub ed25519.PublicKey) map[string]ed25519.PublicKey {
	return map[string]ed25519.PublicKey{kid: pub}
}
