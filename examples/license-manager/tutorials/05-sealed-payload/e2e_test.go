// Tutorial 05 dual E2E: render the Recipients tape AND run the
// verifier+sealed-payload client against a licence with a sealed body.
package tutorial05_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store"
)

func TestTutorial05_VHSAndClient(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not in PATH")
	}
	root := repoRoot(t)
	ctx := context.Background()
	tmp := t.TempDir()

	// ── VHS render ───────────────────────────────────────────────────
	tape := filepath.Join(root, "vhs", "tui-gif", "tutorial-05-sealed.tape")
	gifPath := filepath.Join(tmp, "tutorial-05.gif")
	src, err := os.ReadFile(tape)
	if err != nil {
		t.Fatal(err)
	}
	tapeCopy := filepath.Join(tmp, "tutorial-05.tape")
	if err := os.WriteFile(tapeCopy, []byte(rewriteOutput(string(src), gifPath)), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/tui-gif", tapeCopy)
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("tui-gif render: %v", err)
	}
	if st, err := os.Stat(gifPath); err != nil || st.Size() < 1024 {
		t.Fatalf("gif missing or too small (size=%d, err=%v)", sizeOr(st), err)
	}
	t.Logf("[ok] TUI tape rendered (%d bytes)", mustSize(gifPath))

	// ── Real sealed-payload licence ──────────────────────────────────
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
	iss, _ := svc.Issuer.Generate(ctx, "lab", "tutorial-05", "operator")
	_ = svc.Issuer.SetActive(ctx, iss.ID, "operator")

	rec, err := svc.Recipient.Generate(ctx, "alice-laptop", "operator")
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("activation-token=ZX-7782-DELTA")

	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID:     iss.ID,
		Subject:      "alice@example.com",
		AudienceList: []string{"demo"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
		SealedFor:    &rec.ID,
		SealedPlain:  plaintext,
		Actor:        "operator",
	})
	if err != nil {
		t.Fatal(err)
	}
	priv, err := svc.Recipient.PrivateKey(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}

	pubPEM, _ := svc.Issuer.ExportPublic(ctx, iss.ID)
	licPath := filepath.Join(tmp, "license.pem")
	pubPath := filepath.Join(tmp, "issuer.pub")
	privPath := filepath.Join(tmp, "recipient.x25519")
	_ = os.WriteFile(licPath, out.PEM, 0o600)
	_ = os.WriteFile(pubPath, pubPEM, 0o644)
	_ = os.WriteFile(privPath, priv, 0o600)

	binName := "license-check-5"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmp, binName)
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath,
		"./examples/license-manager/tutorials/05-sealed-payload/client")
	build.Dir = root
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("client build: %v", err)
	}

	// Positive — correct recipient key opens the payload.
	run := exec.CommandContext(ctx, binPath,
		"--license", licPath, "--issuer-pub", pubPath, "--recipient-priv", privPath)
	stdout, err := run.Output()
	if err != nil {
		t.Fatalf("client run (correct key): %v\nstderr: %s", err, stderr(err))
	}
	if !strings.Contains(string(stdout), "activation-token=ZX-7782-DELTA") {
		t.Errorf("plaintext missing in output:\n%s", stdout)
	}
	t.Logf("[ok] client decrypted sealed payload with the correct key")

	// Negative — wrong recipient key is rejected.
	wrongRec, _ := svc.Recipient.Generate(ctx, "mallory", "operator")
	wrongPriv, _ := svc.Recipient.PrivateKey(ctx, wrongRec.ID)
	wrongPath := filepath.Join(tmp, "mallory.x25519")
	_ = os.WriteFile(wrongPath, wrongPriv, 0o600)
	bad := exec.CommandContext(ctx, binPath,
		"--license", licPath, "--issuer-pub", pubPath, "--recipient-priv", wrongPath)
	if err := bad.Run(); err == nil {
		t.Error("client opened sealed payload with the wrong recipient key")
	}
	t.Logf("[ok] client rejected wrong recipient key — tutorial 05 ships green")
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}

func rewriteOutput(src, gif string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Output ") {
			b.WriteString("Output ")
			b.WriteString(gif)
			b.WriteByte('\n')
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func sizeOr(st os.FileInfo) int64 {
	if st == nil {
		return 0
	}
	return st.Size()
}
func mustSize(p string) int64 { st, _ := os.Stat(p); return sizeOr(st) }
func stderr(err error) string {
	if ee, ok := err.(*exec.ExitError); ok {
		return string(ee.Stderr)
	}
	return err.Error()
}
