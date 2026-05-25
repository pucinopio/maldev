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

	// chipRects is the overlay-relative bounding boxes of the suggestion
	// chips, populated by View() so OnClick can hit-test them without
	// re-parsing the rendered string. Reset on every View() call because
	// the chip layout depends on overlay width (single vs two-row spill).
	chipRects []chipRect
	footerY   int // overlay-relative Y of the action button row
}

type chipRect struct {
	x1, x2, y int
	reason    string
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
		// Suggestion chips take priority — clicking one populates the input
		// without confirming, so the operator can edit before submitting.
		for _, c := range o.chipRects {
			if m.Y == c.y && m.X >= c.x1 && m.X < c.x2 {
				o.input.SetValue(c.reason)
				o.input.CursorEnd()
				return o, nil
			}
		}
		// Footer row: hit-test the Annuler / Révoquer buttons. footerY is
		// populated by View() because its position depends on the chip
		// layout (one vs two rows).
		if m.Y != o.footerY {
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
	//
	// Record the overlay-relative bounding box of each chip so OnClick can
	// hit-test it. lipgloss places the modal centered; we know:
	//   border(1) + padding(1) = 2 cells offset at the start of each row.
	// The chip strip is rendered after 8 content lines (title=1, blank=1,
	// lic_id=1, subject=1, blank=1, "raison :"=1, input=1, blank=1,
	// "Suggestions :"=1) → chip first row Y = 2 + 9 = 11.
	const innerW = 56
	const chipStartY = 11
	const chipStartX = 3 // border(1) + padding(2)
	o.chipRects = o.chipRects[:0]
	var rows []string
	var row strings.Builder
	rowW := 0
	rowIdx := 0
	for _, s := range suggestions {
		chip := revokeChipStyle.Render(s)
		cw := lipgloss.Width(chip)
		if rowW > 0 && rowW+1+cw > innerW {
			rows = append(rows, row.String())
			row.Reset()
			rowW = 0
			rowIdx++
		}
		if rowW > 0 {
			row.WriteString(" ")
			rowW++
		}
		x1 := chipStartX + rowW
		row.WriteString(chip)
		rowW += cw
		o.chipRects = append(o.chipRects, chipRect{
			x1: x1, x2: x1 + cw, y: chipStartY + rowIdx, reason: s,
		})
	}
	if row.Len() > 0 {
		rows = append(rows, row.String())
	}
	chipLines := strings.Join(rows, "\n")
	// Footer Y = first chip row Y + (chip rows - 1) + 2 blank lines below.
	o.footerY = chipStartY + len(rows) - 1 + 2

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
