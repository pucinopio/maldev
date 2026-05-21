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
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
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
	case tea.MouseMsg:
		if m.Button != tea.MouseButtonLeft || m.Action != tea.MouseActionPress {
			return o, nil
		}
		// Same geometry as confirm overlay: 54x12 modal, footer at Y=7.
		if m.Y != 7 {
			return o, nil
		}
		if m.X < 27 {
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
		val := o.input.Value()
		if val == "" {
			return o, nil
		}
		id, v := o.id, val
		return o, func() tea.Msg {
			return OverlayDoneMsg{Result: InputResultMsg{ID: id, Value: v}}
		}
	}
	var cmd tea.Cmd
	o.input, cmd = o.input.Update(msg)
	return o, cmd
}

func (o *inputOverlay) View() string {
	const innerW = 48
	footer := renderButtons(innerW,
		button{label: "Annuler", hotkey: "esc", kind: btnNeutral},
		button{label: "Confirmer", hotkey: "↵", kind: btnPrimary, focused: true},
	)
	content := GlowMagent.Render(o.title) + "\n\n" +
		o.input.View() + "\n\n" + footer
	return lipgloss.Place(54, 12, lipgloss.Center, lipgloss.Center, Modal.Render(content))
}
