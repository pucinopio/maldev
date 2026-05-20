package license

import (
	"crypto/ed25519"
	"encoding/pem"
	"errors"
	"fmt"
)

const (
	pemPrivateKey = "MALDEV PRIVATE KEY"
	pemPublicKey  = "MALDEV PUBLIC KEY"
	pemLicense    = "MALDEV LICENSE"
	pemRevoke     = "MALDEV REVOCATION LIST"
)

func MarshalPrivateKey(priv ed25519.PrivateKey) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("license: invalid private key length")
	}
	b := &pem.Block{Type: pemPrivateKey, Bytes: priv}
	return pem.EncodeToMemory(b), nil
}

func ParsePrivateKey(data []byte) (ed25519.PrivateKey, error) {
	blk, _ := pem.Decode(data)
	if blk == nil {
		return nil, errors.New("license: no PEM block found")
	}
	if blk.Type != pemPrivateKey {
		return nil, fmt.Errorf("license: wrong PEM type %q", blk.Type)
	}
	if len(blk.Bytes) != ed25519.PrivateKeySize {
		return nil, errors.New("license: invalid private key length")
	}
	return ed25519.PrivateKey(blk.Bytes), nil
}

func MarshalPublicKey(pub ed25519.PublicKey, kid string) ([]byte, error) {
	if len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("license: invalid public key length")
	}
	var headers map[string]string
	if kid != "" {
		headers = map[string]string{"KID": kid}
	}
	b := &pem.Block{Type: pemPublicKey, Headers: headers, Bytes: pub}
	return pem.EncodeToMemory(b), nil
}

func ParsePublicKey(data []byte) (ed25519.PublicKey, string, error) {
	blk, _ := pem.Decode(data)
	if blk == nil {
		return nil, "", errors.New("license: no PEM block found")
	}
	if blk.Type != pemPublicKey {
		return nil, "", fmt.Errorf("license: wrong PEM type %q", blk.Type)
	}
	if len(blk.Bytes) != ed25519.PublicKeySize {
		return nil, "", errors.New("license: invalid public key length")
	}
	return ed25519.PublicKey(blk.Bytes), blk.Headers["KID"], nil
}
