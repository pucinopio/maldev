package license

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// State is the local cross-invocation memory of Verify. Persisted under
// HMAC; key is derived in deriveStateKey.
type State struct {
	TrustedFloor     time.Time `json:"tf"`
	LastSeenLocal    time.Time `json:"lsl"`
	LastSeenSequence uint64    `json:"lss"`
	LastFetchOk      time.Time `json:"lfo"`
	LastHeartbeatOk  time.Time `json:"lho"`
}

type stateEnvelope struct {
	Body []byte `json:"b"`
	HMAC []byte `json:"m"`
}

func writeState(path string, key []byte, s State) error {
	body, err := json.Marshal(s)
	if err != nil {
		return err
	}
	m := hmac.New(sha256.New, key)
	m.Write([]byte(tagStateV1))
	m.Write(body)
	env := stateEnvelope{Body: body, HMAC: m.Sum(nil)}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return atomicWriteState(path, raw)
}

func readState(path string, key []byte) (State, error) {
	var s State
	raw, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	var env stateEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return s, errors.New("state: malformed envelope")
	}
	m := hmac.New(sha256.New, key)
	m.Write([]byte(tagStateV1))
	m.Write(env.Body)
	if !hmac.Equal(m.Sum(nil), env.HMAC) {
		return s, errors.New("state: HMAC mismatch")
	}
	if err := json.Unmarshal(env.Body, &s); err != nil {
		return s, err
	}
	return s, nil
}

func atomicWriteState(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// deriveStateKey produces a 32-byte HMAC key bound to (license signature ||
// machine fingerprint). The user can wipe the state file but cannot rewrite
// it with a forged HMAC without possessing those inputs.
func deriveStateKey(sig, hostFingerprint []byte) []byte {
	h := sha256.New()
	h.Write([]byte("maldev-state-key-v1\x00"))
	h.Write(sig)
	h.Write(hostFingerprint)
	return h.Sum(nil)[:32]
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
