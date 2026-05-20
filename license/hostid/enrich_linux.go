//go:build linux

package hostid

import (
	"bytes"
	"os"
	"strings"
)

func platformEnrich() [][]byte {
	var out [][]byte
	// CPU model name. /proc/cpuinfo's "model name" line is stable across
	// reboots and reflects the BIOS-reported CPU on bare metal; on VMs it
	// reflects the host CPU exposed by the hypervisor.
	if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range bytes.Split(b, []byte{'\n'}) {
			if !bytes.HasPrefix(line, []byte("model name")) {
				continue
			}
			if i := bytes.IndexByte(line, ':'); i > 0 {
				name := strings.TrimSpace(string(line[i+1:]))
				if name != "" {
					out = append(out, []byte(name))
				}
				break
			}
		}
	}
	// DMI product UUID — set by the BIOS, exposed via sysfs to root only.
	// We read best-effort; absence is not an error.
	if b, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
		if uuid := strings.TrimSpace(string(b)); uuid != "" {
			out = append(out, []byte(uuid))
		}
	}
	return out
}
