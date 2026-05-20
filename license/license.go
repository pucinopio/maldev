package license

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"time"
)

// License is the body of a maldev license. Every field is signed.
type License struct {
	Version int    `json:"v"`
	ID      string `json:"id"`
	KeyID   string `json:"kid"`

	Issuer   string   `json:"iss,omitempty"`
	Subject  string   `json:"sub"`
	Audience []string `json:"aud,omitempty"`

	IssuedAt  time.Time `json:"iat"`
	NotBefore time.Time `json:"nbf,omitempty"`
	NotAfter  time.Time `json:"exp,omitempty"`

	Bindings []Binding `json:"bnd,omitempty"`

	BinarySHA256   string `json:"bin,omitempty"`
	IdentitySHA256 string `json:"id_sha,omitempty"`

	Payload       json.RawMessage `json:"pld,omitempty"`
	SealedPayload []byte          `json:"spld,omitempty"`
}

// Binding is a single constraint that must be matched by caller-provided
// evidence at Verify time. Field semantics are defined in bindings.go.
type Binding struct {
	Type  string   `json:"t"`
	Value []string `json:"v,omitempty"`
	Hash  []byte   `json:"h,omitempty"`
	Salt  []byte   `json:"s,omitempty"`
}

// Verified is the result of a successful Verify.
type Verified struct {
	License
	Payload  []byte
	KeyUsed  string
	Warnings []string
}

// signedLicense is the wire wrapper PEM-encoded on disk. KeyID is duplicated
// here so a verifier can pick the right key before parsing the body.
type signedLicense struct {
	License   License `json:"lic"`
	Signature []byte  `json:"sig"`
	KeyID     string  `json:"kid"`
}

// MaxLicenseSize bounds the PEM input accepted by Verify.
const MaxLicenseSize = 16 * 1024

// IssueOptions configures Issue.
type IssueOptions struct {
	PrivateKey ed25519.PrivateKey
	KeyID      string

	Issuer   string
	Subject  string
	Audience []string

	NotBefore time.Time
	NotAfter  time.Time

	Bindings []Binding

	BinarySHA256   string
	IdentitySHA256 string

	Payload       json.RawMessage
	SealedPayload []byte
}

func jsonUnmarshalStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
