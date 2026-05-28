package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmResultMsg is emitted by confirmOverlay when the user makes a choice.
type ConfirmResultMsg struct {
	ID      string // caller-supplied identifier to disambiguate nested confirms
	Confirm bool
}

// confirmOverlay is a generic yes/no modal.
type confirmOverlay struct {
	id           string
	title        string
	body         string
	confirmLabel string
	cancelLabel  string
	danger       bool

	// footerY is the overlay-relative Y of the footer row, populated by
	// View() at render time. The mouse handler reads it instead of
	// hardcoding a value — bodies wider than one line shift the footer
	// down via lipgloss.Place's vertical centering, and a fixed Y=7
	// missed every multi-line confirm.
	footerY int
}

// NewConfirmOverlay is exported for use by cmd/tui-snap.
// newConfirmOverlay is the package-internal alias used by screen files and tests.
func NewConfirmOverlay(id, title, body, confirmLabel, cancelLabel string, danger bool) Overlay {
	return newConfirmOverlay(id, title, body, confirmLabel, cancelLabel, danger)
}

func newConfirmOverlay(id, title, body, confirmLabel, cancelLabel string, danger bool) *confirmOverlay {
	return &confirmOverlay{
		id: id, title: title, body: body,
		confirmLabel: confirmLabel, cancelLabel: cancelLabel, danger: danger,
	}
}

func (o *confirmOverlay) Init() tea.Cmd { return nil }

func (o *confirmOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	id := o.id
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "y", "Y", "enter":
			return o, func() tea.Msg {
				return OverlayDoneMsg{Result: ConfirmResultMsg{ID: id, Confirm: true}}
			}
		case "n", "N", "esc", "q":
			return o, func() tea.Msg {
				return OverlayDoneMsg{Result: ConfirmResultMsg{ID: id, Confirm: false}}
			}
		}
	case tea.MouseMsg:
		if m.Button != tea.MouseButtonLeft || m.Action != tea.MouseActionPress {
			return o, nil
		}
		// Footer Y is populated by View() so multi-line bodies (which shift
		// the footer downward via lipgloss.Place vertical centering) still
		// hit-test correctly. View must have run at least once — when it
		// hasn't (test paths that click before rendering) fall back to the
		// historical Y=7 to preserve the single-line behaviour.
		footerY := o.footerY
		if footerY == 0 {
			footerY = 7
		}
		if m.Y != footerY {
			return o, nil
		}
		// Inside the modal, the button row spans X≈3..51 (border+padding=3 left).
		// Cancel is the left button (X<27), Confirm is the right (X≥27).
		if m.X < 27 {
			return o, func() tea.Msg {
				return OverlayDoneMsg{Result: ConfirmResultMsg{ID: id, Confirm: false}}
			}
		}
		return o, func() tea.Msg {
			return OverlayDoneMsg{Result: ConfirmResultMsg{ID: id, Confirm: true}}
		}
	}
	return o, nil
}

func (o *confirmOverlay) View() string {
	var title string
	if o.danger {
		title = GlowRed.Render(o.title)
	} else {
		title = GlowMagent.Render(o.title)
	}
	const innerW = 48
	confirmKind := btnPrimary
	if o.danger {
		confirmKind = btnDanger
	}
	footer := renderButtons(innerW, button{
		label: o.cancelLabel, hotkey: "esc", kind: btnNeutral,
	}, button{
		label: o.confirmLabel, hotkey: "↵", kind: confirmKind, focused: true,
	})
	content := title + "\n\n" + Dim.Render(o.body) + "\n\n" + footer

	style := Modal
	if o.danger {
		style = ModalDanger
	}
	rendered := style.Render(content)

	// Place height auto-grows with the body so the modal never clips and the
	// footer position is fully derivable. Minimum 12 preserves the historical
	// look for short bodies; longer bodies inflate it row-for-row.
	modalH := lipgloss.Height(rendered)
	placeH := 12
	if modalH+2 > placeH {
		placeH = modalH + 2
	}
	topPad := (placeH - modalH) / 2
	// Inside the rendered modal the layout is: top border (1) + top padding
	// (1) + content rows + bottom padding (1) + bottom border (1). The footer
	// is the LAST content line, so it sits at modalH - 1 - 1 - 1 = modalH - 3
	// relative to the modal start, plus topPad in the Place canvas.
	// For the historical 5-content-row body this resolves to topPad(1) +
	// 9 - 3 = 7, matching the pre-fix hardcoded value.
	o.footerY = topPad + modalH - 3

	return lipgloss.Place(54, placeH, lipgloss.Center, lipgloss.Center, rendered)
}
