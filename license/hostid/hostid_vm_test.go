//go:build vmtest

package hostid

import "testing"

func TestHostIDLocal_Real(t *testing.T) {
	id, err := Local()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 32 {
		t.Fatalf("len=%d", len(id))
	}
}
