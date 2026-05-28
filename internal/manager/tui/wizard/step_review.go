package wizard

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// IssueResultMsg is emitted after a successful LicenseService.Issue call.
type IssueResultMsg struct {
	Issued *service.IssuedLicense
	Err    error
}

// ErrCancelled is the sentinel returned by step_review's "cancel" branch.
// The wizard root checks against it via errors.Is rather than comparing
// err.Error() to a magic string.
var ErrCancelled = fmt.Errorf("cancelled")

// WizardState holds all collected choices across the 8 steps.
type WizardState struct {
	IssuerID    string
	RecipientID string // empty = no sealed payload
	MachineID   string // empty = skip
	BinarySHA256 string
	BinarySize  int64
	NotBefore   time.Time
	NotAfter    time.Time
	Subject     string // empty = default ("licence")
	Audience    string // comma-separated tags
	FreeFields  map[string]string
	RequireTOTP bool
	TOTPSecretID string // UUID string; empty when RequireTOTP false

	// IsReissue marks the wizard run as a re-issue of an existing licence:
	// identity/recipient/machine/binary/totp steps are pre-populated and
	// skipped, and the Review step calls svc.License.ReIssue instead of
	// Issue. OriginalID points at the licence being superseded.
	IsReissue  bool
	OriginalID string // UUID of the original licence; empty when IsReissue=false
	// Features carries the licence's feature list. The new-licence flow
	// derives features from FreeFields' "features" key; re-issue inherits
	// them straight from the original licence here so they survive the
	// wizard round-trip.
	Features []string
	// PayloadCleartext holds an optional payload to embed unencrypted. The
	// new-licence wizard doesn't currently expose a UI for it; re-issue
	// inherits the original's payload through this field.
	PayloadCleartext []byte
}

// StepReview is step 8: summary + Issue button.
type StepReview struct {
	svc     *service.Services
	state   WizardState
	issuing bool
	focused bool
	bounds  core.Rect

	// issueBtnY / cancelBtnY are body-local Y rows of the two action buttons,
	// recorded by View() so OnClick can hit-test without re-parsing the
	// rendered string.
	issueBtnY  int
	cancelBtnY int
}

// NewStepReview constructs step 8.
func NewStepReview(svc *service.Services) *StepReview {
	return &StepReview{svc: svc}
}

// SetState replaces the accumulated wizard state shown in the summary.
func (s *StepReview) SetState(ws WizardState) { s.state = ws }

func (s *StepReview) Layout(b core.Rect) { s.bounds = b }
func (s *StepReview) Bounds() core.Rect  { return s.bounds }

func (s *StepReview) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case IssueResultMsg:
		s.issuing = false
		return s, nil

	case tea.KeyMsg:
		if !s.focused || s.issuing {
			return s, nil
		}
		switch msg.String() {
		case "enter", "i":
			s.issuing = true
			return s, s.issueCmd()
		case "esc":
			// Cancel — let wizard handle.
			return s, func() tea.Msg { return IssueResultMsg{Err: ErrCancelled} }
		}
	}
	return s, nil
}

func (s *StepReview) issueCmd() tea.Cmd {
	svc := s.svc
	st := s.state
	return func() tea.Msg {
		if svc == nil {
			return IssueResultMsg{Err: fmt.Errorf("services unavailable")}
		}
		if st.IsReissue {
			origID, err := parseUUID(st.OriginalID)
			if err != nil {
				return IssueResultMsg{Err: fmt.Errorf("invalid original id: %w", err)}
			}
			opts := service.ReIssueOptions{
				NotBefore: st.NotBefore,
				NotAfter:  st.NotAfter,
				Actor:     "operator",
				Payload:   st.PayloadCleartext,
			}
			// Audience: csv → []string. Empty input means "clear the audience"
			// during a re-issue (operator intent), so we explicitly assign an
			// empty (non-nil) slice instead of letting the service inherit
			// the original. That keeps the wizard's edit semantics consistent
			// with what the operator sees on screen.
			opts.Audience = []string{}
			for _, tag := range strings.Split(st.Audience, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					opts.Audience = append(opts.Audience, tag)
				}
			}
			// Features: prefer the original's slice (already inherited via
			// state) when the operator did not add any key=value rows that
			// look like features. Free-fields rows are encoded the same way
			// as in the new-licence path (key=value) — when present they
			// REPLACE the original feature list so the operator can prune.
			if len(st.FreeFields) > 0 {
				opts.Features = []string{}
				for k, v := range st.FreeFields {
					opts.Features = append(opts.Features, k+"="+v)
				}
			} else {
				opts.Features = st.Features
			}
			issued, err := svc.License.ReIssue(context.Background(), origID, opts)
			return IssueResultMsg{Issued: issued, Err: err}
		}
		req, err := buildIssueRequest(st)
		if err != nil {
			return IssueResultMsg{Err: err}
		}
		issued, err := svc.License.Issue(context.Background(), req)
		return IssueResultMsg{Issued: issued, Err: err}
	}
}

// buildIssueRequest converts WizardState into service.IssueRequest.
func buildIssueRequest(st WizardState) (service.IssueRequest, error) {
	if st.IssuerID == "" {
		return service.IssueRequest{}, fmt.Errorf("issuer is required")
	}

	issuerUUID, err := parseUUID(st.IssuerID)
	if err != nil {
		return service.IssueRequest{}, fmt.Errorf("invalid issuer id: %w", err)
	}

	subject := strings.TrimSpace(st.Subject)
	if subject == "" {
		subject = "licence"
	}
	req := service.IssueRequest{
		IssuerID:     issuerUUID,
		Subject:      subject,
		NotBefore:    st.NotBefore,
		NotAfter:     st.NotAfter,
		BinarySHA256: st.BinarySHA256,
		Actor:        "operator",
	}
	for _, tag := range strings.Split(st.Audience, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			req.AudienceList = append(req.AudienceList, tag)
		}
	}

	if st.FreeFields != nil {
		// Encode free fields as Features list (key=value) — the schema stores
		// Features as []string; a dedicated Payload field could be used instead
		// for richer structured data.
		for k, v := range st.FreeFields {
			req.Features = append(req.Features, k+"="+v)
		}
	}

	if st.MachineID != "" {
		req.Bindings = append(req.Bindings, service.BindingSpec{
			Type:   "machine",
			Values: []string{st.MachineID},
		})
	}

	if st.RequireTOTP {
		req.Bindings = append(req.Bindings, service.BindingSpec{Type: "totp"})
	}

	if st.RecipientID != "" {
		recipID, err := parseUUID(st.RecipientID)
		if err != nil {
			return service.IssueRequest{}, fmt.Errorf("invalid recipient id: %w", err)
		}
		req.SealedFor = &recipID
	}

	return req, nil
}

func (s *StepReview) View() string {
	header := stepHeader("Step 8 — Review & Issue",
		"Confirm all choices and press enter to sign the licence.")

	row := func(label, value string) string {
		return wizFg.Render(fmt.Sprintf("  %-20s", label)) + wizDim.Render(value)
	}

	notAfterStr := s.state.NotAfter.Format("2006-01-02")
	if s.state.NotAfter.Year() >= 9999 {
		notAfterStr = "forever"
	}

	totpStr := "no"
	if s.state.RequireTOTP {
		totpStr = "yes (secret: " + s.state.TOTPSecretID + ")"
	}

	freeStr := "(none)"
	if len(s.state.FreeFields) > 0 {
		pairs := make([]string, 0, len(s.state.FreeFields))
		for k, v := range s.state.FreeFields {
			pairs = append(pairs, k+"="+v)
		}
		freeStr = strings.Join(pairs, ", ")
	}

	subjectStr := s.state.Subject
	if subjectStr == "" {
		subjectStr = "licence (default)"
	}
	lines := []string{
		header, "",
		row("Subject:", subjectStr),
		row("Audience:", orDash(s.state.Audience)),
		row("Issuer ID:", s.state.IssuerID),
		row("Recipient ID:", orDash(s.state.RecipientID)),
		row("Machine ID:", orDash(s.state.MachineID)),
		row("Binary SHA-256:", orDash(s.state.BinarySHA256)),
		row("Not before:", s.state.NotBefore.Format("2006-01-02")),
		row("Not after:", notAfterStr),
		row("Free fields:", freeStr),
		row("Require TOTP:", totpStr),
		"",
	}

	// Record button Y in RENDERED rows, not slice indices — lines[0] is the
	// 3-line stepHeader and counts as 3 visible rows once JoinVertical
	// expands embedded \n. Walk current lines and accumulate display height
	// so OnClick(_, y) matches the actual row.
	renderedY := 0
	for _, l := range lines {
		renderedY += lipgloss.Height(l)
	}
	if s.issuing {
		s.issueBtnY = renderedY
		s.cancelBtnY = -1
		lines = append(lines, wizDim.Render("  issuing…"))
	} else {
		s.issueBtnY = renderedY
		s.cancelBtnY = s.issueBtnY + 1
		lines = append(lines,
			wizGreen.Render("  [ enter / i ]  Issue licence"),
			wizDim.Render("  [ esc ]        Cancel"),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// OnClick handles button clicks on step 8. body-local coords; ignores X
// since the buttons span the full width and any click on the row counts.
func (s *StepReview) OnClick(_, y int) tea.Cmd {
	if s.issuing {
		return nil
	}
	switch y {
	case s.issueBtnY:
		s.issuing = true
		return s.issueCmd()
	case s.cancelBtnY:
		return func() tea.Msg { return IssueResultMsg{Err: ErrCancelled} }
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func (s *StepReview) Focus()        { s.focused = true }
func (s *StepReview) Blur()         { s.focused = false }
func (s *StepReview) Focused() bool { return s.focused }
