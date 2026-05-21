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
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "y", "Y", "enter":
			return o, func() tea.Msg { return OverlayDoneMsg{Result: true} }
		case "n", "N", "esc", "q":
			return o, func() tea.Msg { return OverlayDoneMsg{Result: false} }
		}
	case tea.MouseMsg:
		if m.Button != tea.MouseButtonLeft || m.Action != tea.MouseActionPress {
			return o, nil
		}
		// 58x12 modal; body has 1 (title) + 2 (blank+body) + 1 (blank) + 1 (footer)
		// = 5 content lines + 4 (border+padding) = 9 rows; centered → starts at Y=1.
		// Footer at content row 4 → overlay Y = 1 + 1 (border) + 1 (pad) + 4 = 7.
		if m.Y != 7 {
			return o, nil
		}
		// Inner X spans ~3..55. Cancel left half, Quit right half.
		if m.X < 29 {
			return o, func() tea.Msg { return OverlayDoneMsg{Result: false} }
		}
		return o, func() tea.Msg { return OverlayDoneMsg{Result: true} }
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
