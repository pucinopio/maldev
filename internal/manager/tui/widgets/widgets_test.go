package widgets

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// TestTabBar_OnClick verifies that clicking the i-th tab fires
// SwitchViewMsg with the matching tab ID. Walks the tab strip by the
// recorded widths after a View() pass to handle wide-rune labels.
func TestTabBar_OnClick(t *testing.T) {
	tabs := []TabItem{
		{ID: "dashboard", Label: "Dashboard"},
		{ID: "licenses", Label: "Licenses"},
		{ID: "issuers", Label: "Issuer keys"},
	}
	tb := NewTabBar(tabs, "dashboard")
	tb.Layout(core.Rect{X: 0, Y: 0, W: 100, H: 1})
	_ = tb.View() // populates tabWidths

	cursor := 0
	for i, want := range tabs {
		w := tb.tabWidths[i]
		hitX := cursor + w/2
		cmd := tb.OnClick(hitX, 0, tea.MouseButtonLeft)
		if cmd == nil {
			t.Errorf("tab %d (%s): OnClick(%d) returned nil", i, want.ID, hitX)
			cursor += w
			continue
		}
		msg := cmd().(SwitchViewMsg)
		if msg.ID != want.ID {
			t.Errorf("tab %d: SwitchViewMsg.ID = %q, want %q", i, msg.ID, want.ID)
		}
		cursor += w
	}
}

// TestTabBar_OnClick_OutOfRange — clicking past the last tab returns nil.
func TestTabBar_OnClick_OutOfRange(t *testing.T) {
	tb := NewTabBar([]TabItem{{ID: "a", Label: "A"}}, "a")
	tb.Layout(core.Rect{W: 100, H: 1})
	_ = tb.View()
	if cmd := tb.OnClick(99, 0, tea.MouseButtonLeft); cmd != nil {
		t.Errorf("click past last tab should return nil, got cmd %T", cmd())
	}
}

// TestTile_OnClick fires the OnPress callback.
func TestTile_OnClick(t *testing.T) {
	called := false
	tile := NewTile("Actives [a]", 42, "active licences", "#39ff88",
		func() tea.Cmd { called = true; return nil })
	tile.Layout(core.Rect{X: 0, Y: 0, W: 20, H: 4})
	tile.OnClick(5, 2, tea.MouseButtonLeft)
	if !called {
		t.Error("tile OnClick did not invoke OnPress")
	}
}

// TestTile_View_BasicShape asserts Tile.View renders a bordered block
// containing the label, value, and footer.
func TestTile_View_BasicShape(t *testing.T) {
	tile := NewTile("Actives [a]", 42, "active licences", "#39ff88", nil)
	tile.Layout(core.Rect{X: 0, Y: 0, W: 24, H: 4})
	out := tile.View()
	if !strings.Contains(out, "Actives") {
		t.Errorf("Tile output missing 'Actives':\n%s", out)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("Tile output missing value '42':\n%s", out)
	}
	if !strings.Contains(out, "active licences") {
		t.Errorf("Tile output missing footer:\n%s", out)
	}
}

// TestStatusBar_Layout verifies the bar emits the hint pairs.
func TestStatusBar_View(t *testing.T) {
	sb := NewStatusBar(
		KeyHint{Key: "q", Desc: "quit"},
		KeyHint{Key: "?", Desc: "help"},
	)
	sb.Layout(core.Rect{X: 0, Y: 0, W: 80, H: 1})
	out := sb.View()
	if !strings.Contains(out, "q") || !strings.Contains(out, "quit") {
		t.Errorf("status bar missing key/desc:\n%s", out)
	}
	if !strings.Contains(out, "~ tour") {
		t.Errorf("status bar should always render the '~ tour' pill:\n%s", out)
	}
}

// TestStatusBar_Update is a no-op that returns the bar unchanged.
func TestStatusBar_Update_NoOp(t *testing.T) {
	sb := NewStatusBar()
	w, cmd := sb.Update(tea.KeyMsg{})
	if w != sb || cmd != nil {
		t.Errorf("Update should be a no-op, got w=%v cmd=%v", w, cmd)
	}
}

// TestSplitTileTitle covers the helper that parses "Label [k]" → ("Label", "k").
func TestSplitTileTitle(t *testing.T) {
	cases := []struct {
		in, wantLabel, wantKey string
	}{
		{"Actives [a]", "Actives", "a"},
		{"No hotkey", "No hotkey", ""},
		{"Empty []", "Empty", ""},
		{"Multi [ab]", "Multi", "ab"},
	}
	for _, c := range cases {
		label, key := splitTileTitle(c.in)
		if label != c.wantLabel || key != c.wantKey {
			t.Errorf("splitTileTitle(%q) = (%q,%q), want (%q,%q)",
				c.in, label, key, c.wantLabel, c.wantKey)
		}
	}
}
