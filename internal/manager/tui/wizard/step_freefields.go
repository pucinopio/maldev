package wizard

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// FreeFieldsMsg is emitted when the operator confirms the free fields.
type FreeFieldsMsg struct {
	// Subject is the licence subject (e.g. user name / device label).
	// When empty the wizard falls back to the default "licence".
	Subject string
	// Audience is a comma-separated list of audience tags consumed by
	// the licence binary at validation time.
	Audience string
	// Fields are arbitrary metadata key/value pairs encoded into the
	// licence's Features list as "key=value" strings.
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

// StepFreeFields is step 6: subject + audience + arbitrary key/value pairs.
type StepFreeFields struct {
	subjectIn  textinput.Model
	audienceIn textinput.Model
	// fixedField is the focus position of the "fixed" inputs at the top:
	// 0=subject, 1=audience, -1=in the free-field rows.
	fixedField int
	rows       []freeFieldRow
	rowIdx     int  // which row is active when fixedField == -1
	colIdx     int  // 0=key, 1=value
	focused    bool
	bounds     core.Rect
}

// NewStepFreeFields constructs step 6 with one empty row + subject/audience
// inputs at the top.
func NewStepFreeFields() *StepFreeFields {
	si := textinput.New()
	si.Placeholder = "subject (e.g. alice@example.com)"
	si.CharLimit = 128
	ai := textinput.New()
	ai.Placeholder = "audience (comma-separated, e.g. prod,eu-west)"
	ai.CharLimit = 256
	return &StepFreeFields{
		subjectIn:  si,
		audienceIn: ai,
		fixedField: 0,
		rows:       []freeFieldRow{newFreeFieldRow()},
	}
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
		s.cycleNext()
		s.activeInput().Focus()
		return s, textinput.Blink

	case "shift+tab":
		s.activeInput().Blur()
		s.cyclePrev()
		s.activeInput().Focus()
		return s, textinput.Blink

	case "a":
		// Add a new row when not typing in any textinput.
		if !s.activeInput().Focused() {
			s.rows = append(s.rows, newFreeFieldRow())
		}
		return s, nil

	case "d":
		// Delete current row (only valid when focus is on a free-field row).
		if s.fixedField == -1 && !s.activeInput().Focused() && len(s.rows) > 1 {
			s.rows = append(s.rows[:s.rowIdx], s.rows[s.rowIdx+1:]...)
			if s.rowIdx >= len(s.rows) {
				s.rowIdx = len(s.rows) - 1
			}
		}
		return s, nil

	case "enter":
		// Enter while no input is focused = submit.
		if !s.activeInput().Focused() {
			return s, s.submitCmd()
		}
		// Enter on an empty active input = submit (caller meant "I'm done").
		if s.activeInput().Value() == "" {
			return s, s.submitCmd()
		}
		// Enter on a non-empty value: advance to the next field instead of
		// submitting so the operator can keep typing without surprise.
		s.activeInput().Blur()
		s.cycleNext()
		s.activeInput().Focus()
		return s, textinput.Blink

	case "ctrl+s", "esc":
		return s, s.submitCmd()
	}

	// Forward to active textinput.
	updated, cmd := s.activeInput().Update(km)
	s.setActiveInput(updated)
	return s, cmd
}

// cycleNext moves focus forward through subject → audience → row.key → row.val
// → next row.key → … wrapping back to subject.
func (s *StepFreeFields) cycleNext() {
	switch s.fixedField {
	case 0:
		s.fixedField = 1
	case 1:
		s.fixedField = -1
		s.rowIdx = 0
		s.colIdx = 0
	case -1:
		if s.colIdx == 0 {
			s.colIdx = 1
		} else {
			s.colIdx = 0
			s.rowIdx++
			if s.rowIdx >= len(s.rows) {
				s.rowIdx = 0
				s.fixedField = 0
			}
		}
	}
}

// cyclePrev moves focus backward in the symmetric order.
func (s *StepFreeFields) cyclePrev() {
	switch s.fixedField {
	case 0:
		s.fixedField = -1
		s.rowIdx = len(s.rows) - 1
		s.colIdx = 1
	case 1:
		s.fixedField = 0
	case -1:
		if s.colIdx == 1 {
			s.colIdx = 0
		} else {
			s.rowIdx--
			if s.rowIdx < 0 {
				s.fixedField = 1
				s.rowIdx = 0
			} else {
				s.colIdx = 1
			}
		}
	}
}

// activeInput returns a pointer to the currently focused textinput,
// depending on whether the fixed (subject/audience) inputs or one of the
// k/v rows is active.
func (s *StepFreeFields) activeInput() *textinput.Model {
	switch s.fixedField {
	case 0:
		return &s.subjectIn
	case 1:
		return &s.audienceIn
	}
	if s.colIdx == 0 {
		return &s.rows[s.rowIdx].keyIn
	}
	return &s.rows[s.rowIdx].valIn
}

func (s *StepFreeFields) setActiveInput(m textinput.Model) {
	switch s.fixedField {
	case 0:
		s.subjectIn = m
		return
	case 1:
		s.audienceIn = m
		return
	}
	if s.colIdx == 0 {
		s.rows[s.rowIdx].keyIn = m
	} else {
		s.rows[s.rowIdx].valIn = m
	}
}

func (s *StepFreeFields) submitCmd() tea.Cmd {
	subject := s.subjectIn.Value()
	audience := s.audienceIn.Value()
	fields := s.collect()
	return func() tea.Msg {
		return FreeFieldsMsg{Subject: subject, Audience: audience, Fields: fields}
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

	title := lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true).Render("Step 6 — Subject, Audience & Free Fields")
	sub := fgDim.Render("Identify the licence holder + audience tags, plus optional metadata.")
	header := lipgloss.JoinVertical(lipgloss.Left, title, sub, "")

	subjectPrefix := fg.Render("  Subject  :  ")
	audiencePrefix := fg.Render("  Audience :  ")
	if s.fixedField == 0 {
		subjectPrefix = sel.Render("> Subject  :  ")
	}
	if s.fixedField == 1 {
		audiencePrefix = sel.Render("> Audience :  ")
	}
	lines := []string{
		header,
		subjectPrefix + s.subjectIn.View(),
		audiencePrefix + s.audienceIn.View(),
		"",
		fgDim.Render("  Free fields (key / value):"),
	}
	for i, r := range s.rows {
		prefix := fg.Render("  ")
		if s.fixedField == -1 && i == s.rowIdx {
			prefix = sel.Render("> ")
		}
		line := prefix + r.keyIn.View() + "  =  " + r.valIn.View()
		lines = append(lines, line)
	}

	lines = append(lines, "",
		fgDim.Render("  tab next field   a add row   d delete row   enter/ctrl+s/esc confirm"),
	)
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (s *StepFreeFields) Focus() {
	s.focused = true
	s.fixedField = 0
	s.rowIdx = 0
	s.colIdx = 0
	s.subjectIn.Focus()
}
func (s *StepFreeFields) Blur() {
	s.focused = false
	s.subjectIn.Blur()
	s.audienceIn.Blur()
	for i := range s.rows {
		s.rows[i].keyIn.Blur()
		s.rows[i].valIn.Blur()
	}
}
func (s *StepFreeFields) Focused() bool { return s.focused }
