package wizard

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// ValidityMsg is emitted when the operator confirms the validity window.
type ValidityMsg struct {
	NotBefore time.Time
	NotAfter  time.Time
}

// validityField tracks which field has focus inside this step.
type validityField int

const (
	validityFieldStart validityField = iota
	validityFieldEnd
)

// StepValidity is step 5: start/end dates with duration shortcuts.
type StepValidity struct {
	startIn textinput.Model
	endIn   textinput.Model
	active  validityField
	errMsg  string
	focused bool
	bounds  core.Rect
}

// NewStepValidity constructs step 5 with sensible defaults.
func NewStepValidity() *StepValidity {
	now := time.Now()
	end := now.AddDate(1, 0, 0)

	si := textinput.New()
	si.Placeholder = "YYYY-MM-DD"
	si.CharLimit = 10
	si.SetValue(now.Format("2006-01-02"))

	ei := textinput.New()
	ei.Placeholder = "YYYY-MM-DD or 'forever'"
	ei.CharLimit = 10
	ei.SetValue(end.Format("2006-01-02"))

	return &StepValidity{startIn: si, endIn: ei}
}

func (s *StepValidity) Layout(b core.Rect) { s.bounds = b }
func (s *StepValidity) Bounds() core.Rect  { return s.bounds }

func (s *StepValidity) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !s.focused {
			return s, nil
		}
		switch msg.String() {
		case "tab":
			s.cycleField()
			return s, textinput.Blink
		case "enter":
			nb, na, err := s.parse()
			if err != nil {
				s.errMsg = err.Error()
				return s, nil
			}
			return s, func() tea.Msg { return ValidityMsg{NotBefore: nb, NotAfter: na} }

		// Duration shortcuts — single-key, only active when end field is focused.
		// 7 = +7d, 3 = +30d, y = +1y, f = forever.
		case "7":
			if s.active == validityFieldEnd && !s.endIn.Focused() {
				s.applyShortcut(7 * 24 * time.Hour)
				return s, nil
			}
		case "3":
			if s.active == validityFieldEnd && !s.endIn.Focused() {
				s.applyShortcut(30 * 24 * time.Hour)
				return s, nil
			}
		case "y":
			if s.active == validityFieldEnd && !s.endIn.Focused() {
				s.applyShortcut(365 * 24 * time.Hour)
				return s, nil
			}
		case "f":
			if s.active == validityFieldEnd && !s.endIn.Focused() {
				s.endIn.SetValue("forever")
				return s, nil
			}
		}
		// Forward to active field.
		return s, s.forwardKey(msg)
	}
	return s, s.forwardKey(msg)
}

func (s *StepValidity) cycleField() {
	if s.active == validityFieldStart {
		s.startIn.Blur()
		s.active = validityFieldEnd
		s.endIn.Focus()
	} else {
		s.endIn.Blur()
		s.active = validityFieldStart
		s.startIn.Focus()
	}
}

func (s *StepValidity) forwardKey(msg tea.Msg) tea.Cmd {
	if s.active == validityFieldStart {
		updated, cmd := s.startIn.Update(msg)
		s.startIn = updated
		return cmd
	}
	updated, cmd := s.endIn.Update(msg)
	s.endIn = updated
	return cmd
}

func (s *StepValidity) applyShortcut(d time.Duration) {
	nb, _, err := s.parse()
	if err != nil {
		nb = time.Now()
	}
	s.endIn.SetValue(nb.Add(d).Format("2006-01-02"))
}

func (s *StepValidity) parse() (time.Time, time.Time, error) {
	nb, err := time.ParseInLocation("2006-01-02", s.startIn.Value(), time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start date: %w", err)
	}
	raw := s.endIn.Value()
	if raw == "forever" {
		return nb, time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), nil
	}
	na, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end date: %w", err)
	}
	if !na.After(nb) {
		return time.Time{}, time.Time{}, fmt.Errorf("end date must be after start date")
	}
	return nb, na, nil
}

func (s *StepValidity) View() string {
	fgDim := lipgloss.NewStyle().Foreground(core.Colors.FgDim)
	red := lipgloss.NewStyle().Foreground(core.Colors.Red)

	title := lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true).Render("Step 5 — Validity Window")
	sub := fgDim.Render("Set the not-before / not-after dates for this licence.")
	header := lipgloss.JoinVertical(lipgloss.Left, title, sub, "")

	startLabel := fgDim.Render("  Not before:")
	endLabel := fgDim.Render("  Not after (or 'forever'):")
	if s.active == validityFieldStart {
		startLabel = lipgloss.NewStyle().Foreground(core.Colors.Fg).Bold(true).Render("  Not before:")
	} else {
		endLabel = lipgloss.NewStyle().Foreground(core.Colors.Fg).Bold(true).Render("  Not after (or 'forever'):")
	}

	lines := []string{
		startLabel,
		"  " + s.startIn.View(),
		"",
		endLabel,
		"  " + s.endIn.View(),
		"",
		fgDim.Render("  shortcuts: +7d  +30d  +1y  forever(0)"),
		fgDim.Render("  tab switch field   enter confirm"),
	}
	if s.errMsg != "" {
		lines = append(lines, red.Render("  "+s.errMsg))
	}
	return lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, lines...)...)
}

func (s *StepValidity) Focus() {
	s.focused = true
	s.active = validityFieldStart
	s.startIn.Focus()
}
func (s *StepValidity) Blur() {
	s.focused = false
	s.startIn.Blur()
	s.endIn.Blur()
}
func (s *StepValidity) Focused() bool { return s.focused }
