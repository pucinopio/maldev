package wizard

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// MachineBindingMsg is emitted when the operator has provided (or skipped)
// machine binding.
type MachineBindingMsg struct {
	MachineID string // empty = skip
}

// machineBMode tracks whether we are in paste or probe sub-mode.
type machineBMode int

const (
	machinePaste machineBMode = iota
	machineProbe
)

// StepBindingMachine is step 3 of the wizard: optional machine-ID binding.
type StepBindingMachine struct {
	mode    machineBMode
	pasteIn textinput.Model
	focused bool
	bounds  core.Rect
}

// NewStepBindingMachine constructs step 3.
func NewStepBindingMachine() *StepBindingMachine {
	ti := textinput.New()
	ti.Placeholder = "paste machine-id hex string…"
	ti.CharLimit = 256
	return &StepBindingMachine{pasteIn: ti}
}

func (s *StepBindingMachine) Layout(b core.Rect) { s.bounds = b }
func (s *StepBindingMachine) Bounds() core.Rect  { return s.bounds }

func (s *StepBindingMachine) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !s.focused {
			return s, nil
		}
		switch msg.String() {
		case "tab":
			// Toggle between paste and probe sub-modes.
			if s.mode == machinePaste {
				s.mode = machineProbe
				s.pasteIn.Blur()
			} else {
				s.mode = machinePaste
				s.pasteIn.Focus()
				return s, textinput.Blink
			}
			return s, nil
		case "esc":
			// esc always skips regardless of focus state.
			return s, func() tea.Msg { return MachineBindingMsg{MachineID: ""} }
		case "s":
			// Skip only when the paste textinput is not active.
			if !s.pasteIn.Focused() {
				return s, func() tea.Msg { return MachineBindingMsg{MachineID: ""} }
			}
		case "enter":
			switch s.mode {
			case machinePaste:
				val := strings.TrimSpace(s.pasteIn.Value())
				if val == "" {
					return s, nil
				}
				return s, func() tea.Msg { return MachineBindingMsg{MachineID: val} }
			case machineProbe:
				return s, func() tea.Msg { return OpenProbeDrawerMsg{} }
			}
		}
		// Forward remaining key events to the paste textinput.
		if s.mode == machinePaste && s.pasteIn.Focused() {
			updated, cmd := s.pasteIn.Update(msg)
			s.pasteIn = updated
			return s, cmd
		}
	}
	if s.mode == machinePaste && s.pasteIn.Focused() {
		updated, cmd := s.pasteIn.Update(msg)
		s.pasteIn = updated
		return s, cmd
	}
	return s, nil
}

// OpenProbeDrawerMsg asks the wizard to slide open the probe drawer.
type OpenProbeDrawerMsg struct{}

func (s *StepBindingMachine) View() string {
	fgDim := lipgloss.NewStyle().Foreground(core.Colors.FgDim)
	title := lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true).Render("Step 3 — Machine Binding (optional)")
	sub := fgDim.Render("Bind this licence to a specific machine ID. Tab to switch input method.")
	header := lipgloss.JoinVertical(lipgloss.Left, title, sub, "")

	pasteStyle := fgDim
	probeStyle := fgDim
	if s.mode == machinePaste {
		pasteStyle = lipgloss.NewStyle().Foreground(core.Colors.Fg).Bold(true)
	} else {
		probeStyle = lipgloss.NewStyle().Foreground(core.Colors.Fg).Bold(true)
	}

	var body string
	if s.mode == machinePaste {
		body = lipgloss.JoinVertical(lipgloss.Left,
			pasteStyle.Render("  [Paste]  "),
			"",
			fgDim.Render("  Machine-ID hex:"),
			"  "+s.pasteIn.View(),
			"",
			fgDim.Render("  enter confirm   tab probe mode   s/esc skip"),
		)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Left,
			probeStyle.Render("  [Probe target]  "),
			"",
			fgDim.Render("  Press enter to open the probe drawer."),
			fgDim.Render("  A one-time curl command will be shown for the target to run."),
			"",
			fgDim.Render("  enter open drawer   tab paste mode   s/esc skip"),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (s *StepBindingMachine) Focus() {
	s.focused = true
	if s.mode == machinePaste {
		s.pasteIn.Focus()
	}
}
func (s *StepBindingMachine) Blur()         { s.focused = false; s.pasteIn.Blur() }
func (s *StepBindingMachine) Focused() bool { return s.focused }
