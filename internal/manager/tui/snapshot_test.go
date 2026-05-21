package tui_test

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/tui"
	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
)

const updateGolden = "UPDATE_GOLDEN"

// goldenPath returns the path to the golden file for a given test name.
func goldenPath(name string) string {
	return filepath.Join("testdata", name+".golden")
}

// compareOrUpdate reads the golden file and compares it to got, or writes it
// when UPDATE_GOLDEN=1 is set.
func compareOrUpdate(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(name)
	if os.Getenv(updateGolden) == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		t.Logf("updated golden: %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with UPDATE_GOLDEN=1 to create)", path, err)
	}
	if string(want) != got {
		t.Errorf("snapshot mismatch for %s\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
	}
}

// initModel drives a model through Init and any seed messages, then returns
// the settled model.
func initModel(m tea.Model, extra ...tea.Msg) tea.Model {
	// Drive Init cmd (synchronously drain it by just ignoring cmds in test).
	m.Init() //nolint:errcheck — Init returns a Cmd we intentionally ignore in tests
	// Apply window size.
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	for _, msg := range extra {
		m, _ = m.Update(msg)
	}
	return m
}

func TestDashboardSnapshot(t *testing.T) {
	// Build a fully-populated dashboard by injecting a DashboardSnapshotMsg
	// directly so the test does not need a real DB.
	snap := cmds.DashboardSnapshotMsg{
		Active:               42,
		Revoked:              3,
		Expired:              7,
		ExpiringSoon:         5,
		ActiveKeyID:          "maldev-prod-01",
		ActiveKeyName:        "production-2026",
		ActiveKeyFingerprint: "ab:cd:ef:01",
		Servers: []cmds.ServerStatus{
			{Name: "Revocation", On: true, URL: "https://revoke.example.com:8443"},
			{Name: "Heartbeat", On: true, URL: "https://heartbeat.example.com:8444"},
			{Name: "Probe", On: false, URL: "—"},
		},
		RecentAudit: []cmds.AuditEntry{
			{Kind: "license.issue", TargetID: "uuid-1", Actor: "operator"},
			{Kind: "issuer.create", TargetID: "uuid-2", Actor: "operator"},
		},
	}

	root := tui.New(nil, nil, tui.SessionReady)
	m := initModel(root, snap)
	compareOrUpdate(t, "dashboard", m.View())
}

func TestPassphraseSnapshot(t *testing.T) {
	root := tui.New(nil, nil, tui.SessionLocked)
	m := initModel(root)
	compareOrUpdate(t, "passphrase_fresh", m.View())

	// Simulate one failed attempt by injecting UnlockResultMsg{OK: false}.
	m, _ = m.Update(cmds.UnlockResultMsg{OK: false})
	compareOrUpdate(t, "passphrase_failed1", m.View())
}

func TestOnboardingStep1Snapshot(t *testing.T) {
	root := tui.New(nil, nil, tui.SessionOnboarding)
	m := initModel(root)
	compareOrUpdate(t, "onboarding_step1", m.View())
}
