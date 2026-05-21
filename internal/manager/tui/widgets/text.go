// Package widgets provides concrete leaf widgets that implement tui.Widget.
// Each widget is non-interactive (Text, Spacer) or implements tui.Clickable /
// tui.Focusable as appropriate.
package widgets

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// Text is a non-interactive leaf widget that renders a styled string.
type Text struct {
	content string
	style   lipgloss.Style
	bounds  core.Rect
}

// NewText constructs a Text widget.
func NewText(content string, style lipgloss.Style) *Text {
	return &Text{content: content, style: style}
}

func (t *Text) Layout(bounds core.Rect) { t.bounds = bounds }
func (t *Text) Bounds() core.Rect      { return t.bounds }

func (t *Text) Update(_ tea.Msg) (core.Widget, tea.Cmd) { return t, nil }

func (t *Text) View() string {
	return t.style.Width(t.bounds.W).Height(t.bounds.H).Render(t.content)
}

// SetContent replaces the text content.
func (t *Text) SetContent(s string) { t.content = s }
