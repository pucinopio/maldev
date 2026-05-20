//go:build windows

package hostid

import "golang.org/x/sys/windows/registry"

func platformEnrich() [][]byte {
	var out [][]byte
	// CPU brand string. Stable across reboots, changes on physical CPU
	// swap. Reading the BIOS-stamped value, not a derived one, so VM
	// migrations that preserve the virtual CPU vendor will preserve this.
	if k, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`HARDWARE\DESCRIPTION\System\CentralProcessor\0`,
		registry.QUERY_VALUE,
	); err == nil {
		if name, _, err := k.GetStringValue("ProcessorNameString"); err == nil && name != "" {
			out = append(out, []byte(name))
		}
		_ = k.Close()
	}
	return out
}
