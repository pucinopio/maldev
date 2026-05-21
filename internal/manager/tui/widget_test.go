package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui"
	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
	"github.com/oioio-space/maldev/internal/manager/tui/widgets"
)

func TestRectContains(t *testing.T) {
	r := tui.Rect{X: 5, Y: 10, W: 20, H: 5}
	cases := []struct {
		x, y int
		want bool
	}{
		{5, 10, true},   // top-left corner
		{24, 14, true},  // bottom-right corner
		{4, 10, false},  // one cell left
		{5, 15, false},  // one row below
		{25, 10, false}, // one cell right
	}
	for _, c := range cases {
		got := r.Contains(c.x, c.y)
		if got != c.want {
			t.Errorf("Rect.Contains(%d,%d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

// TestWidgetFlexHorizontalSnapshot verifies three fixed-width Text widgets
// placed side by side produce the correct combined render width.
func TestWidgetFlexHorizontalSnapshot(t *testing.T) {
	w1 := widgets.NewText("AAA", lipgloss.NewStyle())
	w2 := widgets.NewText("BBB", lipgloss.NewStyle())
	w3 := widgets.NewText("CCC", lipgloss.NewStyle())

	flex := tui.NewFlex(tui.Horizontal, 0,
		tui.FlexChild{W: w1, Min: 10, Flex: 1},
		tui.FlexChild{W: w2, Min: 10, Flex: 1},
		tui.FlexChild{W: w3, Min: 10, Flex: 1},
	)
	flex.Layout(tui.Rect{X: 0, Y: 0, W: 30, H: 5})
	got := flex.View()
	compareOrUpdate(t, "flex_horizontal", got)
}

// TestWidgetFlexVerticalSnapshot verifies two stacked widgets fill height correctly.
func TestWidgetFlexVerticalSnapshot(t *testing.T) {
	w1 := widgets.NewText("TOP", lipgloss.NewStyle())
	w2 := widgets.NewText("BOT", lipgloss.NewStyle())

	flex := tui.NewFlex(tui.Vertical, 1,
		tui.FlexChild{W: w1, Min: 3, Flex: 1},
		tui.FlexChild{W: w2, Min: 3, Flex: 1},
	)
	flex.Layout(tui.Rect{X: 0, Y: 0, W: 20, H: 10})
	got := flex.View()
	compareOrUpdate(t, "flex_vertical", got)
}

// TestDashboardWidgetSnapshot verifies the widget-based dashboard renders
// without panicking and produces stable output.
func TestDashboardWidgetSnapshot(t *testing.T) {
	snap := cmds.DashboardSnapshotMsg{
		Active: 5, Revoked: 1, Expired: 2, ExpiringSoon: 0,
		ActiveKeyID: "k1", ActiveKeyName: "test-key", ActiveKeyFingerprint: "aa:bb",
		Servers:     []cmds.ServerStatus{},
		RecentAudit: []cmds.AuditEntry{{Kind: "license.issue", Actor: "op"}},
	}
	root := tui.New(nil, nil, tui.SessionReady)
	m := initModel(root, snap)
	compareOrUpdate(t, "dashboard_widget", m.View())
}

// TestTileView verifies that Tile.View renders the header, value, and footer
// in prototype order (header row first, big value second, dim footer third).
func TestTileView(t *testing.T) {
	tile := widgets.NewTile("Actives [a]", 47, "signées par la clé active",
		lipgloss.Color("#39ff88"), nil)
	tile.Layout(tui.Rect{W: 24, H: 5})
	got := tile.View()

	// Header label must appear before the value.
	iLabel := strings.Index(got, "Actives")
	iValue := strings.Index(got, "47")
	if iLabel < 0 {
		t.Error("Tile.View: label 'Actives' not found")
	}
	if iValue < 0 {
		t.Error("Tile.View: value '47' not found")
	}
	if iLabel > iValue {
		t.Errorf("Tile.View: label appears after value (label@%d value@%d)", iLabel, iValue)
	}
	// Hotkey extracted from title must appear.
	if !strings.Contains(got, "[a]") {
		t.Error("Tile.View: hotkey '[a]' not found")
	}
	// Footer must appear.
	if !strings.Contains(got, "signées") {
		t.Error("Tile.View: footer text not found")
	}
}

// TestTileViewNoHotkey verifies a title with no "[k]" suffix renders cleanly.
func TestTileViewNoHotkey(t *testing.T) {
	tile := widgets.NewTile("Plain title", 0, "", lipgloss.Color("#00f0ff"), nil)
	tile.Layout(tui.Rect{W: 20, H: 4})
	got := tile.View()
	if !strings.Contains(got, "Plain title") {
		t.Errorf("Tile.View: label 'Plain title' not found in %q", got)
	}
}

// TestStatusBarTourPill verifies the "~ tour" pill appears on the right of the
// status bar, after all hint chips.
func TestStatusBarTourPill(t *testing.T) {
	sb := widgets.NewStatusBar(
		widgets.KeyHint{Key: "n", Desc: "new"},
		widgets.KeyHint{Key: "q", Desc: "quit"},
	)
	sb.Layout(tui.Rect{W: 80, H: 1})
	got := sb.View()

	iTour := strings.Index(got, "~ tour")
	if iTour < 0 {
		t.Fatal("StatusBar: '~ tour' pill not found")
	}
	iHint := strings.Index(got, "new")
	if iHint < 0 {
		t.Fatal("StatusBar: hint text 'new' not found")
	}
	if iHint > iTour {
		t.Errorf("StatusBar: hints appear after '~ tour' (hint@%d tour@%d)", iHint, iTour)
	}
}

// TestWidgetTabBarClickRouting verifies that clicking at a given X coordinate
// in the tab strip returns the correct SwitchViewMsg.
func TestWidgetTabBarClickRouting(t *testing.T) {
	items := []widgets.TabItem{
		{ID: "alpha", Label: "Alpha"},
		{ID: "beta", Label: "Beta"},
		{ID: "gamma", Label: "Gamma"},
	}
	tb := widgets.NewTabBar(items, "alpha")
	tb.Layout(tui.Rect{X: 0, Y: 0, W: 60, H: 1})
	// Render once to populate tabWidths.
	_ = tb.View()

	// Click somewhere clearly in the first tab (x=2, relative to bounds).
	cmd := tb.OnClick(2, 0, tea.MouseButtonLeft)
	if cmd == nil {
		t.Fatal("expected non-nil cmd for click on first tab")
	}
	msg := cmd()
	sv, ok := msg.(widgets.SwitchViewMsg)
	if !ok {
		t.Fatalf("expected SwitchViewMsg, got %T", msg)
	}
	if sv.ID != "alpha" {
		t.Errorf("expected ID=alpha, got %q", sv.ID)
	}
}
