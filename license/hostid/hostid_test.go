package hostid

import (
	"bytes"
	"testing"
)

func TestLocalReturns32Bytes(t *testing.T) {
	id, err := Local()
	if err != nil {
		t.Skipf("hostid not available on this host: %v", err)
	}
	if len(id) != 32 {
		t.Fatalf("len=%d", len(id))
	}
}

func TestLocalDeterministic(t *testing.T) {
	a, err := Local()
	if err != nil {
		t.Skipf("hostid unavailable: %v", err)
	}
	b, err := Local()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("Local() non-deterministic across calls")
	}
}
