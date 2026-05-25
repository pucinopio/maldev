package widgets

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// TabItem describes one tab in a TabBar.
type TabItem struct {
	ID    string
	Label string
}

// SwitchViewMsg is dispatched when the user clicks a tab.
type SwitchViewMsg struct{ ID string }

// TabBar is a horizontal strip of tabs. Clicking a tab fires SwitchViewMsg.
// It implements core.Clickable.
type TabBar struct {
	Tabs   []TabItem
	Active string
	bounds core.Rect

	// tabWidths[i] is the rendered width of tab i, set during View().
	tabWidths []int
}

// NewTabBar constructs a TabBar.
func NewTabBar(tabs []TabItem, active string) *TabBar {
	return &TabBar{Tabs: tabs, Active: active}
}

func (tb *TabBar) Layout(bounds core.Rect) { tb.bounds = bounds }
func (tb *TabBar) Bounds() core.Rect      { return tb.bounds }

func (tb *TabBar) Update(_ tea.Msg) (core.Widget, tea.Cmd) { return tb, nil }

type tabStyleSet struct{ active, inactive, dim lipgloss.Style }

// tabStyleCache initialises styles once after tui/theme.go's init() has
// populated core.Colors. lipgloss.Style is a value type — safe to cache.
var tabStyleCache = sync.OnceValue(func() tabStyleSet {
	return tabStyleSet{
		active: lipgloss.NewStyle().
			Foreground(core.Colors.Fg).Bold(true).Padding(0, 2).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(core.Colors.Magenta),
		inactive: lipgloss.NewStyle().Foreground(core.Colors.FgDim).Padding(0, 2),
		dim:      lipgloss.NewStyle().Foreground(core.Colors.FgDim),
	}
})

func (tb *TabBar) View() string {
	s := tabStyleCache()
	activeStyle, inactiveStyle, dimStyle := s.active, s.inactive, s.dim
	tb.tabWidths = make([]int, len(tb.Tabs))
	parts := make([]string, len(tb.Tabs))

	// Render twice: first the full-label version (the common wide case).
	// If the total exceeds bounds.W, fall back to digit-only labels so
	// the strip stays on one row even at 80-cell terminals. Without this
	// trim, lipgloss soft-wraps the strip and breaks the chrome.
	compact := tb.shouldUseCompact()
	for i, tab := range tb.Tabs {
		var label string
		digit := tabDigit(i)
		switch {
		case compact:
			// Compact: "N" only (dim if inactive, bold via activeStyle).
			label = dimStyle.Render(digit)
		case i < 9:
			label = dimStyle.Render(digit) + " " + tab.Label
		case i == 9:
			label = dimStyle.Render(digit) + " " + tab.Label
		default:
			label = tab.Label
		}
		var rendered string
		if tab.ID == tb.Active {
			rendered = activeStyle.Render(label)
		} else {
			rendered = inactiveStyle.Render(label)
		}
		parts[i] = rendered
		tb.tabWidths[i] = lipgloss.Width(rendered)
	}
	strip := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	return lipgloss.NewStyle().
		Background(core.Colors.Bg1).
		Width(tb.bounds.W).
		Render(strip)
}

// tabDigit returns the single-key shortcut for tab i: "1".."9", then "0"
// for the 10th tab. Tabs beyond 10 have no shortcut.
func tabDigit(i int) string {
	switch {
	case i < 9:
		return string(rune('1' + i))
	case i == 9:
		return "0"
	}
	return ""
}

// shouldUseCompact measures the fully-labelled strip width and reports
// whether it would overflow the bounds. Cheap: each tab is rendered to a
// small string and summed.
func (tb *TabBar) shouldUseCompact() bool {
	if tb.bounds.W <= 0 {
		return false
	}
	total := 0
	s := tabStyleCache()
	for i, tab := range tb.Tabs {
		var label string
		if i < 10 {
			label = s.dim.Render(tabDigit(i)) + " " + tab.Label
		} else {
			label = tab.Label
		}
		// Active style adds underline (no extra width) so width is the
		// same either way; use inactive as a representative measurer.
		total += lipgloss.Width(s.inactive.Render(label))
	}
	return total > tb.bounds.W
}

// OnClick implements core.Clickable. x is relative to bounds.X.
// Forces a View() pass when tabWidths is empty so callers (e.g. the root
// mouse dispatcher) can hit-test without first having to render.
func (tb *TabBar) OnClick(x, _ int, _ tea.MouseButton) tea.Cmd {
	if len(tb.tabWidths) == 0 {
		_ = tb.View()
	}
	cursor := 0
	for i, w := range tb.tabWidths {
		if x >= cursor && x < cursor+w {
			id := tb.Tabs[i].ID
			return func() tea.Msg { return SwitchViewMsg{ID: id} }
		}
		cursor += w
	}
	return nil
}
