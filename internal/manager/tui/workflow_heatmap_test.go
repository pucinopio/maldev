package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TestHeatmap_CellStylesDifferByActivity is the regression guard for the
// operator-reported "la map d'activité sur 91j sur le dashboard ne
// fonctionne pas". Pre-fix the cell styles (heatEmpty, heatGreen, …)
// were captured as package-level vars BEFORE theme.go's init()
// reseeded the underlying Mute/FgGreen etc., so every cell rendered
// with the empty zero lipgloss.Style — the heatmap looked uniform
// regardless of activity.
//
// The fix routes cell colours through heatCellStyle() at render time
// so the active palette is always used. The test inspects each style's
// resolved foreground colour to confirm they differ from each other.
func TestHeatmap_CellStylesDifferByActivity(t *testing.T) {
	fg := func(s lipgloss.Style) string {
		// GetForeground returns a TerminalColor; assert to lipgloss.Color
		// (the concrete type for hex strings) so we can compare values.
		if c, ok := s.GetForeground().(lipgloss.Color); ok {
			return string(c)
		}
		return ""
	}

	// Reset to a known apparence so the bold check is deterministic.
	savedB, savedC, savedT := apparenceBold, apparenceComfort, apparenceLocalTS
	t.Cleanup(func() { ApplyApparence(savedB, savedC, savedT) })
	ApplyApparence(true, false, false)

	empty := heatCellStyle(0, 0, false)
	weak := heatCellStyle(1, 0, false)
	strong := heatCellStyle(3, 0, false)
	red := heatCellStyle(0, 1, false)

	// Every cell must have a non-empty foreground. Pre-fix every bucket
	// resolved to the zero Style with no fg, so cells rendered colourless.
	for name, st := range map[string]lipgloss.Style{
		"empty": empty, "weak": weak, "strong": strong, "red": red,
	} {
		if fg(st) == "" {
			t.Errorf("%s: foreground unset — cell would render colourless", name)
		}
	}
	// empty, weak (=strong), red must each resolve to a DISTINCT colour
	// so the operator can tell quiet days from active days from
	// renewal-dense days at a glance.
	if fg(empty) == fg(weak) {
		t.Errorf("empty and weak share foreground %q — heatmap activity invisible", fg(empty))
	}
	if fg(weak) == fg(red) {
		t.Errorf("weak and red share foreground %q — renewal-dense days indistinguishable", fg(weak))
	}
	// strong vs weak: same green palette by design, but strong is BOLD so
	// dense weeks visually pop while sharing the activity colour.
	if fg(strong) != fg(weak) {
		t.Errorf("strong fg %q should equal weak fg %q (same green ramp)", fg(strong), fg(weak))
	}
	if !strong.GetBold() {
		t.Error("dense-week (strong) cell not bold under bold_saturated=on")
	}
	if weak.GetBold() {
		t.Error("weak cell should not be bold")
	}
}

// TestHeatmap_FullGridRendersInDashboard verifies the full 7-row grid +
// legend is present in the rendered dashboard view. Pre-fix the heatBox
// FlexChild had Max:10 which clipped the legend (or the last day row)
// on the standard layout.
func TestHeatmap_FullGridRendersInDashboard(t *testing.T) {
	svc, _ := newTestServices(t)
	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	for _, msg := range flattenCmd(m.Init()) {
		m, _ = m.Update(msg)
	}
	view := m.View()

	// All 7 day labels + the legend label must appear in the rendered view.
	for _, needle := range []string{"Lun ", "Mar ", "Mer ", "Jeu ", "Ven ", "Sam ", "Dim ", "émissions"} {
		if !strings.Contains(view, needle) {
			t.Errorf("dashboard heatmap missing %q — box height clipping the row?", needle)
		}
	}
}

