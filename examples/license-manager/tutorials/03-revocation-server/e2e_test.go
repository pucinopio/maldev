// Tutorial 03 dual E2E: render the revocation-server tape AND
// run the client against a real running RevocationServer.
package tutorial03_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

func TestTutorial03_VHSAndClient(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not in PATH")
	}
	root := repoRoot(t)
	ctx := context.Background()
	tmp := t.TempDir()

	// ── VHS render ───────────────────────────────────────────────────
	gifPath := filepath.Join(tmp, "tutorial-03.gif")
	if err := renderTape(ctx, root, "tutorial-03-revocation-server.tape", gifPath, tmp); err != nil {
		t.Fatalf("tape render: %v", err)
	}
	t.Logf("[ok] TUI tape rendered (%d bytes)", mustSize(gifPath))

	// ── Real manager + revocation server ─────────────────────────────
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

	iss, _ := svc.Issuer.Generate(ctx, "lab", "tutorial-03", "operator")
	_ = svc.Issuer.SetActive(ctx, iss.ID, "operator")
	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "alice@example.com",
		AudienceList: []string{"demo"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
		Actor:        "operator",
	})
	if err != nil {
		t.Fatal(err)
	}
	pubPEM, _ := svc.Issuer.ExportPublic(ctx, iss.ID)

	// Bind the revocation server to an ephemeral port — same path
	// the Settings screen drives via Bundle.AttachServers.
	if _, err := svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetRevocationListen("127.0.0.1:0")
	}); err != nil {
		t.Fatal(err)
	}
	rev := httpsrv.NewRevocationServer(svc.Revoke, svc.License, svc.Settings, svc.KEK)
	if err := rev.Start(ctx); err != nil {
		t.Fatalf("revocation server start: %v", err)
	}
	defer rev.Stop(time.Second)
	addr := rev.Status().ListenAddr
	if addr == "" {
		t.Fatal("revocation server didn't report a ListenAddr")
	}
	crlURL := fmt.Sprintf("http://%s/revoked.pem", addr)
	t.Logf("[ok] revocation server listening on %s", crlURL)

	// Build the client.
	licPath := filepath.Join(tmp, "license.pem")
	pubPath := filepath.Join(tmp, "issuer.pub")
	_ = os.WriteFile(licPath, out.PEM, 0o600)
	_ = os.WriteFile(pubPath, pubPEM, 0o644)
	binName := "license-check-3"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmp, binName)
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath,
		"./examples/license-manager/tutorials/03-revocation-server/client")
	build.Dir = root
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("client build: %v", err)
	}

	// ── Positive: not revoked → client accepts ──────────────────────
	pos := exec.CommandContext(ctx, binPath,
		"--license", licPath, "--issuer-pub", pubPath, "--crl-url", crlURL,
	)
	stdout, err := pos.Output()
	if err != nil {
		t.Fatalf("client (pre-revoke): %v\nstderr: %s", err, stderrOf(err))
	}
	if !strings.Contains(string(stdout), "[ok] licence verified") {
		t.Errorf("expected success pre-revoke; got:\n%s", stdout)
	}
	t.Logf("[ok] client accepted (CRL empty)")

	// ── Revoke + invalidate cache + retry → client rejects ─────────
	if err := svc.Revoke.Revoke(ctx, out.Row.ID, "key compromise", "operator"); err != nil {
		t.Fatal(err)
	}
	// The HTTP handler's PublishSignedList caches for validFor/2.
	// svc.Revoke.Revoke calls invalidateCache so the next GET signs a
	// fresh list — no sleep needed.
	neg := exec.CommandContext(ctx, binPath,
		"--license", licPath, "--issuer-pub", pubPath, "--crl-url", crlURL,
	)
	if err := neg.Run(); err == nil {
		t.Error("client accepted a revoked licence — CRL not consulted")
	}
	t.Logf("[ok] client rejected after revoke — tutorial 03 ships green")
}

// ─── shared helpers ─────────────────────────────────────────────────

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

// renderTape copies tapeName from vhs/tui-gif/ to scratchDir,
// rewrites the Output line to gifPath, and runs cmd/tui-gif.
func renderTape(ctx context.Context, root, tapeName, gifPath, scratch string) error {
	src, err := os.ReadFile(filepath.Join(root, "vhs", "tui-gif", tapeName))
	if err != nil {
		return err
	}
	var b strings.Builder
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Output ") {
			b.WriteString("Output ")
			b.WriteString(gifPath)
			b.WriteByte('\n')
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	tapeCopy := filepath.Join(scratch, tapeName)
	if err := os.WriteFile(tapeCopy, []byte(b.String()), 0o600); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/tui-gif", tapeCopy)
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	st, err := os.Stat(gifPath)
	if err != nil || st.Size() < 1024 {
		return fmt.Errorf("gif missing or too small (size=%d, err=%v)", sizeOr(st), err)
	}
	return nil
}

func sizeOr(st os.FileInfo) int64 {
	if st == nil {
		return 0
	}
	return st.Size()
}
func mustSize(p string) int64 { st, _ := os.Stat(p); return sizeOr(st) }
func stderrOf(err error) string {
	if ee, ok := err.(*exec.ExitError); ok {
		return string(ee.Stderr)
	}
	return err.Error()
}
