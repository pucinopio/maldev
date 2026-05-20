package revoke

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// cacheMu guards monotonic sequence updates across concurrent Verify calls.
var cacheMu sync.Mutex

// minStore keeps the highest-seen sequence per cache path so a downgrade is
// rejected even when the on-disk file is rewritten externally between calls.
var minStore = map[string]uint64{}

// StoreCache writes the signed list bytes to path. minSeq is the highest
// sequence the caller has observed; subsequent StoreCache with seq < minSeq
// is rejected.
func StoreCache(path string, signed []byte, seq uint64) error {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cur := minStore[path]; cur > seq {
		return fmt.Errorf("revoke: sequence regression (%d < %d)", seq, cur)
	}
	minStore[path] = seq
	return atomicWrite(path, signed)
}

// LoadCache reads, verifies, and returns the cached list. Errors if the cache
// is absent, malformed, mis-signed, expired, or its sequence has regressed.
func LoadCache(path string, pub ed25519.PublicKey, kid string, now time.Time) (*List, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	l, err := VerifyBytes(raw, pub, kid)
	if err != nil {
		return nil, err
	}
	if !l.ExpiresAt.IsZero() && now.After(l.ExpiresAt) {
		return nil, errors.New("revoke: cache expired")
	}
	cacheMu.Lock()
	if cur := minStore[path]; l.Sequence < cur {
		cacheMu.Unlock()
		return nil, fmt.Errorf("revoke: cached sequence < minStore")
	}
	if minStore[path] < l.Sequence {
		minStore[path] = l.Sequence
	}
	cacheMu.Unlock()
	return l, nil
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".cache-*.tmp")
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
