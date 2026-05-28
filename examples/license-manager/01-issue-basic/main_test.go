package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	licensekg "github.com/oioio-space/maldev/license"
)

// TestRun_PrintsValidPEM is the E2E guard for the 01-issue-basic
// example. It runs main.go's run() against the same in-memory
// pipeline an operator gets at the shell, captures stdout (PEM) and
// stderr (status), and asserts:
//   - all five [ok] status lines fired in order
//   - stdout begins with the MALDEV LICENSE marker
//   - the PEM parses + verifies through the standalone license/ pkg
//
// If any of these break, the example documentation is lying and
// the README copy-paste path no longer works.
func TestRun_PrintsValidPEM(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}

	// ── stderr: expected status lines, in order ─────────────────────
	want := []string{
		"[ok] services up",
		"[ok] issuer \"lab\" created",
		"[ok] licence issued for subject \"alice@example.com\"",
		"[ok] verify round-trip green",
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

	// ── stdout: PEM only ────────────────────────────────────────────
	pem := stdout.Bytes()
	if !bytes.HasPrefix(pem, []byte("-----BEGIN MALDEV LICENSE-----")) {
		t.Fatalf("stdout does not start with PEM marker; got %q", string(pem[:min(64, len(pem))]))
	}

	// ── verify via the standalone license/ package ──────────────────
	parsed, err := licensekg.Inspect(pem)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if parsed.Subject != "alice@example.com" {
		t.Errorf("subject = %q, want alice@example.com", parsed.Subject)
	}
	if parsed.KeyID != "demo-2026-q2" {
		t.Errorf("key-id = %q, want demo-2026-q2", parsed.KeyID)
	}
	if len(parsed.Features) != 1 || parsed.Features[0] != "basic" {
		t.Errorf("features = %v, want [basic]", parsed.Features)
	}
}
