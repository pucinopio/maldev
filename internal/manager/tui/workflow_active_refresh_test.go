package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
	"github.com/oioio-space/maldev/internal/manager/tui/wizard"
)

// TestWorkflow_DashboardActiveKeyRefreshesAfterActivate is the regression
// guard for the operator-reported bug "le issuer actif ne se met pas à jour
// dans le dashboard". Root cause: DashboardSnapshotMsg was routed through
// routeReady to the currently-active screen (ViewIssuers when [a] was
// pressed) which silently dropped it. The dashboard tile only refreshed
// when the operator navigated back to the dashboard tab and triggered a
// fresh snapshot manually.
//
// The fix routes DashboardSnapshotMsg directly to the dashboard model in
// the top-level message switch, regardless of m.active.
func TestWorkflow_DashboardActiveKeyRefreshesAfterActivate(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss1, _ := svc.Issuer.Generate(ctx, "first", "k-1", "op")
	iss2, _ := svc.Issuer.Generate(ctx, "second", "k-2", "op")
	if err := svc.Issuer.SetActive(ctx, iss1.ID, "op"); err != nil {
		t.Fatal(err)
	}

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	for _, msg := range flattenCmd(m.Init()) {
		m, _ = m.Update(msg)
	}
	if name := rootOf(t, m).dashboard.activeKey.name; name != "first" {
		t.Fatalf("initial dashboard active key = %q, want 'first'", name)
	}

	// Operator navigates to issuers and activates iss2 (simulating the
	// service call that [a] would trigger). The screen emits an
	// IssuersLoadedMsg back to the root; the rooted dispatcher must batch
	// a dashboard refresh whose snapshot lands on the dashboard model
	// EVEN WHILE the operator is still on the issuers tab.
	m = driveRune(m, '3')
	if err := svc.Issuer.SetActive(ctx, iss2.ID, "op"); err != nil {
		t.Fatal(err)
	}
	rows, _ := svc.Issuer.List(ctx)
	mm, cmd := m.Update(IssuersLoadedMsg{Rows: rows})
	for _, msg := range flattenCmd(cmd) {
		mm, _ = mm.Update(msg)
	}

	got := rootOf(t, mm).dashboard.activeKey.name
	if got != "second" {
		t.Errorf("dashboard active key after activate = %q, want 'second'", got)
	}
}

// TestWorkflow_WizardIdentityStepMarksActiveIssuer is the regression guard
// for the second half of the operator complaint — "ni dans le workflow de
// génération de licence". Before fix: StepIdentity rendered every issuer
// row identically with no visual signal for which one would actually sign
// the licence. Operators had no way to tell unless they remembered which
// key they'd activated last on the issuers screen.
func TestWorkflow_WizardIdentityStepMarksActiveIssuer(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss1, _ := svc.Issuer.Generate(ctx, "alpha", "k-alpha", "op")
	iss2, _ := svc.Issuer.Generate(ctx, "beta", "k-beta", "op")
	_ = iss1
	if err := svc.Issuer.SetActive(ctx, iss2.ID, "op"); err != nil {
		t.Fatal(err)
	}
	rows, _ := svc.Issuer.List(ctx)

	step := wizard.NewStepIdentity(svc)
	step.Focus()
	step.Layout(core.Rect{W: 80, H: 30})
	step.Update(wizard.IdentityLoadedMsg{Rows: rows})

	view := step.View()
	// The active issuer's row must carry the ">>" marker AND the cursor
	// "> " prefix so pressing Enter immediately signs with the active key.
	// The previous fix only added the marker; cursor stayed at row 0
	// regardless of which row was active, defeating the convenience.
	for _, line := range strings.Split(view, "\n") {
		switch {
		case strings.Contains(line, "alpha"):
			if strings.Contains(line, ">>") {
				t.Errorf("inactive issuer 'alpha' row carries '>>' marker: %q", line)
			}
			if strings.Contains(line, "> ") && !strings.Contains(line, ">> ") {
				t.Errorf("inactive issuer 'alpha' row carries the cursor: %q", line)
			}
		case strings.Contains(line, "beta"):
			if !strings.Contains(line, ">>") {
				t.Errorf("active issuer 'beta' row missing '>>' marker: %q", line)
			}
			if !strings.Contains(line, "> ") {
				t.Errorf("active issuer 'beta' row missing cursor '> ': %q", line)
			}
		}
	}
}

// TestWorkflow_WizardIdentityStepPresselectsActive — the operator pressed
// Enter on the freshly-opened wizard and expected the active issuer to
// sign the licence without first having to ↓ to it. Pre-fix: cursor was
// always at row 0, so Enter chose whichever issuer happened to be first
// in the list (often a retired one). Now the cursor lands on the active
// issuer at load time.
func TestWorkflow_WizardIdentityStepPresselectsActive(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss1, _ := svc.Issuer.Generate(ctx, "alpha", "k-alpha", "op")
	iss2, _ := svc.Issuer.Generate(ctx, "beta", "k-beta", "op")
	_ = iss1
	if err := svc.Issuer.SetActive(ctx, iss2.ID, "op"); err != nil {
		t.Fatal(err)
	}
	rows, _ := svc.Issuer.List(ctx)

	step := wizard.NewStepIdentity(svc)
	step.Focus()
	step.Layout(core.Rect{W: 80, H: 30})
	step.Update(wizard.IdentityLoadedMsg{Rows: rows})

	// Press Enter — the chosen IdentityChosenMsg must carry iss2's ID.
	_, cmd := step.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on identity step produced no cmd")
	}
	chosen, ok := cmd().(wizard.IdentityChosenMsg)
	if !ok {
		t.Fatalf("cmd produced %T, want IdentityChosenMsg", cmd())
	}
	if chosen.IssuerID != iss2.ID.String() {
		t.Errorf("Enter chose %q, want active issuer %q (iss2 beta)",
			chosen.IssuerID, iss2.ID.String())
	}
}
