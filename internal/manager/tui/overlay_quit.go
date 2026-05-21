package tui

import (
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

func (o *quitOverlay) Init() tea.Cmd { return nil }

func (o *quitOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return o, nil
	}
	switch km.String() {
	case "y", "Y", "enter":
		return o, func() tea.Msg { return OverlayDoneMsg{Result: true} }
	case "n", "N", "esc", "q":
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

	const innerW = 52
	confirmLabel := "Quitter"
	confirmKind := btnPrimary
	if o.serversRunning {
		confirmLabel = "Arrêter & quitter"
		confirmKind = btnDanger
	}
	footer := renderButtons(innerW,
		button{label: "Annuler", hotkey: "esc", kind: btnNeutral},
		button{label: confirmLabel, hotkey: "↵", kind: confirmKind, focused: true},
	)
	bodyLines = append(bodyLines, "", footer)

	content := title + "\n\n" + lipgloss.JoinVertical(lipgloss.Left, bodyLines...)

	style := Modal
	if o.serversRunning {
		style = ModalDanger
	}
	return lipgloss.Place(58, 12, lipgloss.Center, lipgloss.Center, style.Render(content))
}
