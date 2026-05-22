package wizard

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// TOTPSecretsLoadedMsg carries TOTP secrets for a given issuer.
type TOTPSecretsLoadedMsg struct {
	Rows []*ent.TOTPSecret
	Err  error
}

// TOTPChoiceMsg is emitted when the operator confirms the TOTP preference.
type TOTPChoiceMsg struct {
	Require      bool
	TOTPSecretID string // UUID string; empty when Require is false
}

// StepTOTP is step 7: optionally require TOTP at validation.
type StepTOTP struct {
	svc       *service.Services
	issuerID  string // set by wizard before loading
	rows      []*ent.TOTPSecret
	cursor    int
	requireOn bool
	focused   bool
	bounds    core.Rect
	err       string
}

// NewStepTOTP constructs step 7.
func NewStepTOTP(svc *service.Services) *StepTOTP {
	return &StepTOTP{svc: svc}
}

// SetIssuerID tells the step which issuer to load TOTP secrets for.
func (s *StepTOTP) SetIssuerID(id string) { s.issuerID = id }

func (s *StepTOTP) Layout(b core.Rect) { s.bounds = b }
func (s *StepTOTP) Bounds() core.Rect  { return s.bounds }

// OnClick is called by the wizard mouse router with body-local coords.
// Toggle row lives at Y=4 (header=3 lines + blank at 3 + toggle at 4); when
// the toggle is on, secret rows start at Y=8 ("Select TOTP secret:" at Y=6,
// blank at 7, list from Y=8 down). Mutates state directly via pointer
// receiver — no Cmd round-trip needed.
func (s *StepTOTP) OnClick(_, y int) tea.Cmd {
	if y == 4 {
		s.requireOn = !s.requireOn
		return nil
	}
	if s.requireOn && y >= 8 {
		row := y - 8
		if row >= 0 && row < len(s.rows) {
			s.cursor = row
		}
	}
	return nil
}

func (s *StepTOTP) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case TOTPSecretsLoadedMsg:
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
		switch msg.String() {
		case "t":
			s.requireOn = !s.requireOn
			return s, nil
		case "up", "k":
			if s.requireOn && s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.requireOn && s.cursor < len(s.rows)-1 {
				s.cursor++
			}
		case "enter", "esc":
			if !s.requireOn {
				return s, func() tea.Msg { return TOTPChoiceMsg{Require: false} }
			}
			if len(s.rows) == 0 {
				s.err = "no TOTP secrets available for this issuer"
				return s, nil
			}
			id := s.rows[s.cursor].ID.String()
			return s, func() tea.Msg {
				return TOTPChoiceMsg{Require: true, TOTPSecretID: id}
			}
		}
	}
	return s, nil
}

func (s *StepTOTP) View() string {
	fgDim := lipgloss.NewStyle().Foreground(core.Colors.FgDim)
	fg := lipgloss.NewStyle().Foreground(core.Colors.Fg)
	sel := lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true)
	green := lipgloss.NewStyle().Foreground(core.Colors.Green)
	red := lipgloss.NewStyle().Foreground(core.Colors.Red)

	title := lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true).Render("Step 7 — TOTP Requirement")
	sub := fgDim.Render("Require a time-based one-time password at validation time.")
	header := lipgloss.JoinVertical(lipgloss.Left, title, sub, "")

	var toggleLabel string
	if s.requireOn {
		toggleLabel = green.Render("  [x] Require TOTP")
	} else {
		toggleLabel = fgDim.Render("  [ ] Require TOTP")
	}

	lines := []string{header, "", toggleLabel, ""}

	if s.requireOn {
		lines = append(lines, fg.Render("  Select TOTP secret:"))
		if len(s.rows) == 0 {
			lines = append(lines, fgDim.Render("  (no TOTP secrets linked to this issuer)"))
		} else {
			for i, r := range s.rows {
				label := "  " + r.AccountLabel
				if i == s.cursor {
					lines = append(lines, sel.Render("> "+label))
				} else {
					lines = append(lines, fg.Render("  "+label))
				}
			}
		}
	}

	if s.err != "" {
		lines = append(lines, red.Render("  "+s.err))
	}
	lines = append(lines, "", fgDim.Render("  t toggle   ↑/↓ select secret   enter confirm"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// LoadCmd fetches every available TOTP secret (standalone + licence-bound).
// The same pool is curated from the manager UI's TOTP tab.
func (s *StepTOTP) LoadCmd() tea.Cmd {
	svc := s.svc
	return func() tea.Msg {
		if svc == nil {
			return TOTPSecretsLoadedMsg{}
		}
		rows, err := svc.TOTP.List(context.Background())
		return TOTPSecretsLoadedMsg{Rows: rows, Err: err}
	}
}

func (s *StepTOTP) Focus()        { s.focused = true }
func (s *StepTOTP) Blur()         { s.focused = false }
func (s *StepTOTP) Focused() bool { return s.focused }
