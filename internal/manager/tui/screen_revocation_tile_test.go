package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestRevocInfoTile_HeightStaysEqual guards the visual bug where a long
// footer wrapped onto a second line, making adjacent tiles uneven in
// height — lipgloss.JoinHorizontal then renders the wrap as visible
// misalignment of the bottom borders.
func TestRevocInfoTile_HeightStaysEqual(t *testing.T) {
	const w = 28
	short := revocInfoTile("Pushed via :8443", "oui", "—", lipgloss.Color("#fff"), w)
	long := revocInfoTile("Pushed via :8443", "oui",
		"un footer beaucoup plus long que la largeur du tile pour forcer la troncature",
		lipgloss.Color("#fff"), w)

	if hs, hl := lipgloss.Height(short), lipgloss.Height(long); hs != hl {
		t.Errorf("tile height drift: short=%d, long=%d — footer wrapping breaks JoinHorizontal alignment", hs, hl)
	}
	if !strings.Contains(long, "…") {
		t.Errorf("long footer should be ellipsis-truncated, got:\n%s", long)
	}
	// Widths must match each other — JoinHorizontal alignment needs it.
	// (Actual render width is w+border, but stability across short/long is
	// what protects the row layout.)
	if ws, wl := lipgloss.Width(short), lipgloss.Width(long); ws != wl {
		t.Errorf("tile width drift: short=%d long=%d", ws, wl)
	}
}
