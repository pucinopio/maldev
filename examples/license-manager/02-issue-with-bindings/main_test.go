package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestRun_ProvesAllBindingsRequired is the E2E guard for the
// 02-issue-with-bindings example. It runs the example pipeline and
// asserts that EVERY [ok] status line fires — including the two
// negative paths that prove verify rejects partial evidence.
func TestRun_ProvesAllBindingsRequired(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{
		"[ok] services up",
		"[ok] licence issued",
		"[ok] TOTP secret returned out-of-band",
		"[ok] verify GREEN with full evidence",
		"[ok] verify RED when password evidence missing",
		"[ok] verify RED when machine doesn't match",
	}
	s := stderr.String()
	idx := 0
	for _, w := range want {
		i := strings.Index(s[idx:], w)
		if i < 0 {
			t.Errorf("stderr missing %q (or out of order)\nstderr:\n%s", w, s)
			continue
		}
		idx += i + len(w)
	}
	if !bytes.HasPrefix(stdout.Bytes(), []byte("-----BEGIN MALDEV LICENSE-----")) {
		t.Fatalf("stdout not a PEM; got %q", string(stdout.Bytes()[:min(64, len(stdout.Bytes()))]))
	}
}
