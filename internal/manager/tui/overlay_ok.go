package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// okOverlay is a single-button confirmation that an action completed.
// Mirrors errorOverlay's shape but with a green title and ✓ glyph.
type okOverlay struct {
	title string
	body  string
}

// NewOKOverlay constructs the success modal.
func NewOKOverlay(title, body string) Overlay { return &okOverlay{title: title, body: body} }

func (o *okOverlay) Init() tea.Cmd { return nil }

func (o *okOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "esc", "enter", "q":
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
	case tea.MouseMsg:
		if m.Button == tea.MouseButtonLeft && m.Action == tea.MouseActionPress {
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
	}
	return o, nil
}

func (o *okOverlay) View() string {
	const innerW = 52
	content := GlowGreen.Render("✓ "+o.title) + "\n\n" + Base.Render(o.body) + "\n\n" +
		renderButtons(innerW, button{label: "OK", hotkey: "↵", kind: btnPrimary, focused: true})
	return lipgloss.Place(58, 12, lipgloss.Center, lipgloss.Center, ModalOK.Render(content))
}
