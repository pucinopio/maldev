//go:build linux

package hostid

import (
	"bytes"
	"errors"
	"os"
)

func readPlatformSources() ([][]byte, error) {
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		b, err := os.ReadFile(p)
		if err == nil {
			b = bytes.TrimSpace(b)
			if len(b) > 0 {
				return [][]byte{b}, nil
			}
		}
	}
	return nil, errors.New("hostid: no machine-id file readable")
}
