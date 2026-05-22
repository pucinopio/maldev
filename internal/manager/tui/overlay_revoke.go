package tui

import (
	"strings"

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

// NewRevokeOverlay is exported for use by cmd/tui-snap.
func NewRevokeOverlay(licenseID uuid.UUID, subject string) Overlay {
	return newRevokeOverlay(licenseID, subject)
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
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
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
	case tea.MouseMsg:
		if m.Button != tea.MouseButtonLeft || m.Action != tea.MouseActionPress {
			break
		}
		// 62x18 modal; footer is the last content line. Modal content ≈ 10 rows,
		// rendered with border+padding = 14 rows total. Centered in 18 → starts at
		// Y=2. Footer at content row 9 → overlay Y = 2 + 1 (border) + 1 (pad) + 9 = 13.
		if m.Y != 13 {
			break
		}
		if m.X < 31 {
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
		reason := o.input.Value()
		if reason == "" {
			return o, nil
		}
		id := o.licenseID
		return o, func() tea.Msg {
			return OverlayDoneMsg{Result: RevokeConfirmedMsg{LicenseID: id, Reason: reason}}
		}
	}
	var cmd tea.Cmd
	o.input, cmd = o.input.Update(msg)
	return o, cmd
}

var revokeChipStyle = lipgloss.NewStyle().
	Foreground(Palette.FgMute).
	Border(lipgloss.NormalBorder()).BorderForeground(Palette.Border)

func (o *revokeOverlay) View() string {
	suggestions := []string{
		"key_compromised", "offboarding", "leak", "decommissioned", "abuse",
	}
	// The modal is 62 cells wide with a 3-cell border+padding on each side
	// → 56 cells of usable inner width. Pack chips into rows that fit, with
	// 1-cell gaps; spill to a second row when a chip would overflow.
	const innerW = 56
	var rows []string
	var row strings.Builder
	rowW := 0
	for _, s := range suggestions {
		chip := revokeChipStyle.Render(s)
		cw := lipgloss.Width(chip)
		if rowW > 0 && rowW+1+cw > innerW {
			rows = append(rows, row.String())
			row.Reset()
			rowW = 0
		}
		if rowW > 0 {
			row.WriteString(" ")
			rowW++
		}
		row.WriteString(chip)
		rowW += cw
	}
	if row.Len() > 0 {
		rows = append(rows, row.String())
	}
	chipLines := strings.Join(rows, "\n")

	footer := renderButtons(innerW,
		button{label: "Annuler", hotkey: "esc", kind: btnNeutral},
		button{label: "Révoquer", hotkey: "↵", kind: btnDanger, focused: true},
	)
	content := GlowRed.Render("Révoquer la licence ?") + "\n\n" +
		Dim.Render("lic_id  ") + GlowMagent.Render(o.licenseID.String()[:8]+"…") + "\n" +
		Dim.Render("subject ") + Base.Render(o.subject) + "\n\n" +
		Dim.Render("raison :") + "\n" +
		o.input.View() + "\n\n" +
		Dim.Render("Suggestions :") + "\n" + chipLines + "\n\n" + footer

	// Height grows by the number of chip rows; clamp the Place height so
	// the modal never gets clipped under the global status bar.
	height := 18 + len(rows) - 1
	if height < 18 {
		height = 18
	}
	return lipgloss.Place(62, height, lipgloss.Center, lipgloss.Center, ModalDanger.Render(content))
}
