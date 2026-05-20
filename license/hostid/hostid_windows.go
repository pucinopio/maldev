//go:build windows

package hostid

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

func readPlatformSources() ([][]byte, error) {
	k, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Cryptography`,
		registry.QUERY_VALUE|registry.WOW64_64KEY,
	)
	if err != nil {
		return nil, err
	}
	defer k.Close()
	guid, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return nil, err
	}
	if guid == "" {
		return nil, errors.New("hostid: empty MachineGuid")
	}
	return [][]byte{[]byte(guid)}, nil
}
