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
		// Modal is centered inside a 54x12 lipgloss.Place box. Top padding =
		// (12-9)/2 = 1 row, so the footer line sits at overlay Y=7. Cancel
		// occupies the left half of the inner row, Confirm the right half.
		// Coordinates have already been translated to overlay-relative by
		// rootModel.updateOverlay.
		if m.Y != 7 {
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
	return lipgloss.Place(54, 12, lipgloss.Center, lipgloss.Center, style.Render(content))
}
