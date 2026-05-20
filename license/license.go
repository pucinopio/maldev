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

	// Features is a flat list of entitlement names signed at the top level
	// of the license body. Tools that gate behaviour by tier/feature can
	// check it with Verified.HasFeature without deserialising Payload.
	Features []string `json:"feat,omitempty"`

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
	// Params carries algorithm parameters embedded next to the binding so
	// the issuer can re-tune (e.g. argon2id time/memory/threads) without a
	// flag day for existing licences. Nil = use the package defaults.
	Params *BindingParams `json:"p,omitempty"`
}

// BindingParams stores algorithm tuning next to a Binding. Only the fields
// relevant to the Binding.Type are populated. Forward-compatible: unknown
// fields are ignored.
type BindingParams struct {
	// Argon2id parameters for Type=="password".
	ArgonTime    uint32 `json:"at,omitempty"`
	ArgonMemory  uint32 `json:"am,omitempty"`
	ArgonThreads uint8  `json:"ap,omitempty"`
	ArgonKeyLen  uint32 `json:"akl,omitempty"`
}

// Verified is the result of a successful Verify.
type Verified struct {
	License
	Payload  []byte
	KeyUsed  string
	Warnings []string
}

// HasFeature reports whether the licence grants name. Comparison is
// case-sensitive — pick a convention at issue time (kebab-case is common).
// Defined on License (rather than Verified) so callers inspecting a parsed
// licence without going through Verify can use the same lookup.
func (l *License) HasFeature(name string) bool {
	for _, f := range l.Features {
		if f == name {
			return true
		}
	}
	return false
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

	// Features is signed into the license body at the top level. See
	// Verified.HasFeature for verification-side lookup.
	Features []string

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
