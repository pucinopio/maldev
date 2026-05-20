// Package fileutil provides shared filesystem helpers for the license package
// and its sub-packages. It is internal-only.
package fileutil

import (
	"os"
	"path/filepath"
)

// AtomicWrite writes data to path atomically: write a sibling temp file, fsync,
// rename over the destination. The parent directory is created with 0700 if
// absent. prefix names the temp file (e.g. ".state-*.tmp", ".cache-*.tmp") so
// concurrent writes from different callers don't collide.
func AtomicWrite(path, prefix string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, prefix)
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
