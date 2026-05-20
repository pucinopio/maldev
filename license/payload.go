package license

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNoPayload is returned by Decode and PayloadAs when the verified license
// carries no application payload. Discriminate with errors.Is.
var ErrNoPayload = errors.New("license: no payload")

// MarshalPayload encodes v to JSON suitable for IssueOptions.Payload. Any
// JSON-marshalable Go value is accepted (structs with json tags, maps,
// primitives). The same value round-trips through (*Verified).Decode or
// PayloadAs on the verification side.
func MarshalPayload(v any) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("license: marshal payload: %w", err)
	}
	return data, nil
}

// Decode unmarshals the verified payload into target, which must be a pointer
// (typically &myStruct). Returns ErrNoPayload if the license carries no
// payload, or a JSON error if the bytes don't match target's shape.
//
// This is the non-generic counterpart to PayloadAs. Use it when target's
// type is not known at the call site (e.g. you have an interface).
func (v *Verified) Decode(target any) error {
	if len(v.Payload) == 0 {
		return ErrNoPayload
	}
	if err := json.Unmarshal(v.Payload, target); err != nil {
		return fmt.Errorf("license: decode payload: %w", err)
	}
	return nil
}

// PayloadAs is the generic decoder: it returns the verified payload as *T,
// removing the need to declare a variable beforehand.
//
//	cfg, err := license.PayloadAs[MyConfig](v)
//	if err != nil { ... }
//	use(cfg.Field)
//
// Returns ErrNoPayload if the license carries no payload.
func PayloadAs[T any](v *Verified) (*T, error) {
	if len(v.Payload) == 0 {
		return nil, ErrNoPayload
	}
	var out T
	if err := json.Unmarshal(v.Payload, &out); err != nil {
		return nil, fmt.Errorf("license: decode payload as %T: %w", out, err)
	}
	return &out, nil
}
