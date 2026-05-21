package widgets

import (
	"strings"

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

var (
	hintKey  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff36d4")).Bold(true).Padding(0, 1)
	hintText = lipgloss.NewStyle().Foreground(lipgloss.Color("#7a7ab8"))
	hintDim  = lipgloss.NewStyle().Foreground(lipgloss.Color("#4a4a78"))
)

func (s *StatusBar) View() string {
	parts := make([]string, len(s.Hints))
	for i, h := range s.Hints {
		parts[i] = hintKey.Render(h.Key) + hintText.Render(h.Desc)
	}
	bar := strings.Join(parts, hintDim.Render("  "))
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#0a0a18")).
		Width(s.bounds.W).
		Render(bar)
}
