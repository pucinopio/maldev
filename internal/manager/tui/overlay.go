package tui

import tea "github.com/charmbracelet/bubbletea"

// Overlay is a modal layer rendered on top of the main content.
// Each overlay handles its own key events and signals removal via OverlayDoneMsg.
type Overlay interface {
	Init() tea.Cmd
	Update(tea.Msg) (Overlay, tea.Cmd)
	View() string
}

// OverlayDoneMsg is sent by an overlay when it has finished and wants to be
// popped from the stack. Result carries any data the overlay produced.
type OverlayDoneMsg struct {
	Result any
}
