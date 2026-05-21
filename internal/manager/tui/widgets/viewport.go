package widgets

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// WrappedViewport wraps bubbles/viewport as a core.Widget.
// Mouse wheel events scroll the content.
type WrappedViewport struct {
	inner  viewport.Model
	bounds core.Rect
}

// NewWrappedViewport constructs a WrappedViewport with the given content.
func NewWrappedViewport(content string) *WrappedViewport {
	vp := viewport.New(0, 0)
	vp.SetContent(content)
	return &WrappedViewport{inner: vp}
}

func (wv *WrappedViewport) Layout(bounds core.Rect) {
	wv.bounds = bounds
	wv.inner.Width = bounds.W
	wv.inner.Height = bounds.H
}

func (wv *WrappedViewport) Bounds() core.Rect { return wv.bounds }

func (wv *WrappedViewport) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	if mm, ok := msg.(tea.MouseMsg); ok {
		if mm.Action == tea.MouseActionPress {
			switch mm.Button {
			case tea.MouseButtonWheelUp:
				wv.inner.LineUp(3)
			case tea.MouseButtonWheelDown:
				wv.inner.LineDown(3)
			}
		}
	}
	updated, cmd := wv.inner.Update(msg)
	wv.inner = updated
	return wv, cmd
}

func (wv *WrappedViewport) View() string { return wv.inner.View() }

// SetContent replaces the viewport content.
func (wv *WrappedViewport) SetContent(s string) { wv.inner.SetContent(s) }
