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

// NewQuitOverlay is exported for use by cmd/tui-snap.
func NewQuitOverlay(serversRunning bool) Overlay { return newQuitOverlay(serversRunning) }

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
	var title string
	if o.serversRunning {
		title = GlowRed.Render("Quitter license-manager ?")
	} else {
		title = GlowMagent.Render("Quitter license-manager ?")
	}

	var bodyLines []string
	if o.serversRunning {
		bodyLines = append(bodyLines,
			GlowYellow.Render("⚠  Serveur(s) HTTP actif(s)"),
			"",
			Dim.Render("Quitter va arrêter les serveurs et fermer la base proprement."),
		)
	} else {
		bodyLines = append(bodyLines,
			Dim.Render("Aucun serveur HTTP actif."),
		)
	}

	bodyLines = append(bodyLines,
		"",
		HintKey.Render("y/↵") + HintText.Render(" Arrêter & quitter   ") +
			HintKey.Render("n/esc") + HintText.Render(" Annuler"),
	)

	content := title + "\n\n" + lipgloss.JoinVertical(lipgloss.Left, bodyLines...)

	style := Modal
	if o.serversRunning {
		style = ModalDanger
	}
	return lipgloss.Place(58, 12, lipgloss.Center, lipgloss.Center, style.Render(content))
}
