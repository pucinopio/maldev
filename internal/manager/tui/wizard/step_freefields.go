package wizard

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// FreeFieldsMsg is emitted when the operator confirms the free fields.
type FreeFieldsMsg struct {
	Fields map[string]string
}

// freeFieldRow holds one key/value pair being edited.
type freeFieldRow struct {
	keyIn textinput.Model
	valIn textinput.Model
}

func newFreeFieldRow() freeFieldRow {
	ki := textinput.New()
	ki.Placeholder = "key"
	ki.CharLimit = 64

	vi := textinput.New()
	vi.Placeholder = "value"
	vi.CharLimit = 256

	return freeFieldRow{keyIn: ki, valIn: vi}
}

// StepFreeFields is step 6: arbitrary key/value pairs.
type StepFreeFields struct {
	rows    []freeFieldRow
	rowIdx  int  // which row is active
	colIdx  int  // 0=key, 1=value
	focused bool
	bounds  core.Rect
}

// NewStepFreeFields constructs step 6 with one empty row.
func NewStepFreeFields() *StepFreeFields {
	return &StepFreeFields{rows: []freeFieldRow{newFreeFieldRow()}}
}

func (s *StepFreeFields) Layout(b core.Rect) { s.bounds = b }
func (s *StepFreeFields) Bounds() core.Rect  { return s.bounds }

func (s *StepFreeFields) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok || !s.focused {
		return s, nil
	}

	switch km.String() {
	case "tab":
		s.activeInput().Blur()
		if s.colIdx == 0 {
			s.colIdx = 1
		} else {
			s.colIdx = 0
			s.rowIdx = (s.rowIdx + 1) % len(s.rows)
		}
		s.activeInput().Focus()
		return s, textinput.Blink

	case "a":
		// Add a new row when not typing in a textinput.
		if !s.activeInput().Focused() {
			s.rows = append(s.rows, newFreeFieldRow())
		}
		return s, nil

	case "d":
		// Delete current row (keep at least one).
		if !s.activeInput().Focused() && len(s.rows) > 1 {
			s.rows = append(s.rows[:s.rowIdx], s.rows[s.rowIdx+1:]...)
			if s.rowIdx >= len(s.rows) {
				s.rowIdx = len(s.rows) - 1
			}
		}
		return s, nil

	case "enter":
		if !s.activeInput().Focused() {
			return s, func() tea.Msg { return FreeFieldsMsg{Fields: s.collect()} }
		}

	case "esc":
		return s, func() tea.Msg { return FreeFieldsMsg{Fields: s.collect()} }
	}

	// Forward to active textinput.
	updated, cmd := s.activeInput().Update(km)
	s.setActiveInput(updated)
	return s, cmd
}

// activeInput returns a pointer to the currently focused textinput.
func (s *StepFreeFields) activeInput() *textinput.Model {
	if s.colIdx == 0 {
		return &s.rows[s.rowIdx].keyIn
	}
	return &s.rows[s.rowIdx].valIn
}

func (s *StepFreeFields) setActiveInput(m textinput.Model) {
	if s.colIdx == 0 {
		s.rows[s.rowIdx].keyIn = m
	} else {
		s.rows[s.rowIdx].valIn = m
	}
}

func (s *StepFreeFields) collect() map[string]string {
	out := make(map[string]string, len(s.rows))
	for _, r := range s.rows {
		k := r.keyIn.Value()
		v := r.valIn.Value()
		if k != "" {
			out[k] = v
		}
	}
	return out
}

func (s *StepFreeFields) View() string {
	fgDim := lipgloss.NewStyle().Foreground(core.Colors.FgDim)
	fg := lipgloss.NewStyle().Foreground(core.Colors.Fg)
	sel := lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true)

	title := lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true).Render("Step 6 — Free Fields (optional)")
	sub := fgDim.Render("Add arbitrary key/value metadata to this licence.")
	header := lipgloss.JoinVertical(lipgloss.Left, title, sub, "")

	lines := []string{header, ""}
	for i, r := range s.rows {
		prefix := fg.Render("  ")
		if i == s.rowIdx {
			prefix = sel.Render("> ")
		}
		line := prefix + r.keyIn.View() + "  =  " + r.valIn.View()
		lines = append(lines, line)
	}

	lines = append(lines, "",
		fgDim.Render("  tab next field   a add row   d delete row   enter/esc confirm"),
	)
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (s *StepFreeFields) Focus() {
	s.focused = true
	s.rowIdx = 0
	s.colIdx = 0
	s.rows[0].keyIn.Focus()
}
func (s *StepFreeFields) Blur() {
	s.focused = false
	for i := range s.rows {
		s.rows[i].keyIn.Blur()
		s.rows[i].valIn.Blur()
	}
}
func (s *StepFreeFields) Focused() bool { return s.focused }
