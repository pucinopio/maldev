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
	for i, tab := range tb.Tabs {
		label := tab.Label
		if i < 9 {
			label = dimStyle.Render(string(rune('1'+i))) + " " + label
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

// OnClick implements core.Clickable. x is relative to bounds.X.
func (tb *TabBar) OnClick(x, _ int, _ tea.MouseButton) tea.Cmd {
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
