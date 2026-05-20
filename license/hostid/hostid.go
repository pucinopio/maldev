// Package hostid produces a 32-byte machine fingerprint by mixing OS-provided
// identifiers (registry MachineGuid on Windows, /etc/machine-id on Linux,
// IOPlatformUUID on darwin) through sha256. The output is suitable for use as
// the evidence in WithMachineID(...).
package hostid

import (
	"crypto/sha256"
	"errors"
	"sync"
)

// tagHostIDV1 domain-separates the canonical (single-source) fingerprint
// hash. tagHostIDCompositeV1 separates the multi-source variant — the two
// hashes must NEVER collide, since they're computed over different inputs
// and downstream code may use one as a stable identifier and the other as
// anti-spoofing evidence.
const (
	tagHostIDV1          = "maldev-hostid-v1\x00"
	tagHostIDCompositeV1 = "maldev-hostid-composite-v1\x00"
)

// Local returns a 32-byte fingerprint built from the platform's canonical
// machine identifier (MachineGuid on Windows, /etc/machine-id on Linux,
// IOPlatformUUID on darwin). Stable across reboots, sensitive to OS
// reinstall.
func Local() ([]byte, error) {
	parts, err := readPlatformSources()
	if err != nil {
		return nil, err
	}
	return mix(tagHostIDV1, parts)
}

// Composite returns a 32-byte fingerprint that mixes Local() with extra
// per-platform hardware signals (CPU brand string and the MAC of the first
// stable non-loopback interface). Harder to spoof than Local() alone — an
// attacker who swaps a single source (e.g. fakes /etc/machine-id in a
// container) will see Composite() differ from the issuer's expected value
// even when Local() looks normal.
//
// Composite() is more sensitive than Local() to legitimate hardware
// changes: replacing a NIC, swapping a CPU, or running on a different VM
// flavour will change the result. Prefer Local() for "this is the same
// install" semantics; prefer Composite() for "this is the same hardware".
//
// The result is cached for the process lifetime via sync.Once: enrichment
// reads can fork sysctl (darwin), read /proc/cpuinfo (linux), and walk
// net.Interfaces — none of which change while a process is running.
func Composite() ([]byte, error) {
	compositeOnce.Do(func() {
		parts, err := readPlatformSources()
		if err != nil {
			compositeErr = err
			return
		}
		parts = append(parts, readEnrichmentSources()...)
		compositeVal, compositeErr = mix(tagHostIDCompositeV1, parts)
	})
	return compositeVal, compositeErr
}

var (
	compositeOnce sync.Once
	compositeVal  []byte
	compositeErr  error
)

func mix(tag string, parts [][]byte) ([]byte, error) {
	if len(parts) == 0 {
		return nil, errors.New("hostid: no identifier source available")
	}
	h := sha256.New()
	h.Write([]byte(tag))
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		h.Write([]byte{byte(len(p) >> 8), byte(len(p))})
		h.Write(p)
	}
	return h.Sum(nil), nil
}
