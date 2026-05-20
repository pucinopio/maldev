package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/oioio-space/maldev/license/canonical"
)

// New issues a license with sensible defaults. ttl <= 0 means no expiry.
func New(priv ed25519.PrivateKey, subject string, ttl time.Duration) ([]byte, error) {
	opts := IssueOptions{PrivateKey: priv, Subject: subject}
	if ttl > 0 {
		opts.NotAfter = time.Now().Add(ttl).UTC()
	}
	return Issue(opts)
}

// Issue signs a new license and returns the PEM-armored bytes.
func Issue(opts IssueOptions) ([]byte, error) {
	if len(opts.PrivateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("license: IssueOptions.PrivateKey missing or invalid")
	}
	if opts.Subject == "" {
		return nil, errors.New("license: IssueOptions.Subject required")
	}
	kid := opts.KeyID
	if kid == "" {
		kid = "default"
	}
	id, err := newUUIDv4()
	if err != nil {
		return nil, err
	}
	lic := License{
		Version:        1,
		ID:             id,
		KeyID:          kid,
		Issuer:         opts.Issuer,
		Subject:        opts.Subject,
		Audience:       opts.Audience,
		IssuedAt:       time.Now().UTC(),
		NotBefore:      opts.NotBefore.UTC(),
		NotAfter:       opts.NotAfter.UTC(),
		Bindings:       opts.Bindings,
		BinarySHA256:   opts.BinarySHA256,
		IdentitySHA256: opts.IdentitySHA256,
		Payload:        opts.Payload,
		SealedPayload:  opts.SealedPayload,
	}
	body, err := canonical.Marshal(lic)
	if err != nil {
		return nil, fmt.Errorf("license: canonicalise body: %w", err)
	}
	sig := ed25519.Sign(opts.PrivateKey, signPayload(tagLicenseV1, body))
	wrapped := signedLicense{License: lic, Signature: sig, KeyID: kid}
	wbytes, err := canonical.Marshal(wrapped)
	if err != nil {
		return nil, fmt.Errorf("license: canonicalise wrapper: %w", err)
	}
	block := &pem.Block{
		Type:  pemLicense,
		Bytes: []byte(base64.StdEncoding.EncodeToString(wbytes)),
	}
	return pem.EncodeToMemory(block), nil
}

// Inspect parses a PEM-armored license without verifying its signature. Use
// for diagnostics only — never trust the returned License for authorisation.
func Inspect(data []byte) (*License, error) {
	if len(data) > MaxLicenseSize {
		return nil, invalid(causeBadFormat)
	}
	blk, _ := pem.Decode(data)
	if blk == nil || blk.Type != pemLicense {
		return nil, invalid(causeBadFormat)
	}
	raw, err := base64.StdEncoding.DecodeString(string(blk.Bytes))
	if err != nil {
		return nil, invalid(causeBadFormat)
	}
	var w signedLicense
	if err := jsonUnmarshalStrict(raw, &w); err != nil {
		return nil, invalid(causeBadFormat)
	}
	return &w.License, nil
}

func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
