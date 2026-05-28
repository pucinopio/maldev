package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRun_RevokedLicenceRejectedByCRL(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{
		"[ok] services up",
		"[ok] licence issued",
		"[ok] licence marked revoked",
		"[ok] CRL signed + published",
		"[ok] verify RED with CRL consulted",
		"[ok] verify GREEN without CRL",
	}
	s := stderr.String()
	idx := 0
	for _, w := range want {
		i := strings.Index(s[idx:], w)
		if i < 0 {
			t.Errorf("stderr missing %q\nstderr:\n%s", w, s)
			continue
		}
		idx += i + len(w)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("MALDEV REVOCATION")) {
		t.Errorf("stdout missing CRL marker; got %q", string(stdout.Bytes()[:min(64, len(stdout.Bytes()))]))
	}
}
