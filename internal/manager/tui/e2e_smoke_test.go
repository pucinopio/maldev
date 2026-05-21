package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
			{'3', "Issuers"},
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
		m, _ = m.Update(tea.MouseMsg{X: 60, Y: 1, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, Type: tea.MouseLeft})
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

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
