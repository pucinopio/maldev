// Package tutorial01_test is the top-level E2E for tutorial 01.
//
// It chains BOTH halves of the operator-facing scenario:
//
//  1. Render the TUI walkthrough via cmd/tui-gif against the
//     committed tape (vhs/tui-gif/tutorial-01-issue-and-verify.tape).
//     A successful render proves the documented key sequence still
//     reaches the screens / detail tabs the tutorial promises.
//  2. Build the client binary and run it against a real licence + a
//     real issuer public key produced by service.Services — the same
//     pipeline the TUI's new-licence wizard drives.
//
// The test is end-to-end in the operator sense: TUI demo renders,
// client binary verifies, both green ⇒ tutorial 01 ships.
package tutorial01_test

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

// repoRoot walks up from this file's directory until it finds go.mod.
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

// TestTutorial01_VHSAndClient is the dual E2E: render the VHS tape AND
// run the client verifier. Either failure stops the test.
func TestTutorial01_VHSAndClient(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not in PATH")
	}
	root := repoRoot(t)
	ctx := context.Background()
	tmp := t.TempDir()

	// ── 1. VHS replay via cmd/tui-gif ────────────────────────────────
	// Output to t.TempDir so the rendered .gif doesn't pollute the
	// gitignored vhs/out/ in parallel runs.
	tapeSrc := filepath.Join(root, "vhs", "tui-gif", "tutorial-01-issue-and-verify.tape")
	if _, err := os.Stat(tapeSrc); err != nil {
		t.Fatalf("tape file missing: %v", err)
	}
	// Materialise a copy with the Output line rewritten to t.TempDir
	// so the gif lands under the test sandbox.
	src, err := os.ReadFile(tapeSrc)
	if err != nil {
		t.Fatal(err)
	}
	gifPath := filepath.Join(tmp, "tutorial-01.gif")
	rewritten := rewriteOutput(string(src), gifPath)
	tapeCopy := filepath.Join(tmp, "tutorial-01.tape")
	if err := os.WriteFile(tapeCopy, []byte(rewritten), 0o600); err != nil {
		t.Fatal(err)
	}
	tuiGif := exec.CommandContext(ctx, "go", "run", "./cmd/tui-gif", tapeCopy)
	tuiGif.Dir = root
	tuiGif.Stderr = os.Stderr
	if err := tuiGif.Run(); err != nil {
		t.Fatalf("tui-gif: %v (the TUI tape stopped reaching its targets — operator walkthrough is stale)", err)
	}
	if st, err := os.Stat(gifPath); err != nil || st.Size() < 1024 {
		t.Fatalf("tui-gif produced no usable GIF (size=%d, err=%v)", sizeOr(st), err)
	}
	t.Logf("[ok] TUI walkthrough rendered: %s (%d bytes)", gifPath, mustSize(gifPath))

	// ── 2. Real licence + client verify ──────────────────────────────
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
		AudienceList: []string{"demo"}, Features: []string{"basic"},
		NotBefore: time.Now(), NotAfter: time.Now().Add(30 * 24 * time.Hour),
		Actor: "operator",
	})
	if err != nil {
		t.Fatal(err)
	}
	pubPEM, _ := svc.Issuer.ExportPublic(ctx, iss.ID)
	licPath := filepath.Join(tmp, "license.pem")
	pubPath := filepath.Join(tmp, "issuer.pub")
	_ = os.WriteFile(licPath, out.PEM, 0o600)
	_ = os.WriteFile(pubPath, pubPEM, 0o644)

	binName := "license-check"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmp, binName)
	build := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./examples/license-manager/tutorials/01-issue-and-verify/client")
	build.Dir = root
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("client build: %v", err)
	}
	cmd := exec.CommandContext(ctx, binPath, "--license", licPath, "--issuer-pub", pubPath)
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("client run: %v\nstderr: %s", err, stderr(err))
	}
	for _, w := range []string{"[ok] licence verified", "alice@example.com", "key-id tutorial-01"} {
		if !strings.Contains(string(stdout), w) {
			t.Errorf("client stdout missing %q\nfull stdout:\n%s", w, stdout)
		}
	}
	t.Logf("[ok] client verified the real licence — tutorial 01 ships green")
}

// rewriteOutput replaces the `Output <path>` line in a tape so the
// rendered gif lands at gifPath. Other directives stay intact.
func rewriteOutput(src, gifPath string) string {
	var out strings.Builder
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Output ") {
			out.WriteString("Output ")
			out.WriteString(gifPath)
			out.WriteString("\n")
			continue
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String()
}

func sizeOr(st os.FileInfo) int64 {
	if st == nil {
		return 0
	}
	return st.Size()
}

func mustSize(p string) int64 {
	st, _ := os.Stat(p)
	return sizeOr(st)
}

func stderr(err error) string {
	if ee, ok := err.(*exec.ExitError); ok {
		return string(ee.Stderr)
	}
	return err.Error()
}
