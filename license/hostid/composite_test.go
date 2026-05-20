package hostid

import (
	"bytes"
	"testing"
)

func TestCompositeReturns32Bytes(t *testing.T) {
	id, err := Composite()
	if err != nil {
		t.Skipf("composite unavailable on this host: %v", err)
	}
	if len(id) != 32 {
		t.Fatalf("len=%d", len(id))
	}
}

func TestCompositeDeterministic(t *testing.T) {
	a, err := Composite()
	if err != nil {
		t.Skipf("composite unavailable: %v", err)
	}
	b, _ := Composite()
	if !bytes.Equal(a, b) {
		t.Fatal("Composite non-deterministic across calls")
	}
}

func TestCompositeDiffersFromLocal(t *testing.T) {
	local, err := Local()
	if err != nil {
		t.Skipf("Local unavailable: %v", err)
	}
	composite, err := Composite()
	if err != nil {
		t.Skipf("Composite unavailable: %v", err)
	}
	if bytes.Equal(local, composite) {
		// Possible if no enrichment sources resolved AND the domain tags
		// happened to collide — but the tags are different by design, so
		// the outputs MUST differ.
		t.Fatal("Composite and Local produced the same fingerprint — tag separation is broken")
	}
}

func TestIsVirtualInterface(t *testing.T) {
	cases := map[string]bool{
		"docker0":      true,
		"br-abc":       true,
		"veth0":        true,
		"vmnet8":       true,
		"vboxnet0":     true,
		"vEthernet 1":  true,
		"eth0":         false,
		"en0":          false,
		"Wi-Fi":        false,
		"Ethernet":     false,
	}
	for name, want := range cases {
		if got := isVirtualInterface(name); got != want {
			t.Errorf("isVirtualInterface(%q)=%v want %v", name, got, want)
		}
	}
}
