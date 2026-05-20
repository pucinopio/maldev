package probe

import (
	"bytes"
	"strings"
	"testing"
)

func TestServeAgentReturnsBinary(t *testing.T) {
	cases := []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"}
	for _, c := range cases {
		got, err := ServeAgent(c)
		if err != nil {
			t.Fatalf("ServeAgent(%q): %v", c, err)
		}
		if len(got) < 1024 {
			t.Fatalf("%s too small: %d bytes", c, len(got))
		}
	}
}

func TestServeAgentRejectsUnknown(t *testing.T) {
	_, err := ServeAgent("plan9-mips64")
	if err == nil {
		t.Fatal("unknown target accepted")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("err=%v", err)
	}
}

func TestAvailableTargets(t *testing.T) {
	tgs := AvailableTargets()
	if len(tgs) != 5 {
		t.Fatalf("got %d targets, want 5", len(tgs))
	}
	if !bytes.Contains([]byte(strings.Join(tgs, ",")), []byte("linux-amd64")) {
		t.Fatalf("missing linux-amd64: %v", tgs)
	}
}
