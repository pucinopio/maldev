package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

// RevokeConfirmedMsg is emitted when the operator confirms a revocation.
type RevokeConfirmedMsg struct {
	LicenseID uuid.UUID
	Reason    string
}

// revokeOverlay is a modal that collects a revocation reason.
type revokeOverlay struct {
	licenseID uuid.UUID
	subject   string
	input     textinput.Model
}

func newRevokeOverlay(licenseID uuid.UUID, subject string) *revokeOverlay {
	ti := textinput.New()
	ti.Placeholder = "reason for revocation…"
	ti.CharLimit = 200
	ti.Width = 44
	ti.Focus()
	return &revokeOverlay{licenseID: licenseID, subject: subject, input: ti}
}

func (o *revokeOverlay) Init() tea.Cmd { return textinput.Blink }

func (o *revokeOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			reason := o.input.Value()
			if reason == "" {
				return o, nil
			}
			id := o.licenseID
			return o, func() tea.Msg {
				return OverlayDoneMsg{Result: RevokeConfirmedMsg{LicenseID: id, Reason: reason}}
			}
		case "esc":
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
	}
	var cmd tea.Cmd
	o.input, cmd = o.input.Update(msg)
	return o, cmd
}

func (o *revokeOverlay) View() string {
	suggestions := []string{
		"key_compromised", "offboarding", "leak", "decommissioned", "abuse",
	}
	var chips []string
	for _, s := range suggestions {
		chips = append(chips, lipgloss.NewStyle().
			Foreground(Palette.FgMute).
			Border(lipgloss.NormalBorder()).BorderForeground(Palette.Border).
			Padding(0, 0).
			Render(s))
	}
	chipLine := lipgloss.JoinHorizontal(lipgloss.Top, chips...)

	content := GlowRed.Render("Révoquer la licence ?") + "\n\n" +
		Dim.Render("lic_id  ") + GlowMagent.Render(o.licenseID.String()[:8]+"…") + "\n" +
		Dim.Render("subject ") + Base.Render(o.subject) + "\n\n" +
		Dim.Render("raison :") + "\n" +
		o.input.View() + "\n\n" +
		Dim.Render("Suggestions : ") + chipLine + "\n\n" +
		HintKey.Render("↵") + HintText.Render(" révoquer   ") +
		HintKey.Render("esc") + HintText.Render(" annuler")

	return lipgloss.Place(62, 18, lipgloss.Center, lipgloss.Center, ModalDanger.Render(content))
}
