// Tutorial 02 dual E2E: render the bindings tape AND run the
// client against a licence bound to machine + password + TOTP.
package tutorial02_test

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
	"github.com/oioio-space/maldev/license/totp"
)

func TestTutorial02_VHSAndClient(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not in PATH")
	}
	root := repoRoot(t)
	ctx := context.Background()
	tmp := t.TempDir()

	// ── VHS render ───────────────────────────────────────────────────
	tape := filepath.Join(root, "vhs", "tui-gif", "tutorial-02-bindings.tape")
	gifPath := filepath.Join(tmp, "tutorial-02.gif")
	src, err := os.ReadFile(tape)
	if err != nil {
		t.Fatal(err)
	}
	tapeCopy := filepath.Join(tmp, "tutorial-02.tape")
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

	// ── Real licence with all three bindings ─────────────────────────
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
	iss, _ := svc.Issuer.Generate(ctx, "lab", "tutorial-02", "operator")
	_ = svc.Issuer.SetActive(ctx, iss.ID, "operator")

	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "alice@example.com",
		AudienceList: []string{"demo"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
		Bindings: []service.BindingSpec{
			{Type: "machine", Values: []string{"host-alpha"}},
			{Type: "password", Values: []string{"hunter2"}},
			{Type: "totp"},
		},
		Actor: "operator",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.TOTPs) != 1 {
		t.Fatalf("expected 1 TOTP provisioning, got %d", len(out.TOTPs))
	}
	totpSecret := out.TOTPs[0].Secret
	totpCode, _ := totp.Code(totpSecret, time.Now())
	pubPEM, _ := svc.Issuer.ExportPublic(ctx, iss.ID)
	licPath := filepath.Join(tmp, "license.pem")
	pubPath := filepath.Join(tmp, "issuer.pub")
	_ = os.WriteFile(licPath, out.PEM, 0o600)
	_ = os.WriteFile(pubPath, pubPEM, 0o644)

	binName := "license-check-2"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmp, binName)
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath,
		"./examples/license-manager/tutorials/02-bindings-and-verify/client")
	build.Dir = root
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("client build: %v", err)
	}

	// Positive: full evidence → accept.
	run := exec.CommandContext(ctx, binPath,
		"--license", licPath, "--issuer-pub", pubPath,
		"--machine", "host-alpha", "--password", "hunter2", "--totp", totpCode,
	)
	stdout, err := run.Output()
	if err != nil {
		t.Fatalf("client run (full evidence): %v\nstderr: %s", err, stderr(err))
	}
	if !strings.Contains(string(stdout), "[ok] licence verified") {
		t.Errorf("expected success line; got:\n%s", stdout)
	}
	t.Logf("[ok] client accepted with full evidence")

	// Negative: drop password → reject.
	miss := exec.CommandContext(ctx, binPath,
		"--license", licPath, "--issuer-pub", pubPath,
		"--machine", "host-alpha", "--totp", totpCode,
	)
	if err := miss.Run(); err == nil {
		t.Error("client accepted licence WITHOUT password evidence — binding skipped")
	}
	t.Logf("[ok] client rejected without password evidence")

	// Negative: wrong machine → reject.
	wrong := exec.CommandContext(ctx, binPath,
		"--license", licPath, "--issuer-pub", pubPath,
		"--machine", "host-charlie", "--password", "hunter2", "--totp", totpCode,
	)
	if err := wrong.Run(); err == nil {
		t.Error("client accepted licence on wrong machine")
	}
	t.Logf("[ok] client rejected wrong machine — tutorial 02 ships green")
}

// repoRoot walks up until go.mod is found.
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
