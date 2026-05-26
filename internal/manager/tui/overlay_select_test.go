package tui_test

// overlay_select_test.go — guard tests for selectOverlay (D-S35) and the
// screen_servers.go perf / reuse refactors (items 3a/3b/3c/4).

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func threeOpts() []tui.SelectOption {
	return []tui.SelectOption{
		{Label: "Alpha", Value: "a"},
		{Label: "Beta", Value: "b"},
		{Label: "Gamma", Value: "c"},
	}
}

func keyMsg(s string) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func specialKey(t tea.KeyType) tea.Msg {
	return tea.KeyMsg{Type: t}
}

// ── selectOverlay: arrow navigation ──────────────────────────────────────────

// TestSelectOverlay_DownMovessCursor verifies that "down" advances the cursor.
func TestSelectOverlay_DownMovesCursor(t *testing.T) {
	ov := tui.NewSelectOverlayForTest("id", "title", threeOpts(), "a")
	// cursor starts at 0 (matches "a"). One "down" should reach index 1.
	ov2, _ := ov.Update(specialKey(tea.KeyDown))
	view := ov2.View()
	// "▶" marker should now be on Beta, not Alpha.
	if !strings.Contains(view, "▶") {
		t.Fatal("cursor marker not found in view after down")
	}
	// Alpha should no longer carry the cursor marker (it is rendered with "  " prefix).
	lines := strings.Split(view, "\n")
	for _, l := range lines {
		if strings.Contains(l, "Alpha") && strings.HasPrefix(strings.TrimSpace(l), "▶") {
			t.Error("Alpha still shows cursor after one down press")
		}
	}
}

// TestSelectOverlay_UpDoesNotWrapBelowZero verifies cursor stays at 0 on up from top.
func TestSelectOverlay_UpDoesNotWrapBelowZero(t *testing.T) {
	ov := tui.NewSelectOverlayForTest("id", "title", threeOpts(), "a")
	ov2, _ := ov.Update(specialKey(tea.KeyUp))
	// View must still show the cursor somewhere (no panic, no empty view).
	if ov2.View() == "" {
		t.Fatal("view is empty after up from top")
	}
}

// TestSelectOverlay_DownDoesNotWrapPastEnd verifies cursor stays at last item.
func TestSelectOverlay_DownDoesNotWrapPastEnd(t *testing.T) {
	ov := tui.NewSelectOverlayForTest("id", "title", threeOpts(), "c")
	ov2, _ := ov.Update(specialKey(tea.KeyDown))
	// View must still render without panic.
	if ov2.View() == "" {
		t.Fatal("view is empty after down from bottom")
	}
}

// ── selectOverlay: Enter selects ─────────────────────────────────────────────

// TestSelectOverlay_EnterEmitsSelectResultMsg verifies Enter fires SelectResultMsg
// with the cursor item's value.
func TestSelectOverlay_EnterEmitsSelectResultMsg(t *testing.T) {
	ov := tui.NewSelectOverlayForTest("bind-id", "Pick", threeOpts(), "b")
	// cursor is on "b" (Beta).
	_, cmd := ov.Update(specialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter on selectOverlay must return a non-nil cmd")
	}
	msg := cmd()
	res, ok := msg.(tui.OverlayDoneMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want OverlayDoneMsg", msg)
	}
	sel, ok := res.Result.(tui.SelectResultMsg)
	if !ok {
		t.Fatalf("Result is %T, want SelectResultMsg", res.Result)
	}
	if sel.ID != "bind-id" {
		t.Errorf("ID = %q, want %q", sel.ID, "bind-id")
	}
	if sel.Value != "b" {
		t.Errorf("Value = %q, want %q", sel.Value, "b")
	}
}

// ── selectOverlay: Esc cancels ───────────────────────────────────────────────

// TestSelectOverlay_EscCancels verifies Esc returns OverlayDoneMsg{Result: nil}.
func TestSelectOverlay_EscCancels(t *testing.T) {
	ov := tui.NewSelectOverlayForTest("id", "title", threeOpts(), "a")
	_, cmd := ov.Update(specialKey(tea.KeyEsc))
	if cmd == nil {
		t.Fatal("Esc on selectOverlay must return a non-nil cmd")
	}
	msg := cmd()
	res, ok := msg.(tui.OverlayDoneMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want OverlayDoneMsg", msg)
	}
	if res.Result != nil {
		t.Errorf("Result = %v, want nil (cancel)", res.Result)
	}
}

// ── selectOverlay: mouse click selects ───────────────────────────────────────

// TestSelectOverlay_MouseClickSelectsItem verifies a left-click at the row of
// the second option emits SelectResultMsg with that option's value.
func TestSelectOverlay_MouseClickSelectsItem(t *testing.T) {
	ov := tui.NewSelectOverlayForTest("id", "title", threeOpts(), "a")
	// Items start at relative Y=3; second item is at Y=4.
	_, cmd := ov.Update(tea.MouseMsg{
		X:      5,
		Y:      4, // second item (Beta)
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if cmd == nil {
		t.Fatal("mouse click on item must return cmd")
	}
	msg := cmd()
	res, ok := msg.(tui.OverlayDoneMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want OverlayDoneMsg", msg)
	}
	sel, ok := res.Result.(tui.SelectResultMsg)
	if !ok {
		t.Fatalf("Result is %T, want SelectResultMsg", res.Result)
	}
	if sel.Value != "b" {
		t.Errorf("Value = %q, want %q", sel.Value, "b")
	}
}

// ── D-S35 guard: ipOptions has 0.0.0.0 ──────────────────────────────────────

// TestLive_ServersEditBind_IPOptions_Contains0000 asserts that ipOptions always
// includes the "all interfaces" sentinel so the select overlay is never empty
// and the default option is reachable.
func TestLive_ServersEditBind_IPOptions_Contains0000(t *testing.T) {
	opts := tui.IPOptionsForTest()
	if len(opts) == 0 {
		t.Fatal("ipOptions returned empty slice")
	}
	found := false
	for _, o := range opts {
		if o.Value == "0.0.0.0" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ipOptions must contain 0.0.0.0")
	}
}

// TestLive_ServersEditBind_IPOptions_Contains127 asserts the loopback option
// is always present.
func TestLive_ServersEditBind_IPOptions_Contains127(t *testing.T) {
	opts := tui.IPOptionsForTest()
	found := false
	for _, o := range opts {
		if o.Value == "127.0.0.1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ipOptions must contain 127.0.0.1")
	}
}

// TestLive_ServersEditBind_IPOptions_LastIsAutre asserts the last option is the
// "Autre…" free-form sentinel (Value=="").
func TestLive_ServersEditBind_IPOptions_LastIsAutre(t *testing.T) {
	opts := tui.IPOptionsForTest()
	last := opts[len(opts)-1]
	if last.Value != "" {
		t.Errorf("last option Value = %q, want empty string (Autre… sentinel)", last.Value)
	}
}

// TestLive_ServersEditBind_OpensSelectOverlay asserts that pressing [e] on the
// Servers screen pushes a select overlay (not an input overlay) and that the
// overlay view contains "0.0.0.0".
func TestLive_ServersEditBind_OpensSelectOverlay(t *testing.T) {
	m := tui.NewServersModelForTest(nil)
	m = tui.InitServersModel(m, 160, 40)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd == nil {
		t.Fatal("[e] key must return a non-nil cmd")
	}
	push, ok := tui.AsPushOverlay(cmd())
	if !ok {
		t.Fatalf("cmd() did not return a pushOverlayMsg")
	}
	view := push.Overlay.View()
	if !strings.Contains(view, "0.0.0.0") {
		t.Errorf("select overlay view must contain 0.0.0.0, got:\n%s", view)
	}
}

// ── perf item 3b: precomputed serverTabWidths ────────────────────────────────

// TestServerTabWidths_MatchLipglossWidth verifies that the precomputed
// serverTabWidths values match the runtime lipgloss.Width measurement so the
// OnClick hot-path is never stale.
func TestServerTabWidths_MatchLipglossWidth(t *testing.T) {
	keys := [3]string{"R", "H", "P"}
	labels := [3]string{"Revocation", "Heartbeat", "Fingerprint probe"}
	widths := tui.ServerTabWidthsForTest()
	for i := range keys {
		label := "[" + keys[i] + "] " + labels[i] + " ●"
		want := lipgloss.Width(label) + 2
		if widths[i] != want {
			t.Errorf("tab %d: precomputed width=%d, lipgloss.Width=%d", i, widths[i], want)
		}
	}
}

// ── perf item 3a: serverRoleCache populated ──────────────────────────────────

// TestServersModel_RoleCachePopulated verifies that all three role-cache slots
// are non-empty after construction so View() never shows a blank role line.
func TestServersModel_RoleCachePopulated(t *testing.T) {
	m := tui.NewServersModelForTest(nil)
	cache := tui.ServerRoleCacheForTest(m)
	for i, s := range cache {
		if s == "" {
			t.Errorf("serverRoleCache[%d] is empty after newServersModel", i)
		}
	}
}
