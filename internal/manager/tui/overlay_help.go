package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpOverlay renders the keybindings reference modal — the "?" key target.
// Layout matches design/prototype/overlays.jsx HelpModal: two-column grid with
// sections grouped by context (universal, lists, forms, servers, detail).
type helpOverlay struct{}

// NewHelpOverlay is exported for use by cmd/tui-snap.
func NewHelpOverlay() Overlay { return &helpOverlay{} }

func (o *helpOverlay) Init() tea.Cmd { return nil }

func (o *helpOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "esc", "enter", "q", "?":
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
	case tea.MouseMsg:
		// Click anywhere dismisses the help overlay — it's a read-only modal.
		if m.Button == tea.MouseButtonLeft && m.Action == tea.MouseActionPress {
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
	}
	return o, nil
}

type helpRow struct{ key, label string }

func (o *helpOverlay) View() string {
	left := []any{
		helpSection("Universel"),
		helpRow{"1-9", "changer d'onglet"},
		helpRow{"Tab / ⇧Tab", "onglet suivant / précédent"},
		helpRow{"esc", "retour / fermer overlay"},
		helpRow{"?", "cette aide"},
		helpRow{"q", "quitter (confirme si serveur ON)"},
		helpRow{"/", "rechercher dans la vue"},
		helpRow{"r", "rafraîchir"},
		"",
		helpSection("Listes"),
		helpRow{"↑ ↓", "naviguer"},
		helpRow{"d", "détail (split-pane)"},
		helpRow{"n", "nouveau"},
		helpRow{"e", "éditer / re-émettre"},
		helpRow{"x", "supprimer / révoquer"},
		helpRow{"f", "cycler filtre status"},
	}
	right := []any{
		helpSection("Formulaires & wizard"),
		helpRow{"Tab", "champ suivant"},
		helpRow{"⇧Tab", "champ précédent"},
		helpRow{"↵", "valider"},
		helpRow{"ctrl+c", "annuler opération"},
		helpRow{"1-8", "aller à étape (wizard)"},
		"",
		helpSection("Serveurs / Probe"),
		helpRow{"s", "start / stop serveur"},
		helpRow{"g", "régénérer admin token"},
		helpRow{"R H P", "sous-onglets revoc/hb/probe"},
		helpRow{"T H L", "probe: Tokens/History/Live"},
		"",
		helpSection("Détail licence"),
		helpRow{"I B P A C", "Identité · Bindings · PEM · Audit · Chaîne"},
	}

	const colW = 44
	leftCol := renderHelpColumn(left, colW)
	rightCol := renderHelpColumn(right, colW)
	grid := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "  ", rightCol)

	title := GlowMagent.Render("? Aide — touches")
	footer := HintKey.Render("↵/esc") + HintText.Render(" fermer")
	footerLine := lipgloss.NewStyle().Width(lipgloss.Width(grid)).Align(lipgloss.Right).Render(footer)

	content := title + "\n\n" + grid + "\n\n" + footerLine
	return Modal.Render(content)
}

func renderHelpColumn(rows []any, width int) string {
	keyStyle := GlowMagent.Width(14)
	labelStyle := Dim
	var lines []string
	for _, r := range rows {
		switch v := r.(type) {
		case helpRow:
			lines = append(lines, keyStyle.Render(v.key)+labelStyle.Render(v.label))
		case string:
			lines = append(lines, v)
		}
	}
	return lipgloss.NewStyle().Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func helpSection(title string) string {
	return GlowCyan.Render(title)
}
