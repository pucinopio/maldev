package wizard

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
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
	svc     *service.Services
	mode    machineBMode
	pasteIn textinput.Model
	probed  []*ent.ProbeToken // recent probed machines, populated on Focus
	focused bool
	bounds  core.Rect
}

// NewStepBindingMachine constructs step 3.
func NewStepBindingMachine(svc *service.Services) *StepBindingMachine {
	ti := textinput.New()
	ti.Placeholder = "paste machine-id hex string…"
	ti.CharLimit = 256
	return &StepBindingMachine{svc: svc, pasteIn: ti}
}

// loadProbedCmd fetches the most recent probed machines so the operator can
// pick one instead of pasting the CompositeHex by hand.
func (s *StepBindingMachine) loadProbedCmd() tea.Cmd {
	svc := s.svc
	return func() tea.Msg {
		if svc == nil {
			return probedMachinesMsg{}
		}
		rows, _ := svc.Probe.History(context.Background(), 5)
		// Only entries that actually resolved a CompositeHex are useful — the
		// rest are pending tokens with no hostid yet.
		var out []*ent.ProbeToken
		for _, r := range rows {
			if r.CompositeHex != "" {
				out = append(out, r)
			}
		}
		return probedMachinesMsg{Rows: out}
	}
}

// probedMachinesMsg carries the recent-probe-token slice into Update.
type probedMachinesMsg struct{ Rows []*ent.ProbeToken }

func (s *StepBindingMachine) Layout(b core.Rect) { s.bounds = b }
func (s *StepBindingMachine) Bounds() core.Rect  { return s.bounds }

func (s *StepBindingMachine) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	if m, ok := msg.(probedMachinesMsg); ok {
		s.probed = m.Rows
		return s, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !s.focused {
			return s, nil
		}
		// Digits 1..5 → pick the corresponding probed machine.
		if len(msg.Runes) == 1 && msg.Runes[0] >= '1' && msg.Runes[0] <= '5' {
			idx := int(msg.Runes[0] - '1')
			if idx < len(s.probed) {
				mid := s.probed[idx].CompositeHex
				return s, func() tea.Msg { return MachineBindingMsg{MachineID: mid} }
			}
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
		case "esc", "ctrl+s":
			// esc and ctrl+s always skip — ctrl+s bypasses input focus so the
			// operator can skip even while typing in the paste field.
			return s, func() tea.Msg { return MachineBindingMsg{MachineID: ""} }
		case "s":
			// Bare 's' only skips when the paste textinput is not active
			// (otherwise it would be consumed as a literal character).
			if !s.pasteIn.Focused() {
				return s, func() tea.Msg { return MachineBindingMsg{MachineID: ""} }
			}
		case "enter":
			switch s.mode {
			case machinePaste:
				// Enter on empty paste field = skip the binding entirely so the
				// operator can press their way through this optional step
				// without having to remember the ctrl+s shortcut.
				val := strings.TrimSpace(s.pasteIn.Value())
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
		var probedSection string
		if len(s.probed) > 0 {
			lines := []string{lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true).Render("  Machines probées récentes (1..5):")}
			for i, p := range s.probed {
				if i >= 5 {
					break
				}
				host := p.Hostname
				if host == "" {
					host = "(unknown)"
				}
				short := p.CompositeHex
				if len(short) > 16 {
					short = short[:16] + "…"
				}
				key := lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true).Render(fmt.Sprintf("  [%d]", i+1))
				lines = append(lines, key+" "+fgDim.Render(host+"  ")+lipgloss.NewStyle().Foreground(core.Colors.Fg).Render(short))
			}
			probedSection = lipgloss.JoinVertical(lipgloss.Left, lines...) + "\n"
		}
		body = lipgloss.JoinVertical(lipgloss.Left,
			pasteStyle.Render("  [Paste]  "),
			"",
			probedSection,
			fgDim.Render("  Machine-ID hex (ou colle):"),
			"  "+s.pasteIn.View(),
			"",
			renderHints("enter confirm", "1-5 pick probed", "TAB probe mode", "ctrl+s/esc skip"),
		)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Left,
			probeStyle.Render("  [Probe target]  "),
			"",
			fgDim.Render("  Press enter to open the probe drawer."),
			fgDim.Render("  A one-time curl command will be shown for the target to run."),
			"",
			renderHints("enter open drawer", "TAB paste mode", "ctrl+s/esc skip"),
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

// FocusCmd is called by wizardModel.initStep when this step becomes active.
// Returns the cmd that loads probed machines for the suggestion list.
func (s *StepBindingMachine) FocusCmd() tea.Cmd { return s.loadProbedCmd() }
func (s *StepBindingMachine) Blur()         { s.focused = false; s.pasteIn.Blur() }
func (s *StepBindingMachine) Focused() bool { return s.focused }
