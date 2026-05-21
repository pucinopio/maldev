package widgets

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui"
)

// Tile is a counter card shown on the dashboard. Clicking it fires OnPress.
// It implements tui.Clickable.
type Tile struct {
	Title    string
	Value    int
	Subtitle string
	Color    lipgloss.Color
	OnPress  func() tea.Cmd
	bounds   tui.Rect
}

// NewTile constructs a Tile.
func NewTile(title string, value int, subtitle string, color lipgloss.Color, onPress func() tea.Cmd) *Tile {
	return &Tile{
		Title:    title,
		Value:    value,
		Subtitle: subtitle,
		Color:    color,
		OnPress:  onPress,
	}
}

func (t *Tile) Layout(bounds tui.Rect) { t.bounds = bounds }
func (t *Tile) Bounds() tui.Rect      { return t.bounds }

func (t *Tile) Update(_ tea.Msg) (tui.Widget, tea.Cmd) { return t, nil }

func (t *Tile) View() string {
	valStyle := lipgloss.NewStyle().Foreground(t.Color).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7a7ab8"))

	inner := lipgloss.JoinVertical(lipgloss.Center,
		valStyle.Render(fmt.Sprintf("%d", t.Value)),
		dimStyle.Render(t.Title),
	)
	if t.Subtitle != "" {
		inner = lipgloss.JoinVertical(lipgloss.Center, inner, dimStyle.Render(t.Subtitle))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#2a2a52")).
		Padding(0, 1).
		Width(t.bounds.W).
		Align(lipgloss.Center).
		Render(inner)
}

// OnClick implements tui.Clickable.
func (t *Tile) OnClick(_, _ int, _ tea.MouseButton) tea.Cmd {
	if t.OnPress != nil {
		return t.OnPress()
	}
	return nil
}
