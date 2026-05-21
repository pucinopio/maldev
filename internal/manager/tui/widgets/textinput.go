package widgets

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// WrappedTextInput wraps bubbles/textinput as a Focusable + Clickable widget.
type WrappedTextInput struct {
	inner  textinput.Model
	bounds core.Rect
}

// NewWrappedTextInput constructs a WrappedTextInput from an already-configured model.
func NewWrappedTextInput(ti textinput.Model) *WrappedTextInput {
	return &WrappedTextInput{inner: ti}
}

func (w *WrappedTextInput) Layout(bounds core.Rect) {
	w.bounds = bounds
	w.inner.Width = bounds.W - 4 // leave room for prompt + border
}

func (w *WrappedTextInput) Bounds() core.Rect { return w.bounds }

func (w *WrappedTextInput) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	updated, cmd := w.inner.Update(msg)
	w.inner = updated
	return w, cmd
}

func (w *WrappedTextInput) View() string { return w.inner.View() }

// Focus implements core.Focusable.
func (w *WrappedTextInput) Focus()        { w.inner.Focus() } //nolint:errcheck — Focus returns tea.Cmd in newer bubbles; we discard it intentionally here
func (w *WrappedTextInput) Blur()         { w.inner.Blur() }
func (w *WrappedTextInput) Focused() bool { return w.inner.Focused() }

// OnClick implements core.Clickable — clicking focuses the input.
func (w *WrappedTextInput) OnClick(_, _ int, _ tea.MouseButton) tea.Cmd {
	return w.inner.Focus()
}

// Value returns the current text value.
func (w *WrappedTextInput) Value() string { return w.inner.Value() }
