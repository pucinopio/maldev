package totp

import "os"

// writeFile writes data to path with 0o644 mode. Extracted so QR helpers and
// callers share a consistent file-creation pattern.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
