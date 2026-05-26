package tui

// Regression guards for D-S3 / D-S5 — digit-key collision between the chrome
// tab-nav (digit ⇒ goto Nth view) and screen-local digit hotkeys (Settings
// argon presets [1][2][3], Servers log filter [1][2][3][4]).
//
// Before the fix, pressing `1` on the Settings or Servers screen jumped to
// the Dashboard (tab index 0) instead of triggering the in-screen action.
// The fix gates the chrome digit interception via screenConsumesDigit() in
// app.go.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// rootAt drives the root model to the named view via the chrome digit shortcut
// (which still works for views that DON'T consume digits locally), then
// returns the resulting model for further driving.
func rootAt(t *testing.T, view ViewID) tea.Model {
	t.Helper()
	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	// Map ViewID → tab digit per viewOrder.
	idx := -1
	for i, v := range viewOrder {
		if v == view {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("view %q not in viewOrder", view)
	}
	var digit rune
	switch {
	case idx < 9:
		digit = '1' + rune(idx)
	case idx == 9:
		digit = '0'
	default:
		// Fall back to tab cycling for positions beyond 10.
		for i := 0; i <= idx; i++ {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		}
		return m
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{digit}})
	return m
}

// TestDigitCollision_SettingsConsumes1to3 — pressing 1/2/3 on Settings must
// stay on Settings, not jump to the matching tab. The screen handler picks
// up the digit for its argon presets.
func TestDigitCollision_SettingsConsumes1to3(t *testing.T) {
	for _, d := range []rune{'1', '2', '3'} {
		t.Run(string(d), func(t *testing.T) {
			m := rootAt(t, ViewSettings)
			if got := m.(rootModel).active; got != ViewSettings {
				t.Fatalf("setup: active=%v, want ViewSettings", got)
			}
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{d}})
			if got := m.(rootModel).active; got != ViewSettings {
				t.Fatalf("digit %q on Settings: active=%v, want ViewSettings (chrome stole it)", d, got)
			}
		})
	}
}

// TestDigitCollision_SettingsDoesNotConsume4 — guards that the gating is
// narrow: digits the screen DOESN'T bind (4..9) still navigate to the
// matching tab. Otherwise the operator would lose tab-nav from Settings.
func TestDigitCollision_SettingsDoesNotConsume4(t *testing.T) {
	m := rootAt(t, ViewSettings)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	if got := m.(rootModel).active; got != ViewRecipients {
		t.Fatalf("'4' on Settings: active=%v, want ViewRecipients (tab 4)", got)
	}
}

// TestDigitCollision_ServersConsumes1to4 — same pattern as Settings, but the
// Servers screen reserves 1..4 for its probe log filter.
func TestDigitCollision_ServersConsumes1to4(t *testing.T) {
	for _, d := range []rune{'1', '2', '3', '4'} {
		t.Run(string(d), func(t *testing.T) {
			m := rootAt(t, ViewServers)
			if got := m.(rootModel).active; got != ViewServers {
				t.Fatalf("setup: active=%v, want ViewServers", got)
			}
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{d}})
			if got := m.(rootModel).active; got != ViewServers {
				t.Fatalf("digit %q on Servers: active=%v, want ViewServers (chrome stole it)", d, got)
			}
		})
	}
}

// TestDigitCollision_ServersDoesNotConsume5 — digit 5 is not bound by the
// Servers screen, so chrome MUST steal it for tab-nav (→ Identities).
func TestDigitCollision_ServersDoesNotConsume5(t *testing.T) {
	m := rootAt(t, ViewServers)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	if got := m.(rootModel).active; got != ViewIdentities {
		t.Fatalf("'5' on Servers: active=%v, want ViewIdentities (tab 5)", got)
	}
}

// TestDigitCollision_DashboardStillNavigates — guards against over-gating:
// the Dashboard does NOT consume digits, so 1..9 must still navigate from it.
func TestDigitCollision_DashboardStillNavigates(t *testing.T) {
	m := rootAt(t, ViewDashboard)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'7'}})
	if got := m.(rootModel).active; got != ViewServers {
		t.Fatalf("'7' on Dashboard: active=%v, want ViewServers (tab 7)", got)
	}
}
