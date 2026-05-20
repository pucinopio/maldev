package hostid

import (
	"bytes"
	"net"
	"sort"
)

// readEnrichmentSources returns extra hardware signals to mix into the
// composite fingerprint. Platform-specific sources (CPU brand, motherboard
// serial) are pulled from enrich_windows.go / enrich_linux.go /
// enrich_darwin.go; the cross-platform piece (primary MAC) lives here so
// the platform files don't repeat the net.Interfaces dance.
func readEnrichmentSources() [][]byte {
	var out [][]byte
	if mac := primaryMAC(); len(mac) > 0 {
		out = append(out, mac)
	}
	out = append(out, platformEnrich()...)
	return out
}

// primaryMAC returns the hardware address of the first up, non-loopback,
// non-virtual interface in deterministic order. Virtual / docker / tap /
// VPN interfaces are filtered out because their MACs are unstable.
func primaryMAC() []byte {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	sort.Slice(ifs, func(i, j int) bool { return ifs[i].Index < ifs[j].Index })
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(ifc.HardwareAddr) == 0 {
			continue
		}
		if isVirtualInterface(ifc.Name) {
			continue
		}
		// Some adapters expose all-zero MACs — skip them to avoid a stable
		// but uninformative fingerprint component.
		if bytes.Equal(ifc.HardwareAddr, make([]byte, len(ifc.HardwareAddr))) {
			continue
		}
		return append([]byte(nil), ifc.HardwareAddr...)
	}
	return nil
}

// isVirtualInterface reports whether an interface name matches a known
// virtual adapter prefix. Conservative — when in doubt, keep the
// interface (the cost is fingerprint instability across VPN toggles, not
// security).
func isVirtualInterface(name string) bool {
	prefixes := []string{
		"docker", "br-", "veth", "virbr", "tap", "tun",
		"vmnet", "vboxnet", "vEthernet", "VBox", "Tailscale",
		"wg", "utun", "ZeroTier", "Hyper-V",
	}
	lname := name
	for _, p := range prefixes {
		if len(lname) >= len(p) && lname[:len(p)] == p {
			return true
		}
	}
	return false
}
