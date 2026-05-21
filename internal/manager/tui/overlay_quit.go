package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// quitOverlay asks the operator to confirm exit when servers are running.
type quitOverlay struct {
	serversRunning bool
}

func newQuitOverlay(serversRunning bool) *quitOverlay {
	return &quitOverlay{serversRunning: serversRunning}
}

var (
	quitConfirmKey = key.NewBinding(key.WithKeys("y", "Y"), key.WithHelp("y", "yes, quit"))
	quitCancelKey  = key.NewBinding(key.WithKeys("n", "N", "esc", "q"), key.WithHelp("n/esc", "cancel"))
)

func (o *quitOverlay) Init() tea.Cmd { return nil }

func (o *quitOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return o, nil
	}
	switch {
	case key.Matches(km, quitConfirmKey):
		return o, func() tea.Msg { return OverlayDoneMsg{Result: true} }
	case key.Matches(km, quitCancelKey):
		return o, func() tea.Msg { return OverlayDoneMsg{Result: false} }
	}
	return o, nil
}

func (o *quitOverlay) View() string {
	var body string
	if o.serversRunning {
		body = GlowYellow.Render("⚠  Servers are running.") + "\n\n" +
			Base.Render("Stop servers and quit?")
	} else {
		body = Base.Render("Quit license-manager?")
	}
	body += "\n\n" +
		HintKey.Render("y") + HintText.Render(" confirm   ") +
		HintKey.Render("n/esc") + HintText.Render(" cancel")

	style := ModalDanger
	if !o.serversRunning {
		style = Modal
	}
	return lipgloss.Place(
		40, 10,
		lipgloss.Center, lipgloss.Center,
		style.Render(body),
	)
}
