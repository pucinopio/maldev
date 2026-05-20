//go:build darwin

package hostid

import (
	"os/exec"
	"strings"
)

func platformEnrich() [][]byte {
	var out [][]byte
	// CPU brand string via sysctl.
	if b, err := exec.Command("/usr/sbin/sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
		if brand := strings.TrimSpace(string(b)); brand != "" {
			out = append(out, []byte(brand))
		}
	}
	return out
}
