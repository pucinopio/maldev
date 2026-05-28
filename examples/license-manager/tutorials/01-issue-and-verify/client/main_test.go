package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store"
)

// TestClient_EndToEnd compiles this client binary, generates a real
// licence + issuer public-key on disk via the same code the TUI's
// new-licence wizard runs, then invokes the compiled client to verify
// the PEM. Proves the operator's "issue in the TUI, hand the binary +
// PEM + pub-key to a licensee, the licensee's binary verifies" path
// works end-to-end.
//
// Skipped when `go` is not on PATH (rare; only matters in cross-build
// environments).
func TestClient_EndToEnd(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not in PATH")
	}

	ctx := context.Background()
	tmp := t.TempDir()
	licPath := filepath.Join(tmp, "license.pem")
	pubPath := filepath.Join(tmp, "issuer.pub")
	binPath := filepath.Join(tmp, "license-check.exe")

	// Build the client.
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("go build client: %v", err)
	}

	// Issue a licence the same way the TUI does.
	st, err := store.New(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	salt := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	kek := crypto.DeriveFromPassphrase("demo", salt)
	canary, _ := crypto.NewCanary(kek)
	if err := st.EnsureSingletons(ctx, salt[:], canary); err != nil {
		t.Fatal(err)
	}
	svc := service.New(st, kek)
	defer svc.Close()
	iss, _ := svc.Issuer.Generate(ctx, "lab", "tutorial-01", "operator")
	_ = svc.Issuer.SetActive(ctx, iss.ID, "operator")
	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "alice@example.com",
		AudienceList: []string{"demo"},
		Features:     []string{"basic"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
		Actor:        "operator",
	})
	if err != nil {
		t.Fatal(err)
	}
	pubPEM, _ := svc.Issuer.ExportPublic(ctx, iss.ID)

	if err := os.WriteFile(licPath, out.PEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		t.Fatal(err)
	}

	// Invoke the client.
	cmd := exec.CommandContext(ctx, binPath, "--license", licPath, "--issuer-pub", pubPath)
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("client exit: %v\nstderr: %s", err, exitStderr(err))
	}
	got := string(stdout)
	for _, want := range []string{
		"[ok] licence verified",
		"subject:  alice@example.com",
		"key-id tutorial-01",
		"features: [basic]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("client stdout missing %q\nstdout:\n%s", want, got)
		}
	}

	// Negative: tampered licence (flip one byte) → client exits non-zero.
	tampered := append([]byte(nil), out.PEM...)
	tampered[len(tampered)/2] ^= 0x01
	if err := os.WriteFile(licPath, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := exec.CommandContext(ctx, binPath, "--license", licPath, "--issuer-pub", pubPath).Run(); err == nil {
		t.Error("client accepted a tampered licence — verifier short-circuited")
	}
}

func exitStderr(err error) string {
	if ee, ok := err.(*exec.ExitError); ok {
		return string(ee.Stderr)
	}
	return ""
}
