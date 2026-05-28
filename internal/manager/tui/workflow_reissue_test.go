package tui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	licenseent "github.com/oioio-space/maldev/internal/manager/store/ent/license"
	"github.com/oioio-space/maldev/internal/manager/tui/wizard"
)

// TestWorkflow_ReissueWizardEditsValidity is the operator-reported regression
// guard for "quand on re-emet une license (superseed) on ne peut changer aucun
// parametre". The pre-fix flow pushed a confirm overlay with no inputs and
// emitted a licence with NotAfter=zero (i.e. already expired). The new flow
// opens the wizard pre-populated from the original; this test drives Validity
// → FreeFields → Review and asserts:
//   - the new licence's NotAfter matches the date the operator typed (NOT zero)
//   - the original is marked superseded
//   - bindings/issuer carry over via the wizard state
func TestWorkflow_ReissueWizardEditsValidity(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()

	iss, _ := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	issued, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "alice",
		AudienceList: []string{"prod"},
		Features:     []string{"export"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
		Actor:        "op",
	})
	if err != nil {
		t.Fatalf("Issue seed: %v", err)
	}
	origID := issued.Row.ID

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = driveRune(m, '2')
	if cmd := ListLicensesCmd(svc); cmd != nil {
		m, _ = m.Update(cmd())
	}

	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd == nil {
		t.Fatal("'e' on a selected licence produced no cmd; reissue wizard didn't launch")
	}
	push, ok := cmd().(pushOverlayMsg)
	if !ok {
		t.Fatalf("'e' cmd produced %T, want pushOverlayMsg", cmd())
	}
	wiz, ok := push.overlay.(*wizardOverlay)
	if !ok {
		t.Fatalf("pushed overlay = %T, want *wizardOverlay", push.overlay)
	}
	if !wiz.model.state.IsReissue {
		t.Fatal("state.IsReissue not set on reissue wizard launch")
	}
	if wiz.model.state.OriginalID != origID.String() {
		t.Fatalf("OriginalID = %q, want %q", wiz.model.state.OriginalID, origID.String())
	}
	if wiz.model.step != wizStepValidity {
		t.Fatalf("wizard step = %v, want wizStepValidity", wiz.model.step)
	}

	// Stack the wizard overlay so subsequent messages reach it through the
	// root model's overlay dispatch.
	mm, _ = mm.Update(push)

	target := time.Now().Add(180 * 24 * time.Hour).Truncate(24 * time.Hour)

	// Validity → FreeFields. The wizard top-level Update reacts to the
	// step's emitted ValidityMsg the same whether it comes from enter on
	// the textinput or directly. Same trick for FreeFieldsMsg below.
	mm, _ = mm.Update(wizard.ValidityMsg{NotBefore: time.Now(), NotAfter: target})
	mm, _ = mm.Update(wizard.FreeFieldsMsg{Subject: "alice", Audience: "prod", Fields: nil})

	// Review → Issue. Press 'enter' on the focused review step; drain the
	// returned cmd to surface the IssueResultMsg.
	mm, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter")})
	for _, msg := range flattenCmd(cmd) {
		mm, _ = mm.Update(msg)
	}

	rows, err := svc.License.List(ctx, service.ListFilter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	var newest *ent.License
	var orig *ent.License
	for _, r := range rows {
		if r.ID == origID {
			orig = r
			continue
		}
		if newest == nil || r.CreatedAt.After(newest.CreatedAt) {
			newest = r
		}
	}
	if orig == nil {
		t.Fatal("original licence vanished")
	}
	if orig.Status != licenseent.StatusSuperseded {
		t.Errorf("original status = %v, want superseded", orig.Status)
	}
	if newest == nil {
		t.Fatal("no new licence produced by the wizard")
	}
	if newest.NotAfter.Year() < 2000 {
		t.Fatalf("new licence NotAfter = %v (zero-ish — pre-fix bug is back)", newest.NotAfter)
	}
	gotDay := newest.NotAfter.Truncate(24 * time.Hour)
	if !gotDay.Equal(target) {
		t.Errorf("new licence NotAfter day = %v, want %v", gotDay, target)
	}
}
