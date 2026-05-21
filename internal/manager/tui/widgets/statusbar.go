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
//
// Layout (matches chrome.jsx StatusBar):
//
//	[k] hint  [k2] hint2  …          ~ tour
//	^── hints left ───────^          ^─ tour pill right
//
// The "~ tour" pill is always rendered on the far right.
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

type statusStyleSet struct {
	key, text, sep, tour lipgloss.Style
}

var statusStyleCache = sync.OnceValue(func() statusStyleSet {
	return statusStyleSet{
		key:  lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true),
		text: lipgloss.NewStyle().Foreground(core.Colors.FgDim),
		sep:  lipgloss.NewStyle().Foreground(core.Colors.FgMute),
		// "~ tour" tag: flat single-line style so the status bar stays exactly 1 row.
		// A bordered pill is 3 rows tall and would overflow the 1-row chrome slot.
		tour: lipgloss.NewStyle().
			Foreground(core.Colors.Yellow).
			Bold(true).
			Padding(0, 1).
			Background(lipgloss.Color("#2a2200")),
	}
})

func (s *StatusBar) View() string {
	st := statusStyleCache()

	parts := make([]string, len(s.Hints))
	for i, h := range s.Hints {
		parts[i] = st.key.Render(h.Key) + " " + st.text.Render(h.Desc)
	}
	hintsStr := strings.Join(parts, st.sep.Render("  "))

	tourPill := st.tour.Render("~ tour")
	tourW := lipgloss.Width(tourPill)
	hintsW := lipgloss.Width(hintsStr)

	gap := s.bounds.W - hintsW - tourW
	if gap < 1 {
		gap = 1
	}

	bar := hintsStr + strings.Repeat(" ", gap) + tourPill
	return lipgloss.NewStyle().
		Background(core.Colors.Bg1).
		Width(s.bounds.W).
		Render(bar)
}
