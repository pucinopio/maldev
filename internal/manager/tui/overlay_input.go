package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputResultMsg is emitted by inputOverlay when the user submits a value.
type InputResultMsg struct {
	ID    string
	Value string
}

// inputOverlay is a generic single-line text-input modal.
type inputOverlay struct {
	id          string
	title       string
	placeholder string
	input       textinput.Model
}

// NewInputOverlay is exported for use by cmd/tui-snap.
func NewInputOverlay(id, title, placeholder string, charLimit int) Overlay {
	return newInputOverlay(id, title, placeholder, charLimit)
}

func newInputOverlay(id, title, placeholder string, charLimit int) *inputOverlay {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = charLimit
	ti.Width = 44
	ti.Focus()
	return &inputOverlay{id: id, title: title, placeholder: placeholder, input: ti}
}

func (o *inputOverlay) Init() tea.Cmd { return textinput.Blink }

func (o *inputOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			val := o.input.Value()
			if val == "" {
				return o, nil
			}
			id, v := o.id, val
			return o, func() tea.Msg {
				return OverlayDoneMsg{Result: InputResultMsg{ID: id, Value: v}}
			}
		case "esc":
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
	}
	var cmd tea.Cmd
	o.input, cmd = o.input.Update(msg)
	return o, cmd
}

func (o *inputOverlay) View() string {
	content := GlowMagent.Render(o.title) + "\n\n" +
		o.input.View() + "\n\n" +
		HintKey.Render("enter") + HintText.Render(" confirm   ") +
		HintKey.Render("esc") + HintText.Render(" cancel")
	return lipgloss.Place(54, 12, lipgloss.Center, lipgloss.Center, Modal.Render(content))
}
