package widgets

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui"
)

// Spacer is an empty widget used for layout gaps.
type Spacer struct{ bounds tui.Rect }

// NewSpacer constructs a Spacer.
func NewSpacer() *Spacer { return &Spacer{} }

func (s *Spacer) Layout(bounds tui.Rect) { s.bounds = bounds }
func (s *Spacer) Bounds() tui.Rect      { return s.bounds }

func (s *Spacer) Update(_ tea.Msg) (tui.Widget, tea.Cmd) { return s, nil }

func (s *Spacer) View() string {
	return lipgloss.NewStyle().Width(s.bounds.W).Height(s.bounds.H).Render("")
}
