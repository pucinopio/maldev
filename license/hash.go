package license

import (
	"crypto/sha256"
	"encoding/hex"
)

// Domain tags ensure a signature over one message type can never be replayed
// as a signature over another. Each tag is appended verbatim (with the NUL
// terminator) to the canonical bytes before Ed25519 signing/verification.
const (
	tagLicenseV1   = "maldev-license-v1\x00"
	tagRevokeV1    = "maldev-revoke-v1\x00"
	tagHeartbeatV1 = "maldev-heartbeat-v1\x00"
	tagStateV1     = "maldev-state-v1\x00"
)

func signPayload(tag string, canonical []byte) []byte {
	out := make([]byte, 0, len(tag)+len(canonical))
	out = append(out, tag...)
	out = append(out, canonical...)
	return out
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hexEncode(b []byte) string {
	return hex.EncodeToString(b)
}
