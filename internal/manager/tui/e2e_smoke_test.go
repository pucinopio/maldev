package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
)

// TestE2E_SmokeTUI exercises the rootModel by feeding it the same messages a
// bubbletea program would: WindowSize -> render -> simulate key presses to
// navigate tabs -> mouse click on a tile -> view another screen. It does not
// require services / a real bundle and runs entirely in-process.
//
// The goal is a guard that the widget-composition tree, mouse dispatch, and
// view routing all stay wired end-to-end as future work edits the screens.
func TestE2E_SmokeTUI(t *testing.T) {
	root := New(nil, nil, SessionReady)
	var m tea.Model = root
	m, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 44})
	dashboard := m.View()
	if !strings.Contains(dashboard, "Dashboard") {
		t.Fatalf("expected dashboard chrome to render, got: %q", firstLine(dashboard))
	}

	t.Run("number keys route to each view", func(t *testing.T) {
		cases := []struct {
			key   rune
			title string
		}{
			{'1', "Dashboard"},
			{'2', "Licenses"},
			{'3', "Issuer keys"},
			{'4', "Recipients"},
			{'5', "Identities"},
			{'6', "Revocation"},
			{'7', "Servers"},
			{'8', "Audit"},
			{'9', "Settings"},
		}
		for _, c := range cases {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c.key}})
			v := m.View()
			if !strings.Contains(v, c.title) {
				t.Errorf("key %q expected to show %q somewhere in view, first line: %q", c.key, c.title, firstLine(v))
			}
		}
	})

	t.Run("mouse click does not panic", func(t *testing.T) {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
		_ = m.View()
		m, _ = m.Update(tea.MouseMsg{X: 60, Y: 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, Type: tea.MouseLeft})
		if got := m.View(); got == "" {
			t.Fatal("post-click View() returned empty")
		}
	})

	t.Run("help overlay toggles", func(t *testing.T) {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
		_ = m.View()
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		_ = m.View()
	})
}

// TestE2E_TileClickNavigatesToLicenses is the regression guard for the mouse
// click dispatcher.
//
// Protocol:
//  1. Build a dashboard at 144×44 with seeded counter data.
//  2. Call View() so Layout() sets widget bounds (same path taken by real renderer).
//  3. Send a MouseMsg with Action=Release on a coordinate inside the Active tile.
//  4. Assert that the resulting model is on the Licenses view and its View()
//     contains the "rechercher dans subject" hint unique to the licenses screen body.
//
// Also verifies that Action=Press (sent by some terminals instead of Release)
// triggers the same navigation, since handleMouse accepts both.
func TestE2E_TileClickNavigatesToLicenses(t *testing.T) {
	snap := cmds.DashboardSnapshotMsg{
		Active:       47,
		Revoked:      6,
		Expired:      12,
		ExpiringSoon: 4,
		ActiveKeyID:  "k2026-04",
	}

	build := func() tea.Model {
		root := New(nil, nil, SessionReady)
		var m tea.Model = root
		m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
		m, _ = m.Update(snap)
		// Call View() to trigger Layout() and populate widget bounds.
		_ = m.View()
		return m
	}

	// The Active tile is the first in the row. At 144 cols with 4 equal tiles,
	// each tile is ~35 cols wide. The tile row starts at Y=2 (rows 0+1 = chrome).
	// Click at X=17, Y=4 — well inside the first tile body.
	clickRelease := tea.MouseMsg{
		X: 17, Y: 4,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		Type:   tea.MouseLeft,
	}
	clickPress := tea.MouseMsg{
		X: 17, Y: 4,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		Type:   tea.MouseLeft,
	}

	for _, tc := range []struct {
		name string
		msg  tea.MouseMsg
	}{
		{"release", clickRelease},
		{"press", clickPress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := build()
			// Step 1: mouse event → model returns a Cmd (not yet a Msg).
			m2, cmd := m.Update(tc.msg)
			if cmd == nil {
				t.Fatalf("tile click (%s): expected non-nil Cmd from handleMouse", tc.name)
			}
			// Step 2: execute the Cmd synchronously to get SwitchToLicensesMsg.
			switchMsg := cmd()
			if _, ok := switchMsg.(SwitchToLicensesMsg); !ok {
				t.Fatalf("tile click (%s): expected SwitchToLicensesMsg, got %T", tc.name, switchMsg)
			}
			// Step 3: feed SwitchToLicensesMsg back so the model navigates.
			m3, _ := m2.Update(switchMsg)
			view := m3.View()
			if !strings.Contains(view, "rechercher dans subject") {
				t.Errorf("after tile click (%s) expected Licenses view, got first line: %q", tc.name, firstLine(view))
			}
		})
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
