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
	Color lipgloss.Color
	// NoLeftBorder suppresses the left border so adjacent tiles share a single wall.
	NoLeftBorder bool
	OnPress      func() tea.Cmd
	bounds       core.Rect
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
	header, hotkey, footer lipgloss.Style
}

var tileStyleCache = sync.OnceValue(func() tileStyleSet {
	return tileStyleSet{
		// No Padding on header — the outer border already provides 1-char left/right
		// margin via the NormalBorder box. Adding Padding here would double-consume
		// width and cause label/hotkey to wrap on narrow tiles.
		header: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(core.Colors.Border).
			Foreground(core.Colors.FgDim),
		hotkey: lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true),
		footer: lipgloss.NewStyle().Foreground(core.Colors.FgDim),
	}
})

// tileOuterBorder returns the outer border style for a tile.
// When noLeftBorder is true the left wall is omitted so adjacent tiles share a
// single │ rather than rendering ││ between them.
// lipgloss.BorderLeft(false) suppresses the left edge at render time without
// affecting the border size calculation (corners remain in the struct but the
// renderer skips the left column), so innerW must account for this.
func tileOuterBorder(noLeftBorder bool) lipgloss.Style {
	st := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(core.Colors.Border)
	if noLeftBorder {
		st = st.BorderLeft(false)
	}
	return st
}

func (t *Tile) View() string {
	st := tileStyleCache()
	w := t.bounds.W
	if w < 4 {
		w = 4
	}

	// Outer border: left(1) + right(1) = 2 chars consumed horizontally.
	// When NoLeftBorder, lipgloss suppresses the left column but still reserves
	// it in Width(); subtract only 1 to use the recovered char for content.
	innerW := w - 2
	if t.NoLeftBorder {
		innerW = w - 1
	}
	if innerW < 1 {
		innerW = 1
	}

	// Header row: label flush-left, [hotkey] flush-right.
	var headerRight string
	if t.Hotkey != "" {
		headerRight = st.hotkey.Render("[" + t.Hotkey + "]")
	}
	labelW := lipgloss.Width(t.Label)
	hotkeyW := lipgloss.Width(headerRight)
	gap := innerW - labelW - hotkeyW
	if gap < 0 {
		gap = 0
	}
	headerContent := t.Label + strings.Repeat(" ", gap) + headerRight
	headerRow := st.header.Width(innerW).Render(headerContent)

	// Value row: Color applied per-call (not in shared cache).
	valueRow := lipgloss.NewStyle().Foreground(t.Color).Bold(true).
		Width(innerW).Render(fmt.Sprintf("%d", t.Value))

	rows := []string{headerRow, valueRow}
	if t.Footer != "" {
		// Truncate footer to innerW so it never wraps — compact single-line layout.
		footer := t.Footer
		if lipgloss.Width(footer) > innerW {
			// Trim to innerW-1 and append ellipsis.
			runes := []rune(footer)
			for lipgloss.Width(string(runes)+"…") > innerW && len(runes) > 0 {
				runes = runes[:len(runes)-1]
			}
			footer = string(runes) + "…"
		}
		rows = append(rows, st.footer.Width(innerW).Render(footer))
	}

	// Do NOT pass Width() to the border style — rows are already padded to
	// innerW, and adding Width(w) would set the content area to w then add
	// borders on top, making total visual width = w+2 instead of w.
	borderStyle := tileOuterBorder(t.NoLeftBorder)
	return borderStyle.Render(strings.Join(rows, "\n"))
}

// OnClick implements core.Clickable.
func (t *Tile) OnClick(_, _ int, _ tea.MouseButton) tea.Cmd {
	if t.OnPress != nil {
		return t.OnPress()
	}
	return nil
}
