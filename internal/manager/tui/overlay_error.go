package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// errorOverlay shows an error message with an optional details block.
// Esc, enter, or q dismisses it.
type errorOverlay struct {
	title   string
	msg     string
	details string // optional pre-formatted detail block (e.g. stack trace)
}

// NewErrorOverlay is exported for use by cmd/tui-snap.
func NewErrorOverlay(title, msg string) Overlay { return newErrorOverlay(title, msg) }

func newErrorOverlay(title, msg string) *errorOverlay {
	return &errorOverlay{title: title, msg: msg}
}

// newErrorOverlayWithDetails constructs an error overlay with an extra details block.
func newErrorOverlayWithDetails(title, msg, details string) *errorOverlay {
	return &errorOverlay{title: title, msg: msg, details: details}
}

func (o *errorOverlay) Init() tea.Cmd { return nil }

func (o *errorOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return o, nil
	}
	switch km.String() {
	case "esc", "enter", "q":
		return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
	}
	return o, nil
}

func (o *errorOverlay) View() string {
	// ✗ prefix matches prototype `b-red` title style.
	content := GlowRed.Render("✗ "+o.title) + "\n\n" +
		Base.Render(o.msg)
	if o.details != "" {
		// Pre-formatted details in a dim code block.
		content += "\n\n" + lipgloss.NewStyle().
			Foreground(Palette.Red).
			Border(lipgloss.NormalBorder()).BorderForeground(Palette.Border).
			Padding(0, 1).
			Render(o.details)
	}
	const innerW = 52
	content += "\n\n" + renderButtons(innerW, button{
		label: "Fermer", hotkey: "↵", kind: btnDanger, focused: true,
	})
	return lipgloss.Place(58, 14, lipgloss.Center, lipgloss.Center, ModalDanger.Render(content))
}
