package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// errorOverlay shows a single error message. Esc or enter dismisses it.
type errorOverlay struct {
	title string
	msg   string
}

func newErrorOverlay(title, msg string) *errorOverlay {
	return &errorOverlay{title: title, msg: msg}
}

func (o *errorOverlay) Init() tea.Cmd { return nil }

func (o *errorOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return o, nil
	}
	switch km.String() {
	case "esc", "enter", "q":
		return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
	}
	return o, nil
}

func (o *errorOverlay) View() string {
	content := GlowRed.Render(o.title) + "\n\n" +
		Base.Render(o.msg) + "\n\n" +
		HintKey.Render("esc") + HintText.Render(" dismiss")
	return lipgloss.Place(54, 10, lipgloss.Center, lipgloss.Center, ModalDanger.Render(content))
}
