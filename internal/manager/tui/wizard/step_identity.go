// Package wizard contains the individual step widgets for the New License Wizard.
// Each step implements tui/core.Focusable so the parent wizard can route focus.
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

// IdentityLoadedMsg is sent when the identity list has been fetched.
type IdentityLoadedMsg struct {
	Rows []*ent.Issuer
	Err  error
}

// IdentityChosenMsg is emitted when the operator has chosen or created an issuer.
type IdentityChosenMsg struct {
	IssuerID string // UUID string of selected issuer
}

// StepIdentity is step 1 of the wizard: pick or create a signing issuer.
type StepIdentity struct {
	svc     *service.Services
	rows    []*ent.Issuer
	cursor  int
	mode    identityMode // browse | create
	nameIn  textinput.Model
	keyIDIn textinput.Model
	focused bool
	bounds  core.Rect
	err     string
}

type identityMode int

const (
	identityBrowse identityMode = iota
	identityCreate
)

// NewStepIdentity constructs the identity step.
func NewStepIdentity(svc *service.Services) *StepIdentity {
	ni := textinput.New()
	ni.Placeholder = "issuer name (e.g. prod-2026)"
	ni.CharLimit = 80

	ki := textinput.New()
	ki.Placeholder = "key-id (e.g. maldev-prod-01)"
	ki.CharLimit = 64

	return &StepIdentity{svc: svc, nameIn: ni, keyIDIn: ki}
}

// --- Widget interface --------------------------------------------------------

func (s *StepIdentity) Layout(b core.Rect) { s.bounds = b }
func (s *StepIdentity) Bounds() core.Rect  { return s.bounds }

func (s *StepIdentity) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case IdentityLoadedMsg:
		s.err = ""
		if msg.Err != nil {
			s.err = msg.Err.Error()
		}
		s.rows = msg.Rows
		// Pre-select the active issuer so pressing Enter immediately
		// signs the licence with the operator's intended key. Falls back
		// to the first row when no row is active (e.g. fresh install).
		s.cursor = 0
		for i, r := range s.rows {
			if r.Active {
				s.cursor = i
				break
			}
		}
		return s, nil

	case tea.KeyMsg:
		if !s.focused {
			return s, nil
		}
		switch s.mode {
		case identityBrowse:
			return s, s.handleBrowseKey(msg)
		case identityCreate:
			// Only intercept the navigation keys; everything else falls through
			// to the textinput forwarder below so the operator can actually
			// type into the name + keyID fields.
			switch msg.String() {
			case "tab", "enter", "esc":
				return s, s.handleCreateKey(msg)
			}
		}
	}
	// Forward to active text-input in create mode.
	if s.mode == identityCreate {
		var cmds []tea.Cmd
		if s.nameIn.Focused() {
			updated, cmd := s.nameIn.Update(msg)
			s.nameIn = updated
			cmds = append(cmds, cmd)
		} else if s.keyIDIn.Focused() {
			updated, cmd := s.keyIDIn.Update(msg)
			s.keyIDIn = updated
			cmds = append(cmds, cmd)
		}
		return s, tea.Batch(cmds...)
	}
	return s, nil
}

// OnClick handles row clicks in browse mode: clicking an issuer row selects
// it, clicking the "Create new issuer" sentinel switches to create mode.
// In create mode clicks are ignored (let the operator type).
// body-local coords: header takes Y=0..2, rows start at Y=3.
func (s *StepIdentity) OnClick(_, y int) tea.Cmd {
	if s.mode == identityCreate {
		return nil
	}
	const headerH = 3
	idx := y - headerH
	if idx < 0 {
		return nil
	}
	if idx < len(s.rows) {
		s.cursor = idx
		id := s.rows[idx].ID.String()
		return func() tea.Msg { return IdentityChosenMsg{IssuerID: id} }
	}
	if idx == len(s.rows) {
		s.cursor = len(s.rows)
		s.mode = identityCreate
		s.nameIn.Focus()
		return textinput.Blink
	}
	return nil
}

func (s *StepIdentity) handleBrowseKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		// +1 for the "create new" sentinel at the bottom
		if s.cursor < len(s.rows) {
			s.cursor++
		}
	case "enter":
		if s.cursor == len(s.rows) {
			// "create new" selected
			s.mode = identityCreate
			s.nameIn.Focus()
			return textinput.Blink
		}
		if s.cursor < len(s.rows) {
			id := s.rows[s.cursor].ID.String()
			return func() tea.Msg { return IdentityChosenMsg{IssuerID: id} }
		}
	case "n":
		s.mode = identityCreate
		s.nameIn.Focus()
		return textinput.Blink
	}
	return nil
}

// handleCreateKey returns a non-nil Cmd only for actions that move the wizard
// forward; text-input forwarding is handled separately in Update so the key
// is NOT double-dispatched.
func (s *StepIdentity) handleCreateKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "tab":
		if s.nameIn.Focused() {
			s.nameIn.Blur()
			s.keyIDIn.Focus()
		} else {
			s.keyIDIn.Blur()
			s.nameIn.Focus()
		}
		return textinput.Blink
	case "enter":
		if s.nameIn.Focused() {
			s.nameIn.Blur()
			s.keyIDIn.Focus()
			return textinput.Blink
		}
		name := s.nameIn.Value()
		keyID := s.keyIDIn.Value()
		if name == "" || keyID == "" {
			s.err = "name and key-id are required"
			return nil
		}
		svc := s.svc
		return func() tea.Msg {
			// Idempotent create: revisiting this step after Back/Next would
			// otherwise hit a duplicate-key error on the second Generate call.
			existing, _ := svc.Issuer.List(context.Background())
			for _, e := range existing {
				if e.KeyID == keyID || e.Name == name {
					return IdentityChosenMsg{IssuerID: e.ID.String()}
				}
			}
			row, err := svc.Issuer.Generate(context.Background(), name, keyID, "operator")
			if err != nil {
				return IdentityLoadedMsg{Err: err}
			}
			return IdentityChosenMsg{IssuerID: row.ID.String()}
		}
	case "esc":
		s.mode = identityBrowse
		s.nameIn.Blur()
		s.keyIDIn.Blur()
		s.nameIn.SetValue("")
		s.keyIDIn.SetValue("")
		s.err = ""
	}
	return nil
}

func (s *StepIdentity) View() string {
	w := s.bounds.W
	if w < 20 {
		w = 20
	}

	header := stepHeader("Step 1 — Signing Identity",
		"Pick an existing issuer or create a new Ed25519 signing key.")

	if s.mode == identityCreate {
		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			s.renderCreateForm(w),
		)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		s.renderList(w),
	)
}

func (s *StepIdentity) renderList(_ int) string {
	var lines []string
	for i, r := range s.rows {
		// Active-issuer marker — ASCII ">>" identical to the issuers
		// screen so the operator can identify the in-use signing key at
		// a glance. Without this the wizard's identity list looked
		// identical for active and retired issuers, hiding which key
		// would actually sign the new licence.
		actMark := "    "
		if r.Active {
			actMark = ">>  "
		}
		label := fmt.Sprintf("%s%-28s  %s", actMark, r.Name, r.KeyID)
		if i == s.cursor {
			lines = append(lines, wizSel.Render("> "+label))
		} else {
			lines = append(lines, wizFg.Render("  "+label))
		}
	}
	createLabel := "  + Create new issuer"
	if s.cursor == len(s.rows) {
		lines = append(lines, wizSel.Render(">"+createLabel))
	} else {
		lines = append(lines, wizDim.Render(" "+createLabel))
	}

	if len(lines) == 0 {
		lines = []string{wizDim.Render("  (no issuers yet)")}
	}

	list := lipgloss.JoinVertical(lipgloss.Left, lines...)
	hints := wizDim.Render("\n  ↑/↓ navigate   enter select   n create new")
	if s.err != "" {
		return lipgloss.JoinVertical(lipgloss.Left,
			list,
			wizRed.Render("  error: "+s.err),
			hints,
		)
	}
	return lipgloss.JoinVertical(lipgloss.Left, list, hints)
}

func (s *StepIdentity) renderCreateForm(_ int) string {
	lines := []string{
		wizDim.Render("  Name:"),
		"  " + s.nameIn.View(),
		"",
		wizDim.Render("  Key-ID:"),
		"  " + s.keyIDIn.View(),
	}
	if s.err != "" {
		lines = append(lines, wizRed.Render("  "+s.err))
	}
	lines = append(lines, "", wizDim.Render("  tab next field   enter confirm   esc cancel"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// Focus / Blur / Focused implement core.Focusable.
func (s *StepIdentity) Focus()        { s.focused = true }
func (s *StepIdentity) Blur()         { s.focused = false }
func (s *StepIdentity) Focused() bool { return s.focused }

// LoadCmd returns the tea.Cmd that fetches issuers.
func (s *StepIdentity) LoadCmd() tea.Cmd {
	svc := s.svc
	return func() tea.Msg {
		if svc == nil {
			return IdentityLoadedMsg{}
		}
		rows, err := svc.Issuer.List(context.Background())
		return IdentityLoadedMsg{Rows: rows, Err: err}
	}
}
