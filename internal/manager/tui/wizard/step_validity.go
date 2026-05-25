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

		// Duration shortcuts — ctrl-prefixed so they don't collide with digit
		// entry inside the YYYY-MM-DD textinput. Always apply to the end date
		// regardless of which field has focus, and blur the input so the new
		// value is visible immediately.
		case "ctrl+w":
			s.applyShortcut(7 * 24 * time.Hour)
			return s, nil
		case "ctrl+m":
			s.applyShortcut(30 * 24 * time.Hour)
			return s, nil
		case "ctrl+y":
			s.applyShortcut(365 * 24 * time.Hour)
			return s, nil
		case "ctrl+f":
			s.endIn.SetValue("forever")
			return s, nil
		}
		// Forward to active field.
		return s, s.forwardKey(msg)
	}
	return s, s.forwardKey(msg)
}

// OnClick maps clicks on the "shortcuts: ..." line to the preset they
// describe. body-local coords; header(3) + 6 body lines → shortcuts row Y=9.
// The line reads: "  shortcuts: ctrl+w +7d   ctrl+m +30d   ctrl+y +1y   ctrl+f forever".
func (s *StepValidity) OnClick(x, y int) tea.Cmd {
	const shortcutsY = 9
	if y != shortcutsY {
		return nil
	}
	switch {
	case x >= 13 && x < 24:
		s.applyShortcut(7 * 24 * time.Hour)
	case x >= 27 && x < 38:
		s.applyShortcut(30 * 24 * time.Hour)
	case x >= 41 && x < 51:
		s.applyShortcut(365 * 24 * time.Hour)
	case x >= 54 && x < 68:
		s.endIn.SetValue("forever")
	}
	return nil
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
	header := stepHeader("Step 5 — Validity Window",
		"Set the not-before / not-after dates for this licence.")

	activeLabel := wizFg.Bold(true)
	startLabel := wizDim.Render("  Not before:")
	endLabel := wizDim.Render("  Not after (or 'forever'):")
	if s.active == validityFieldStart {
		startLabel = activeLabel.Render("  Not before:")
	} else {
		endLabel = activeLabel.Render("  Not after (or 'forever'):")
	}

	lines := []string{
		startLabel,
		"  " + s.startIn.View(),
		"",
		endLabel,
		"  " + s.endIn.View(),
		"",
		wizDim.Render("  shortcuts: ctrl+w +7d   ctrl+m +30d   ctrl+y +1y   ctrl+f forever"),
		wizDim.Render("  tab switch field   enter confirm"),
	}
	if s.errMsg != "" {
		lines = append(lines, wizRed.Render("  "+s.errMsg))
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
