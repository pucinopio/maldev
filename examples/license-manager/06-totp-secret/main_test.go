package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRun_TOTPSecretAndQR(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, w := range []string{
		"[ok] secret created",
		"[ok] view materialised",
		"[ok] 6-digit code for now:",
	} {
		if !strings.Contains(stderr.String(), w) {
			t.Errorf("stderr missing %q", w)
		}
	}
	// stdout: URI + blank + QR.
	out := stdout.String()
	if !strings.HasPrefix(out, "otpauth://totp/") {
		t.Errorf("stdout doesn't start with otpauth://, got %q", out[:min(64, len(out))])
	}
	if !strings.ContainsAny(out, "█▀▄ ") {
		t.Error("stdout missing QR half-block glyphs")
	}
}
