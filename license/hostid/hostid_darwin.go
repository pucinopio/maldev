//go:build darwin

package hostid

import (
	"errors"
	"os/exec"
	"strings"
)

func readPlatformSources() ([][]byte, error) {
	out, err := exec.Command("/usr/sbin/ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, `"IOPlatformUUID"`) {
			continue
		}
		j := strings.LastIndex(line, `"`)
		if j < 0 {
			continue
		}
		k := strings.LastIndex(line[:j], `"`)
		if k < 0 || j <= k+1 {
			continue
		}
		return [][]byte{[]byte(line[k+1 : j])}, nil
	}
	return nil, errors.New("hostid: IOPlatformUUID not found")
}
