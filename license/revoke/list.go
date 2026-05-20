package revoke

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/oioio-space/maldev/license/canonical"
)

const (
	pemRevokeBlock = "MALDEV REVOCATION LIST"
	tagRevokeV1    = "maldev-revoke-v1\x00"
)

// List is the body of a revocation publication. Signed under tagRevokeV1.
//
// Revoked is the canonical, wire-compatible flat ID list. Entries carries
// the same IDs plus optional Reason / RevokedAt metadata that issuers can
// use to surface "why" to operators and audit pipelines. When both are set,
// Revoked is the authoritative ID set — Entries is read-only context.
type List struct {
	Version    int       `json:"v"`
	KeyID      string    `json:"kid"`
	Sequence   uint64    `json:"seq"`
	PrevHash   []byte    `json:"prev,omitempty"`
	IssuedAt   time.Time `json:"iat"`
	ExpiresAt  time.Time `json:"exp"`
	ServerTime time.Time `json:"st"`
	Revoked    []string  `json:"rev"`
	Entries    []Entry   `json:"ent,omitempty"`
}

// Entry is the richer per-revocation record. Reason is free-form admin text
// (e.g. "refunded", "fraud", "key compromised"). RevokedAt is a pointer so
// the zero time.Time is genuinely omitted from the wire (json's
// omitempty does not suppress zero time.Time values).
type Entry struct {
	ID        string     `json:"id"`
	Reason    string     `json:"r,omitempty"`
	RevokedAt *time.Time `json:"at,omitempty"`
}

// IsRevoked reports whether id is on the list. The lookup is a linear scan
// — fine for the operational expectation (handful of revocations) and
// timing-side-channel exposure here is negligible since the caller is the
// verifying binary on a machine it already owns.
func (l *List) IsRevoked(id string) bool {
	for _, r := range l.Revoked {
		if r == id {
			return true
		}
	}
	return false
}

// LookupEntry returns the detailed record for id if Entries was populated.
// Returns (nil, false) if id is not present or if the publisher did not
// include extended metadata. Callers should still consult IsRevoked for the
// authoritative accept/refuse decision.
func (l *List) LookupEntry(id string) (*Entry, bool) {
	for i := range l.Entries {
		if l.Entries[i].ID == id {
			return &l.Entries[i], true
		}
	}
	return nil, false
}

type signedList struct {
	List      List   `json:"lst"`
	Signature []byte `json:"sig"`
	KeyID     string `json:"kid"`
}

func Sign(l List, priv ed25519.PrivateKey) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("revoke: invalid private key")
	}
	if l.Version == 0 {
		l.Version = 1
	}
	if l.IssuedAt.IsZero() {
		l.IssuedAt = time.Now().UTC()
	}
	body, err := canonical.Marshal(l)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(priv, append([]byte(tagRevokeV1), body...))
	wrapped, err := canonical.Marshal(signedList{List: l, Signature: sig, KeyID: l.KeyID})
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  pemRevokeBlock,
		Bytes: []byte(base64.StdEncoding.EncodeToString(wrapped)),
	}), nil
}

func VerifyBytes(data []byte, pub ed25519.PublicKey, expectedKID string) (*List, error) {
	blk, _ := pem.Decode(data)
	if blk == nil || blk.Type != pemRevokeBlock {
		return nil, errors.New("revoke: not a revocation list PEM")
	}
	raw, err := base64.StdEncoding.DecodeString(string(blk.Bytes))
	if err != nil {
		return nil, fmt.Errorf("revoke: base64: %w", err)
	}
	var w signedList
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		return nil, fmt.Errorf("revoke: json: %w", err)
	}
	if expectedKID != "" && w.KeyID != expectedKID {
		return nil, errors.New("revoke: kid mismatch")
	}
	body, err := canonical.Marshal(w.List)
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(pub, append([]byte(tagRevokeV1), body...), w.Signature) {
		return nil, errors.New("revoke: signature invalid")
	}
	return &w.List, nil
}
