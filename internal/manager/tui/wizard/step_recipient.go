package wizard

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// RecipientLoadedMsg carries the result of fetching all recipients.
type RecipientLoadedMsg struct {
	Rows []*ent.RecipientKey
	Err  error
}

// RecipientChosenMsg is emitted when a recipient has been picked.
type RecipientChosenMsg struct {
	RecipientID string // UUID string; empty = skip (no sealed payload)
}

// StepRecipient is step 2 of the wizard: pick or create a recipient.
type StepRecipient struct {
	svc     *service.Services
	rows    []*ent.RecipientKey
	cursor  int
	mode    recipientMode
	nameIn  textinput.Model
	focused bool
	bounds  core.Rect
	err     string
}

type recipientMode int

const (
	recipientBrowse recipientMode = iota
	recipientCreate
)

// NewStepRecipient constructs the recipient step.
func NewStepRecipient(svc *service.Services) *StepRecipient {
	ni := textinput.New()
	ni.Placeholder = "recipient name (e.g. acme-corp)"
	ni.CharLimit = 80

	return &StepRecipient{svc: svc, nameIn: ni}
}

func (s *StepRecipient) Layout(b core.Rect) { s.bounds = b }
func (s *StepRecipient) Bounds() core.Rect  { return s.bounds }

func (s *StepRecipient) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case RecipientLoadedMsg:
		s.err = ""
		if msg.Err != nil {
			s.err = msg.Err.Error()
		}
		s.rows = msg.Rows
		s.cursor = 0
		return s, nil

	case tea.KeyMsg:
		if !s.focused {
			return s, nil
		}
		switch s.mode {
		case recipientBrowse:
			return s, s.handleBrowseKey(msg)
		case recipientCreate:
			// handleCreateKey consumes structural keys (enter, esc).
			// Only forward to textinput when those keys were not consumed.
			if cmd := s.handleCreateKey(msg); cmd != nil {
				return s, cmd
			}
			// esc resets to browse mode — don't forward to input.
			if s.mode == recipientBrowse {
				return s, nil
			}
			updated, iCmd := s.nameIn.Update(msg)
			s.nameIn = updated
			return s, iCmd
		}
	}
	return s, nil
}

// OnClick handles browse-mode row clicks. body-local Y; header takes 3 rows
// (title + sub + blank). Recipient rows first, then "skip" sentinel, then
// "create new" sentinel.
func (s *StepRecipient) OnClick(_, y int) tea.Cmd {
	if s.mode == recipientCreate {
		return nil
	}
	const headerH = 3
	idx := y - headerH
	if idx < 0 {
		return nil
	}
	skipIdx := len(s.rows)
	createIdx := len(s.rows) + 1
	switch {
	case idx < len(s.rows):
		s.cursor = idx
		id := s.rows[idx].ID.String()
		return func() tea.Msg { return RecipientChosenMsg{RecipientID: id} }
	case idx == skipIdx:
		s.cursor = skipIdx
		return func() tea.Msg { return RecipientChosenMsg{RecipientID: ""} }
	case idx == createIdx:
		s.cursor = createIdx
		s.mode = recipientCreate
		s.nameIn.Focus()
		return textinput.Blink
	}
	return nil
}

func (s *StepRecipient) handleBrowseKey(msg tea.KeyMsg) tea.Cmd {
	// +2 for "create new" and "skip" sentinels
	total := len(s.rows) + 2
	switch msg.String() {
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < total-1 {
			s.cursor++
		}
	case "enter":
		skipIdx := len(s.rows)
		createIdx := len(s.rows) + 1
		switch s.cursor {
		case createIdx:
			s.mode = recipientCreate
			s.nameIn.Focus()
			return textinput.Blink
		case skipIdx:
			return func() tea.Msg { return RecipientChosenMsg{RecipientID: ""} }
		default:
			if s.cursor < len(s.rows) {
				id := s.rows[s.cursor].ID.String()
				return func() tea.Msg { return RecipientChosenMsg{RecipientID: id} }
			}
		}
	}
	return nil
}

func (s *StepRecipient) handleCreateKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		name := s.nameIn.Value()
		if name == "" {
			s.err = "name is required"
			return nil
		}
		svc := s.svc
		return func() tea.Msg {
			// Idempotent create: if a key with this name already exists (e.g.
			// the operator went Back/Next and revisited this step), reuse it
			// rather than failing with a duplicate-name error.
			existing, _ := svc.Recipient.List(context.Background())
			for _, e := range existing {
				if e.Name == name {
					return RecipientChosenMsg{RecipientID: e.ID.String()}
				}
			}
			row, err := svc.Recipient.Generate(context.Background(), name, "operator")
			if err != nil {
				return RecipientLoadedMsg{Err: err}
			}
			return RecipientChosenMsg{RecipientID: row.ID.String()}
		}
	case "esc":
		s.mode = recipientBrowse
		s.nameIn.Blur()
		s.nameIn.SetValue("")
		s.err = ""
	}
	return nil
}

func (s *StepRecipient) View() string {
	title := lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true).Render("Step 2 — Recipient")
	sub := lipgloss.NewStyle().Foreground(core.Colors.FgDim).Render("Pick or create the X25519 recipient key for sealed payload delivery.")
	header := lipgloss.JoinVertical(lipgloss.Left, title, sub, "")

	if s.mode == recipientCreate {
		return lipgloss.JoinVertical(lipgloss.Left, header, s.renderCreateForm())
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, s.renderList())
}

func (s *StepRecipient) renderList() string {
	fgDim := lipgloss.NewStyle().Foreground(core.Colors.FgDim)
	fg := lipgloss.NewStyle().Foreground(core.Colors.Fg)
	sel := lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true)

	var lines []string
	for i, r := range s.rows {
		label := fmt.Sprintf("  %-30s", r.Name)
		if i == s.cursor {
			lines = append(lines, sel.Render("> "+label))
		} else {
			lines = append(lines, fg.Render("  "+label))
		}
	}

	skipIdx := len(s.rows)
	createIdx := len(s.rows) + 1

	skipLabel := "  — Skip (no sealed payload)"
	if s.cursor == skipIdx {
		lines = append(lines, sel.Render(">"+skipLabel))
	} else {
		lines = append(lines, fgDim.Render(" "+skipLabel))
	}

	createLabel := "  + Create new recipient"
	if s.cursor == createIdx {
		lines = append(lines, sel.Render(">"+createLabel))
	} else {
		lines = append(lines, fgDim.Render(" "+createLabel))
	}

	// When there are no rows the list only has the two sentinel entries.
	if len(s.rows) == 0 {
		lines = []string{
			fgDim.Render("  (no recipients yet)"),
			lines[0], // skip sentinel
			lines[1], // create sentinel
		}
	}

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	if s.err != "" {
		return lipgloss.JoinVertical(lipgloss.Left,
			body,
			lipgloss.NewStyle().Foreground(core.Colors.Red).Render("  error: "+s.err),
		)
	}
	return body
}

func (s *StepRecipient) renderCreateForm() string {
	fgDim := lipgloss.NewStyle().Foreground(core.Colors.FgDim)
	lines := []string{
		fgDim.Render("  Recipient name:"),
		"  " + s.nameIn.View(),
	}
	if s.err != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(core.Colors.Red).Render("  "+s.err))
	}
	lines = append(lines, "", fgDim.Render("  enter confirm   esc cancel"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (s *StepRecipient) Focus()        { s.focused = true }
func (s *StepRecipient) Blur()         { s.focused = false }
func (s *StepRecipient) Focused() bool { return s.focused }

// LoadCmd fetches the recipient list.
func (s *StepRecipient) LoadCmd() tea.Cmd {
	svc := s.svc
	return func() tea.Msg {
		if svc == nil {
			return RecipientLoadedMsg{}
		}
		rows, err := svc.Recipient.List(context.Background())
		return RecipientLoadedMsg{Rows: rows, Err: err}
	}
}
