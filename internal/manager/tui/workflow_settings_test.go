package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestWorkflow_SettingsTogglesPersistAcrossRestart is the regression guard
// for the operator-reported "toutes les options doivent être persistentes
// entre les démarrages". Each settings toggle is exercised via its hotkey
// and the new value is checked back via svc.Settings.Get — proving the
// change reached the DB and would survive a process restart.
func TestWorkflow_SettingsTogglesPersistAcrossRestart(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = driveRune(m, '0') // Settings tab (10th = "0") — switch FIRST so the
	// SettingsLoadedMsg routes to the settings screen rather than being
	// dropped at the active Dashboard screen.
	for _, msg := range flattenCmd(loadSettingsCmd(svc)) {
		m, _ = m.Update(msg)
	}

	// Capture baseline values so we can flip and verify.
	base, err := svc.Settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		key      string
		fieldGet func() bool
		label    string
	}{
		{"Q", func() bool { r, _ := svc.Settings.Get(ctx); return r.ConfirmQuitWithServers }, "confirm_quit_with_servers"},
		{"U", func() bool { r, _ := svc.Settings.Get(ctx); return r.AutoStartServers }, "auto_start_servers"},
		{"S", func() bool { r, _ := svc.Settings.Get(ctx); return r.StopServersOnExit }, "stop_servers_on_exit"},
		{"G", func() bool { r, _ := svc.Settings.Get(ctx); return r.BoldSaturated }, "bold_saturated"},
		{"D", func() bool { r, _ := svc.Settings.Get(ctx); return r.ComfortDensity }, "comfort_density"},
		{"T", func() bool { r, _ := svc.Settings.Get(ctx); return r.TimestampsLocal }, "timestamps_local"},
	}
	_ = base

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			before := tc.fieldGet()
			// Drive the hotkey then iteratively drain every follow-up cmd:
			// keyhandler → settingsToggleMsg → settingsPersistCmd →
			// settingsPersistedMsg. Each step may emit a new cmd; flatten
			// recursively so the actual svc.Settings.Update fires.
			mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
			for cmd != nil {
				var next tea.Cmd
				for _, msg := range flattenCmd(cmd) {
					var c tea.Cmd
					mm, c = mm.Update(msg)
					if c != nil {
						next = c
					}
				}
				cmd = next
			}
			after := tc.fieldGet()
			if before == after {
				t.Errorf("%s: value did not flip (before=%v after=%v)", tc.label, before, after)
			}
		})
	}
}

// TestWorkflow_SettingsViewDropsUselessDashboardLine is the regression
// guard for the operator complaint "l'option ouvrir directement le
// dashboard me semble inutile". Pre-fix the toggle was hardcoded ON in
// the View() with no underlying setting; it now no longer renders.
func TestWorkflow_SettingsViewDropsUselessDashboardLine(t *testing.T) {
	svc, _ := newTestServices(t)
	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	for _, msg := range flattenCmd(loadSettingsCmd(svc)) {
		m, _ = m.Update(msg)
	}
	m = driveRune(m, '0')
	view := m.View()
	if strings.Contains(view, "ouvrir directement Dashboard") {
		t.Error("settings view still renders the useless 'ouvrir directement Dashboard' toggle")
	}
}

// TestWorkflow_SettingsRestoreActionReachable is the regression guard for
// "il est possible de faire un backup de la DB mais pas de la réimporter".
// The settings screen now wires [I] to push a restore input overlay
// (stub implementation pending the backup format spec — symmetric with
// the existing [B] backup path).
func TestWorkflow_SettingsRestoreActionReachable(t *testing.T) {
	svc, _ := newTestServices(t)
	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	for _, msg := range flattenCmd(loadSettingsCmd(svc)) {
		m, _ = m.Update(msg)
	}
	m = driveRune(m, '0')
	// [I] emits settingsActionMsg{kind:"restore"} which Update dispatches
	// to a pushOverlayMsg{newInputOverlay(...)} on the second tick.
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("I")})
	if cmd == nil {
		t.Fatal("[I] on settings produced no cmd")
	}
	var pushed pushOverlayMsg
	for cmd != nil {
		var next tea.Cmd
		for _, msg := range flattenCmd(cmd) {
			if p, ok := msg.(pushOverlayMsg); ok {
				pushed = p
				next = nil
				break
			}
			var c tea.Cmd
			mm, c = mm.Update(msg)
			if c != nil {
				next = c
			}
		}
		cmd = next
		if pushed.overlay != nil {
			break
		}
	}
	if pushed.overlay == nil {
		t.Fatal("[I] never produced a pushOverlayMsg")
	}
	if !strings.Contains(pushed.overlay.View(), "Restaurer un backup") {
		t.Errorf("restore input overlay title missing; got view: %s", pushed.overlay.View())
	}
}
