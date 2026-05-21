package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// placeholderModel renders a "not yet implemented" screen for views 2-9.
type placeholderModel struct {
	id    ViewID
	phase string
	width int
	hgt   int
}

func newPlaceholderModel(id ViewID, phase string) placeholderModel {
	return placeholderModel{id: id, phase: phase}
}

func (m placeholderModel) Init() tea.Cmd { return nil }

func (m placeholderModel) Update(msg tea.Msg) (placeholderModel, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.hgt = ws.Height
	}
	return m, nil
}

func (m placeholderModel) View() string {
	msg := fmt.Sprintf("%s — not yet implemented", m.phase)
	content := lipgloss.JoinVertical(lipgloss.Center,
		GlowMagent.Render(string(m.id)),
		"",
		Dim.Render(msg),
	)
	w := m.width
	h := m.hgt
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}
