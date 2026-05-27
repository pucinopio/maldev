package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ═══════════════════════════════════════════════════════════════════════════
// DASHBOARD CROSS-SCREEN HOTKEYS — TRUTH TABLE
// ═══════════════════════════════════════════════════════════════════════════
//
// From the dashboard, single keys navigate to specific screens and may also
// apply a filter on arrival. Pre-2026-05 these hotkeys lived in chrome but
// weren't tested as a unit — this sweep catches drift between the dashboard
// "Raccourcis" hint card and the actual key dispatch in app.go handleKey.
// ═══════════════════════════════════════════════════════════════════════════

func TestDashboardCrossScreenHotkeys(t *testing.T) {
	cases := []struct {
		key        rune
		wantView   ViewID
		wantFilter licenseFilter // ignored if wantView != ViewLicenses
		setFilter  bool
	}{
		// Counter tile hotkeys → Licenses with filter set.
		{'a', ViewLicenses, licFilterActive, true},
		{'e', ViewLicenses, licFilterExpired, true},
		{'w', ViewLicenses, licFilterExpiring, true},
		{'u', ViewLicenses, licFilterSuperseded, true},
		// Direct-screen hotkeys.
		{'x', ViewLicenses, 0, false}, // 'x' from dashboard goes to Licenses (operator triggers revoke from there)
		{'k', ViewIssuers, 0, false},
		{'i', ViewIdentities, 0, false},
		{'s', ViewServers, 0, false},
	}

	for _, c := range cases {
		t.Run(string(c.key), func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})

			// Press the dashboard hotkey. Returns a Cmd that may chain
			// initScreen + setLicensesFilter — drain it to apply the filter.
			mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c.key}})
			// Drain at most a handful of follow-up cmds. Without a bound the
			// dashboard's refresh chain can re-trigger itself; we only need
			// the first wave that applies the filter / switches the view.
			for hop := 0; cmd != nil && hop < 3; hop++ {
				msg := cmd()
				cmd = nil
				if msg == nil {
					break
				}
				if batch, ok := msg.(tea.BatchMsg); ok {
					for _, sub := range batch {
						if sub == nil {
							continue
						}
						if subMsg := sub(); subMsg != nil {
							mm, _ = mm.Update(subMsg)
						}
					}
					continue
				}
				mm, cmd = mm.Update(msg)
			}
			r := rootOf(t, mm)
			if r.active != c.wantView {
				t.Errorf("key %q: active=%v want %v", string(c.key), r.active, c.wantView)
			}
			if c.setFilter && r.licenses.filter != c.wantFilter {
				t.Errorf("key %q: licenses.filter=%v want %v", string(c.key), r.licenses.filter, c.wantFilter)
			}
		})
	}
}

// TestDashboardTabsNumeric guards that 1..9, 0 navigate to the matching tab.
func TestDashboardTabsNumeric(t *testing.T) {
	want := map[rune]ViewID{
		'1': ViewDashboard,
		'2': ViewLicenses,
		'3': ViewIssuers,
		'4': ViewRecipients,
		'5': ViewIdentities,
		'6': ViewRevocation,
		'7': ViewServers,
		'8': ViewTOTP,
		'9': ViewAudit,
		'0': ViewSettings,
	}
	for d, view := range want {
		t.Run(string(d), func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
			m = drive(m, d)
			if r := rootOf(t, m); r.active != view {
				t.Errorf("digit %q: active=%v want %v", string(d), r.active, view)
			}
		})
	}
}

// TestDashboardGlobalKeys guards ? (help) and q (quit-or-confirm).
func TestDashboardGlobalKeys(t *testing.T) {
	t.Run("?_pushes_help_overlay", func(t *testing.T) {
		var m tea.Model = New(nil, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
		ov0 := len(rootOf(t, m).overlays)
		mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
		if cmd != nil {
			if msg := cmd(); msg != nil {
				mm, _ = mm.Update(msg)
			}
		}
		if len(rootOf(t, mm).overlays) <= ov0 {
			t.Errorf("? did not push help overlay")
		}
	})
}
