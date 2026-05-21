package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// TestE2E_WorkflowCoverage is the meta-tracker for every user-visible workflow
// of license-manager. Each sub-test documents one workflow path (status:
// covered / TODO / blocked) so the team can see at a glance what is and isn't
// proven by E2E.
//
// Conventions:
//
//   - covered() means the sub-test asserts a state change matching the workflow
//   - todo()    means the workflow exists in the UI but no assertion is wired
//     yet (test still passes — the call documents the gap)
//   - blocked() means an upstream service / wiring is missing
//
// Running:
//
//	go test ./internal/manager/tui/... -run TestE2E_WorkflowCoverage -v
//
// produces a checklist of every workflow path the manager exposes.
func TestE2E_WorkflowCoverage(t *testing.T) {
	const W, H = 160, 50

	// onboardingFlow drives the first-launch wizard end-to-end.
	t.Run("onboarding/welcome→passphrase→issuer→license", func(t *testing.T) {
		var m tea.Model = New(nil, nil, SessionOnboarding)
		m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})

		// Welcome → Enter
		m = driveKey(m, tea.KeyEnter)
		if rootOf(t, m).onboarding.step != stepPassphrase {
			t.Fatalf("after Enter expected stepPassphrase")
		}
		// Passphrase → "secret" + Enter → confirm field → "secret" + Enter
		m = driveStr(m, "secret123")
		m = driveKey(m, tea.KeyEnter)
		m = driveStr(m, "secret123")
		m = driveKey(m, tea.KeyEnter)
		if rootOf(t, m).onboarding.step != stepIssuer {
			t.Fatalf("after passphrases match expected stepIssuer")
		}
		// Issuer → name + Tab + keyID + Enter
		m = driveStr(m, "prod-2026")
		m = driveKey(m, tea.KeyTab)
		m = driveStr(m, "maldev-prod-01")
		m = driveKey(m, tea.KeyEnter)
		if rootOf(t, m).onboarding.step != stepLicense {
			t.Fatalf("after issuer step expected stepLicense")
		}
		// License → Enter completes
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			if msg := cmd(); msg != nil {
				if d, ok := msg.(OnboardingDoneMsg); ok {
					if d.Passphrase != "secret123" || d.IssuerName != "prod-2026" || d.IssuerKeyID != "maldev-prod-01" {
						t.Errorf("OnboardingDoneMsg has wrong fields: %+v", d)
					}
				}
			}
		}
		_ = updated
	})

	// ── Dashboard ──────────────────────────────────────────────────────────
	t.Run("dashboard/tab-click-cycles-views", func(t *testing.T) {
		var m tea.Model = New(nil, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
		// Tab 4 times → ViewIdentities
		for i := 0; i < 4; i++ {
			m = driveKey(m, tea.KeyTab)
		}
		if rootOf(t, m).active != ViewIdentities {
			t.Errorf("Tab×4 expected ViewIdentities, got %s", rootOf(t, m).active)
		}
	})
	t.Run("dashboard/superseded-tile-filters-correctly", func(t *testing.T) {
		var m tea.Model = New(nil, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
		// Send SwitchToLicensesMsg with Filter=superseded and verify routing.
		m, _ = m.Update(SwitchToLicensesMsg{Filter: "superseded"})
		r := rootOf(t, m)
		if r.active != ViewLicenses || r.licenses.filter != licFilterSuperseded {
			t.Errorf("expected ViewLicenses+licFilterSuperseded, got %s+%v", r.active, r.licenses.filter)
		}
	})

	// ── Licenses ───────────────────────────────────────────────────────────
	t.Run("licenses/n-opens-wizard-overlay", func(t *testing.T) {
		var m tea.Model = New(nil, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
		m = driveRune(m, '2')
		// Wait for table to load (empty rows) then press 'n'.
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		for i := 0; i < 4 && cmd != nil; i++ {
			msg := cmd()
			if msg == nil {
				break
			}
			updated, cmd = updated.Update(msg)
		}
		if r := rootOf(t, updated); len(r.overlays) == 0 {
			t.Errorf("expected wizard overlay pushed after 'n', got 0 overlays")
		}
	})
	t.Run("licenses/detail-tab-IBPAC", func(t *testing.T) {
		var m tea.Model = New(nil, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
		m = driveRune(m, '2')
		for _, k := range []rune{'I', 'B', 'P', 'A', 'C'} {
			m = driveRune(m, k)
		}
		if r := rootOf(t, m); r.licenses.detailTab != 4 {
			t.Errorf("after I/B/P/A/C expected detailTab=4 (Chain), got %d", r.licenses.detailTab)
		}
	})

	// ── Servers ────────────────────────────────────────────────────────────
	t.Run("servers/RHP-keys-cycle-subtabs", func(t *testing.T) {
		var m tea.Model = New(nil, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
		m = driveRune(m, '7')
		m = driveRune(m, 'H')
		if rootOf(t, m).servers.activeTab != serverTabHeartbeat {
			t.Errorf("expected serverTabHeartbeat after H, got %v", rootOf(t, m).servers.activeTab)
		}
		m = driveRune(m, 'P')
		if rootOf(t, m).servers.activeTab != serverTabProbe {
			t.Errorf("expected serverTabProbe after P")
		}
	})

	// ── Settings ───────────────────────────────────────────────────────────
	t.Run("settings/PVB-keys-push-overlay", func(t *testing.T) {
		var m tea.Model = New(nil, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
		m = driveRune(m, '9')
		// Drain init cmd
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'V'}})
		for i := 0; i < 4 && cmd != nil; i++ {
			msg := cmd()
			if msg == nil {
				break
			}
			updated, cmd = updated.Update(msg)
		}
		if r := rootOf(t, updated); len(r.overlays) == 0 {
			t.Errorf("expected confirm overlay after V key, got 0 overlays")
		}
	})

	// ── Help / Quit ────────────────────────────────────────────────────────
	t.Run("global/?-help+esc-cycle", func(t *testing.T) {
		var m tea.Model = New(nil, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
		m = driveRune(m, '?')
		if r := rootOf(t, m); len(r.overlays) != 1 {
			t.Errorf("expected help overlay after ?, got %d overlays", len(r.overlays))
		}
		// Dismiss
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if cmd != nil {
			if msg := cmd(); msg != nil {
				updated, _ = updated.Update(msg)
			}
		}
		if r := rootOf(t, updated); len(r.overlays) != 0 {
			t.Errorf("expected help dismissed after esc")
		}
	})

	// ── Overlays ──────────────────────────────────────────────────────────
	t.Run("overlays/all-instantiate-without-panic", func(t *testing.T) {
		overlays := map[string]Overlay{
			"confirm":     NewConfirmOverlay("id", "T", "B?", "OK", "C", false),
			"error":       NewErrorOverlay("E", "msg"),
			"input":       NewInputOverlay("id", "T", "p", 80),
			"quit":        NewQuitOverlay(true),
			"help":        NewHelpOverlay(),
			"revoke":      NewRevokeOverlay(uuid.New(), "alice@research"),
			"qr":          NewQROverlay(nil),
			"ok":          NewOKOverlay("Done", "Ok."),
		}
		for name, o := range overlays {
			if o.View() == "" {
				t.Errorf("overlay %s rendered empty", name)
			}
		}
	})

	// ── TODO (workflows existing in UI but no E2E yet) ─────────────────────
	t.Run("TODO/audit-detail-vp-scroll", func(t *testing.T) { t.Skip("vp scroll keys not asserted") })
	t.Run("TODO/wizard-step1-through-step8", func(t *testing.T) { t.Skip("only step rendering checked, not field validation") })
	t.Run("TODO/identity-regen-overlay-flow", func(t *testing.T) { t.Skip("R+overlay confirm not asserted") })
	t.Run("TODO/revocation-x-unrevoke-flow", func(t *testing.T) { t.Skip("x+overlay confirm not asserted") })
	t.Run("TODO/recipients-K-export-key-flow", func(t *testing.T) { t.Skip("K shortcut shown in detail but no handler") })
	t.Run("TODO/passphrase-prompt-3-attempts", func(t *testing.T) { t.Skip("max retry not enforced in tests") })

	// ── BLOCKED (requires upstream service work) ──────────────────────────
	t.Run("BLOCKED/settings-theme-persistence", func(t *testing.T) { t.Skip("svc.Settings.SetTheme not implemented") })
	t.Run("BLOCKED/settings-argon-persistence", func(t *testing.T) { t.Skip("svc.Settings.SetArgonPreset not implemented") })
	t.Run("BLOCKED/server-status-uptime-clock", func(t *testing.T) { t.Skip("requires real httpsrv lifecycle integration test") })
	t.Run("BLOCKED/probe-drawer-fingerprint-receive", func(t *testing.T) { t.Skip("requires real probe POST handler test") })

	// Force a no-op reference to silence unused-import warnings if all blocks
	// skip when the package is being explored standalone.
	_ = ent.Setting{}
}
