package widgets

import (
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// KeyHint is a key + description pair shown in the status bar.
type KeyHint struct {
	Key  string
	Desc string
}

// StatusBar renders a row of keyboard hints at the bottom of a screen.
type StatusBar struct {
	Hints  []KeyHint
	bounds core.Rect
}

// NewStatusBar constructs a StatusBar.
func NewStatusBar(hints ...KeyHint) *StatusBar {
	return &StatusBar{Hints: hints}
}

func (s *StatusBar) Layout(bounds core.Rect) { s.bounds = bounds }
func (s *StatusBar) Bounds() core.Rect      { return s.bounds }

func (s *StatusBar) Update(_ tea.Msg) (core.Widget, tea.Cmd) { return s, nil }

type statusStyleSet struct{ key, text, sep lipgloss.Style }

var statusStyleCache = sync.OnceValue(func() statusStyleSet {
	return statusStyleSet{
		key:  lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true).Padding(0, 1),
		text: lipgloss.NewStyle().Foreground(core.Colors.FgDim),
		sep:  lipgloss.NewStyle().Foreground(core.Colors.FgMute),
	}
})

func (s *StatusBar) View() string {
	st := statusStyleCache()
	parts := make([]string, len(s.Hints))
	for i, h := range s.Hints {
		parts[i] = st.key.Render(h.Key) + st.text.Render(h.Desc)
	}
	bar := strings.Join(parts, st.sep.Render("  "))
	return lipgloss.NewStyle().
		Background(core.Colors.Bg1).
		Width(s.bounds.W).
		Render(bar)
}
