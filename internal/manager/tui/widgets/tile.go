package widgets

import (
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// Tile is a counter card shown on the dashboard.
//
// Layout (matches primitives.jsx Tile):
//
//	┌──────────────────────────────┐
//	│ Label               [hotkey] │  ← dim header row, bordered bottom
//	│ VALUE                        │  ← big bold coloured number (min 5ch)
//	│ subtitle text                │  ← dim footer (optional)
//	└──────────────────────────────┘
//
// Clicking anywhere fires OnPress. Implements core.Clickable.
type Tile struct {
	// Label is the human-readable name shown in the header (e.g. "Actives").
	Label string
	// Hotkey is the single-char shortcut shown as [k] in the header, or "" to omit.
	Hotkey string
	// Value is the integer counter rendered as the primary content.
	Value int
	// Footer is the dim subtitle line below the value, or "" to omit.
	Footer string
	// Color is the foreground color for Value.
	Color   lipgloss.Color
	OnPress func() tea.Cmd
	bounds  core.Rect
}

// NewTile constructs a Tile.
//
// title may embed "[k]" (old callers); the new signature separates label and
// hotkey. For callers that pass a combined title such as "Actives [a]" the
// widget splits on " [" automatically so old call-sites remain correct.
func NewTile(title string, value int, subtitle string, color lipgloss.Color, onPress func() tea.Cmd) *Tile {
	label, hotkey := splitTileTitle(title)
	return &Tile{
		Label:   label,
		Hotkey:  hotkey,
		Value:   value,
		Footer:  subtitle,
		Color:   color,
		OnPress: onPress,
	}
}

// splitTileTitle splits a combined title like "Actives [a]" into ("Actives", "a").
// If no "[k]" suffix is present the whole string is returned as the label.
func splitTileTitle(title string) (label, key string) {
	if i := strings.Index(title, " ["); i >= 0 && strings.HasSuffix(title, "]") {
		return title[:i], title[i+2 : len(title)-1]
	}
	return title, ""
}

func (t *Tile) Layout(bounds core.Rect) { t.bounds = bounds }
func (t *Tile) Bounds() core.Rect      { return t.bounds }

func (t *Tile) Update(_ tea.Msg) (core.Widget, tea.Cmd) { return t, nil }

type tileStyleSet struct {
	header, hotkey, footer, border lipgloss.Style
}

var tileStyleCache = sync.OnceValue(func() tileStyleSet {
	return tileStyleSet{
		header: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(core.Colors.Border).
			Foreground(core.Colors.FgDim).
			Padding(0, 1),
		hotkey: lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true),
		footer: lipgloss.NewStyle().Foreground(core.Colors.FgDim),
		border: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(core.Colors.Border),
	}
})

func (t *Tile) View() string {
	st := tileStyleCache()
	w := t.bounds.W
	if w < 4 {
		w = 4
	}

	// border (1 each side) + padding (1 each side) = 4 cells consumed by the outer box.
	innerW := w - 4
	if innerW < 1 {
		innerW = 1
	}

	// Header row: label flush-left, [hotkey] flush-right.
	var headerRight string
	if t.Hotkey != "" {
		headerRight = st.hotkey.Render("[" + t.Hotkey + "]")
	}
	headerContent := t.Label +
		strings.Repeat(" ", max(0, innerW-lipgloss.Width(t.Label)-lipgloss.Width(headerRight))) +
		headerRight
	headerRow := st.header.Width(innerW).Render(headerContent)

	// Value row: instance Color applied per-call (Color is not in the shared cache).
	valueRow := lipgloss.NewStyle().Foreground(t.Color).Bold(true).
		Width(innerW).Padding(0, 1).Render(fmt.Sprintf("%d", t.Value))

	rows := []string{headerRow, valueRow}
	if t.Footer != "" {
		rows = append(rows, st.footer.Width(innerW).Padding(0, 1).Render(t.Footer))
	}

	return st.border.Width(w).Render(strings.Join(rows, "\n"))
}

// OnClick implements core.Clickable.
func (t *Tile) OnClick(_, _ int, _ tea.MouseButton) tea.Cmd {
	if t.OnPress != nil {
		return t.OnPress()
	}
	return nil
}
