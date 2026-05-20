package license

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"sync"

	"github.com/oioio-space/maldev/license/identity"
)

// HashFile returns the hex sha256 of a file's contents.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// selfHash caches the hash of os.Executable for the process lifetime. The
// running binary cannot change under a live process, so re-reading and
// re-hashing on every Verify is pure waste.
var (
	selfHashOnce sync.Once
	selfHashVal  string
	selfHashErr  error
)

func selfBinaryHash() (string, error) {
	selfHashOnce.Do(func() {
		path, err := os.Executable()
		if err != nil {
			selfHashErr = err
			return
		}
		selfHashVal, selfHashErr = HashFile(path)
	})
	return selfHashVal, selfHashErr
}

// HashIdentity is a convenience re-export to avoid callers importing the
// identity sub-package just to compute a hash for License.IdentitySHA256.
func HashIdentity(b []byte) string { return identity.HashIdentity(b) }

func checkPinning(lic License, s *verifyState) cause {
	if !s.binaryPinning {
		return causeOK
	}
	haveDisk := lic.BinarySHA256 != ""
	haveID := lic.IdentitySHA256 != ""
	if !haveDisk && !haveID {
		s.warnings = append(s.warnings, "pinning requested but license carries no pin")
		return causeOK
	}
	if haveDisk {
		got, err := selfBinaryHash()
		if err != nil || got != lic.BinarySHA256 {
			return causeBinaryHashMismatch
		}
	}
	if haveID {
		b := s.identityBytes
		if b == nil {
			b = identity.Read()
		}
		if HashIdentity(b) != lic.IdentitySHA256 {
			return causeIdentityMismatch
		}
	}
	return causeOK
}
