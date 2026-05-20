package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/oioio-space/maldev/license/revoke"
)

// RevocationStore persists the current revocation list. The handler signs the
// list itself on each GET; the store keeps the cleartext.
type RevocationStore interface {
	Load(ctx context.Context) (revoke.List, error)
	Save(ctx context.Context, l revoke.List) error
}

// LicenseStatus reports a single license's state in the issuer's records.
type LicenseStatus int

const (
	StatusUnknown LicenseStatus = iota
	StatusActive
	StatusRevoked
	StatusExpired
)

type LicenseStore interface {
	Status(ctx context.Context, licenseID string) (LicenseStatus, error)
}

// StaticLicenseStore is a test helper.
type StaticLicenseStore map[string]LicenseStatus

func (s StaticLicenseStore) Status(_ context.Context, id string) (LicenseStatus, error) {
	if v, ok := s[id]; ok {
		return v, nil
	}
	return StatusUnknown, nil
}

// FileStore returns a RevocationStore backed by a JSON file.
func FileStore(path string) RevocationStore { return &fileStore{path: path} }

type fileStore struct {
	mu   sync.Mutex
	path string
}

func (f *fileStore) Load(_ context.Context) (revoke.List, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return revoke.List{}, nil
	}
	if err != nil {
		return revoke.List{}, err
	}
	var l revoke.List
	if err := json.Unmarshal(raw, &l); err != nil {
		return revoke.List{}, err
	}
	return l, nil
}

func (f *fileStore) Save(_ context.Context, l revoke.List) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return os.WriteFile(f.path, raw, 0o600)
}
