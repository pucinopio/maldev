// Package hostid produces a 32-byte machine fingerprint by mixing OS-provided
// identifiers (registry MachineGuid on Windows, /etc/machine-id on Linux,
// IOPlatformUUID on darwin) through sha256. The output is suitable for use as
// the evidence in WithMachineID(...).
package hostid

import (
	"crypto/sha256"
	"errors"
)

// tagHostIDV1 domain-separates the fingerprint hash so it cannot be confused
// with any other maldev-prefixed digest.
const tagHostIDV1 = "maldev-hostid-v1\x00"

// Local returns a 32-byte fingerprint of the current machine.
func Local() ([]byte, error) {
	parts, err := readPlatformSources()
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, errors.New("hostid: no identifier source available")
	}
	h := sha256.New()
	h.Write([]byte(tagHostIDV1))
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		h.Write([]byte{byte(len(p) >> 8), byte(len(p))})
		h.Write(p)
	}
	return h.Sum(nil), nil
}
