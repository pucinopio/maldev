package widgets

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

// TestButton_ClickAndKey: OnClick + hotkey press both invoke OnPress.
func TestButton_ClickAndKey(t *testing.T) {
	var pressed int
	b := NewButton("Save", "s", func() tea.Cmd {
		pressed++
		return nil
	})
	b.Layout(core.Rect{X: 0, Y: 0, W: 12, H: 3})
	b.OnClick(2, 1, tea.MouseButtonLeft)
	if pressed != 1 {
		t.Errorf("OnClick: pressed = %d, want 1", pressed)
	}
	// Hotkey only fires when focused.
	b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if pressed != 1 {
		t.Errorf("unfocused hotkey: pressed = %d, want 1 (unchanged)", pressed)
	}
	b.Focus()
	if !b.Focused() {
		t.Error("Focus() did not flip Focused()")
	}
	b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if pressed != 2 {
		t.Errorf("focused hotkey: pressed = %d, want 2", pressed)
	}
	b.Blur()
	if b.Focused() {
		t.Error("Blur() did not flip Focused() off")
	}
}

// TestButton_View renders the label + bracketed hotkey.
func TestButton_View(t *testing.T) {
	b := NewButton("Save", "s", nil)
	b.Layout(core.Rect{W: 12, H: 3})
	out := b.View()
	if !strings.Contains(out, "[s]") {
		t.Errorf("Button view missing [s] hotkey:\n%s", out)
	}
	if !strings.Contains(out, "Save") {
		t.Errorf("Button view missing label:\n%s", out)
	}
}

// TestText_RenderAndLayout — Text widget echoes its content under the
// provided style, clipping nothing.
func TestText_RenderAndLayout(t *testing.T) {
	tx := NewText("hello world", lipgloss.NewStyle())
	tx.Layout(core.Rect{X: 0, Y: 0, W: 20, H: 1})
	if got := tx.Bounds().W; got != 20 {
		t.Errorf("Bounds.W = %d, want 20", got)
	}
	if !strings.Contains(tx.View(), "hello world") {
		t.Errorf("Text view missing content:\n%s", tx.View())
	}
}

// TestSpacer is a no-op widget that fills empty space.
func TestSpacer_NoOpUpdate(t *testing.T) {
	s := NewSpacer()
	s.Layout(core.Rect{X: 0, Y: 0, W: 10, H: 5})
	w, cmd := s.Update(tea.KeyMsg{})
	if w != s || cmd != nil {
		t.Error("Spacer.Update should be a no-op")
	}
	out := s.View()
	if out == "" {
		t.Error("Spacer.View should produce non-empty output (blank fill)")
	}
}

// TestWrappedTable_RowClick — clicking row N produces RowClickedMsg{N}.
// Header sits at relative Y=0; data rows start at Y=1.
func TestWrappedTable_RowClick(t *testing.T) {
	cols := []table.Column{{Title: "A", Width: 5}, {Title: "B", Width: 5}}
	rows := []table.Row{{"r0a", "r0b"}, {"r1a", "r1b"}, {"r2a", "r2b"}}
	tm := table.New(table.WithColumns(cols), table.WithRows(rows))
	wt := NewWrappedTable(tm)
	wt.Layout(core.Rect{X: 0, Y: 0, W: 20, H: 5})

	cases := []struct {
		y    int
		want int // -1 = expect nil cmd
	}{
		{0, -1}, // header row, no msg
		{1, 0},
		{2, 1},
		{3, 2},
		{4, -1}, // past last row
	}
	for _, c := range cases {
		cmd := wt.OnClick(0, c.y, tea.MouseButtonLeft)
		if c.want < 0 {
			if cmd != nil {
				t.Errorf("y=%d: got cmd, want nil", c.y)
			}
			continue
		}
		if cmd == nil {
			t.Errorf("y=%d: got nil, want RowClickedMsg{%d}", c.y, c.want)
			continue
		}
		m := cmd().(RowClickedMsg)
		if m.Index != c.want {
			t.Errorf("y=%d: RowClickedMsg.Index = %d, want %d", c.y, m.Index, c.want)
		}
	}
}

// TestWrappedTextInput_OnClick focuses the input.
func TestWrappedTextInput_OnClick(t *testing.T) {
	ti := textinput.New()
	wti := NewWrappedTextInput(ti)
	wti.Layout(core.Rect{W: 30, H: 1})
	if wti.Focused() {
		t.Fatal("fresh textinput should be unfocused")
	}
	wti.OnClick(0, 0, tea.MouseButtonLeft)
	if !wti.Focused() {
		t.Error("OnClick should focus the input")
	}
	wti.Blur()
	if wti.Focused() {
		t.Error("Blur should unfocus")
	}
}

// TestWrappedTextInput_Value reads back what's typed.
func TestWrappedTextInput_Value(t *testing.T) {
	ti := textinput.New()
	ti.SetValue("hello")
	wti := NewWrappedTextInput(ti)
	if got := wti.Value(); got != "hello" {
		t.Errorf("Value() = %q, want %q", got, "hello")
	}
}

// TestWrappedViewport_SetContent + View() outputs the seeded content.
func TestWrappedViewport_SetContent(t *testing.T) {
	wv := NewWrappedViewport("initial content")
	wv.Layout(core.Rect{W: 40, H: 5})
	if !strings.Contains(wv.View(), "initial") {
		t.Errorf("viewport view missing initial content:\n%s", wv.View())
	}
	wv.SetContent("replaced text")
	if !strings.Contains(wv.View(), "replaced") {
		t.Errorf("viewport view missing replaced content:\n%s", wv.View())
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
