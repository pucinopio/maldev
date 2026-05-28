package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRun_CrossInstanceImportAndVerify(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, w := range []string{
		"[ok] instance-A issued",
		"[ok] instance-B imported",
		"[ok] verify GREEN with public key only",
	} {
		if !strings.Contains(stderr.String(), w) {
			t.Errorf("stderr missing %q", w)
		}
	}
	if !bytes.HasPrefix(stdout.Bytes(), []byte("-----BEGIN MALDEV LICENSE-----")) {
		t.Fatal("stdout not a PEM")
	}
}
