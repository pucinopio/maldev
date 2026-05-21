package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmResultMsg is emitted by confirmOverlay when the user makes a choice.
type ConfirmResultMsg struct {
	ID      string // caller-supplied identifier to disambiguate nested confirms
	Confirm bool
}

// confirmOverlay is a generic yes/no modal.
type confirmOverlay struct {
	id           string
	title        string
	body         string
	confirmLabel string
	cancelLabel  string
	danger       bool
}

func newConfirmOverlay(id, title, body, confirmLabel, cancelLabel string, danger bool) *confirmOverlay {
	return &confirmOverlay{
		id:           id,
		title:        title,
		body:         body,
		confirmLabel: confirmLabel,
		cancelLabel:  cancelLabel,
		danger:       danger,
	}
}

func (o *confirmOverlay) Init() tea.Cmd { return nil }

func (o *confirmOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return o, nil
	}
	switch km.String() {
	case "y", "Y", "enter":
		id := o.id
		return o, func() tea.Msg {
			return OverlayDoneMsg{Result: ConfirmResultMsg{ID: id, Confirm: true}}
		}
	case "n", "N", "esc", "q":
		id := o.id
		return o, func() tea.Msg {
			return OverlayDoneMsg{Result: ConfirmResultMsg{ID: id, Confirm: false}}
		}
	}
	return o, nil
}

func (o *confirmOverlay) View() string {
	content := GlowMagent.Render(o.title) + "\n\n" +
		Base.Render(o.body) + "\n\n" +
		HintKey.Render("y/enter") + HintText.Render(" "+o.confirmLabel+"   ") +
		HintKey.Render("n/esc") + HintText.Render(" "+o.cancelLabel)

	style := Modal
	if o.danger {
		style = ModalDanger
	}
	return lipgloss.Place(50, 12, lipgloss.Center, lipgloss.Center, style.Render(content))
}
