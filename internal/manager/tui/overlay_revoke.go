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

// revokeChipStyle is a flat chip — `· reason ·` separators around dim text.
// Previously this style added a NormalBorder, which made each chip a
// 3-line bordered block; that broke alignment when several chips were
// concatenated horizontally with string-builder (the embedded \n inside
// chip[i] left chip[i+1] on the next row instead of beside it).
var revokeChipStyle = Mute.Padding(0, 1)
var revokeChipSep = Dim.Render(" · ")

func (o *revokeOverlay) View() string {
	suggestions := []string{
		"key_compromised", "offboarding", "leak", "decommissioned", "abuse",
	}
	// Modal: 62 cells wide, border(1) + padding(1,2) per side → text room
	// = 62 - 2 (border) - 4 (padding) = 56 cells inside the styled box.
	// The chips render on ONE line: each chip = " label " (padding(0,1)),
	// joined by " · ". They fit at this width (typical sum = 75 chars but
	// the longest individual chip is "decommissioned" = 16 → if they spill,
	// JoinHorizontal still keeps them on one rendered row by definition;
	// we measure the rendered width and let the modal auto-wrap only if
	// it would exceed the 56-cell room — in which case we split into rows
	// using lipgloss.JoinHorizontal for proper multi-row layout).
	const innerW = 56
	const chipStartY = 11
	const chipStartX = 3 // border(1) + padding(2)
	o.chipRects = o.chipRects[:0]

	// Pack chips into rows of <= innerW cells. Each row is a single-line
	// string joined by revokeChipSep; rows are then JoinVertical'd.
	var rows []string
	var rowChips []string
	var rowChipReasons []string
	rowW := 0
	rowIdx := 0
	flushRow := func() {
		if len(rowChips) == 0 {
			return
		}
		joined := strings.Join(rowChips, revokeChipSep)
		// Record per-chip X bounds for this row.
		cursor := chipStartX
		sepW := lipgloss.Width(revokeChipSep)
		for i, c := range rowChips {
			w := lipgloss.Width(c)
			o.chipRects = append(o.chipRects, chipRect{
				x1: cursor, x2: cursor + w, y: chipStartY + rowIdx, reason: rowChipReasons[i],
			})
			cursor += w + sepW
		}
		rows = append(rows, joined)
		rowChips = rowChips[:0]
		rowChipReasons = rowChipReasons[:0]
		rowW = 0
		rowIdx++
	}
	sepW := lipgloss.Width(revokeChipSep)
	for _, s := range suggestions {
		chip := revokeChipStyle.Render(s)
		cw := lipgloss.Width(chip)
		need := cw
		if len(rowChips) > 0 {
			need += sepW
		}
		if rowW+need > innerW {
			flushRow()
			need = cw
		}
		rowChips = append(rowChips, chip)
		rowChipReasons = append(rowChipReasons, s)
		rowW += need
	}
	flushRow()
	chipLines := strings.Join(rows, "\n")
	// Footer Y = chipStartY + (rendered chip rows - 1) + 1 blank + 1 line.
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

	// Height grows by the number of chip rows; chips are now single-line
	// each so +len(rows)-1 still accurately reflects the extra rows.
	height := 18 + len(rows) - 1
	if height < 18 {
		height = 18
	}
	return lipgloss.Place(62, height, lipgloss.Center, lipgloss.Center, ModalDanger.Render(content))
}
