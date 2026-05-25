package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// TestWorkflow_RevokeFromLicensesScreen walks the operator through the full
// revoke flow without a live service: licenses screen → press x → revoke
// overlay appears → type reason → enter → RevokeConfirmedMsg lands in
// app.go → handleRevokeResult is invoked.
//
// Guards the user-reported regression where the revoke popup had display
// issues + 'peu de choses fonctionne'. After this test passes the chain
// from licenses-x to RevokeConfirmedMsg dispatch is end-to-end exercised.
func TestWorkflow_RevokeFromLicensesScreen(t *testing.T) {
	licID := uuid.New()
	root := New(nil, nil, SessionReady)
	root.active = ViewLicenses
	root.licenses.rows = []*ent.License{{
		ID:          licID,
		LicenseUUID: licID.String(),
		Subject:     "alice@research",
	}}
	root.licenses.detail = true

	var m tea.Model = root
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})

	// Press 'x' → licenses screen should push the revoke overlay.
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd == nil {
		t.Fatal("'x' on licenses produced no cmd; revoke overlay should be pushed")
	}
	push, ok := cmd().(pushOverlayMsg)
	if !ok {
		t.Fatalf("'x' cmd produced %T, want pushOverlayMsg", cmd())
	}
	if _, isRevoke := push.overlay.(*revokeOverlay); !isRevoke {
		t.Fatalf("pushed overlay = %T, want *revokeOverlay", push.overlay)
	}

	// Forward the pushOverlayMsg into the root so the overlay stacks.
	m, _ = m.Update(push)
	r := m.(rootModel)
	if len(r.overlays) != 1 {
		t.Fatalf("overlay stack len = %d, want 1", len(r.overlays))
	}

	// The revoke overlay's render must include the licence subject so the
	// operator sees what they're revoking.
	view := m.View()
	if !strings.Contains(view, "alice@research") {
		t.Errorf("revoke overlay view missing licence subject:\n%s", view)
	}

	// Type a reason — the overlay's textinput receives the keys via
	// updateOverlay → overlay.Update → fall-through to textinput.
	for _, r := range "compromised" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press enter — overlay emits OverlayDoneMsg{Result: RevokeConfirmedMsg}.
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on revoke overlay produced no cmd")
	}
	done, ok := cmd().(OverlayDoneMsg)
	if !ok {
		t.Fatalf("enter cmd produced %T, want OverlayDoneMsg", cmd())
	}
	res, ok := done.Result.(RevokeConfirmedMsg)
	if !ok {
		t.Fatalf("OverlayDoneMsg.Result = %T, want RevokeConfirmedMsg", done.Result)
	}
	if res.LicenseID != licID {
		t.Errorf("LicenseID = %v, want %v", res.LicenseID, licID)
	}
	if res.Reason != "compromised" {
		t.Errorf("Reason = %q, want compromised", res.Reason)
	}

	// Feed the OverlayDoneMsg back through the root — overlay should be
	// popped + the licenses screen's handleRevokeResult should run (no svc
	// wired so the resulting cmd is nil; the test just confirms no panic).
	m, _ = m.Update(done)
	r = m.(rootModel)
	if len(r.overlays) != 0 {
		t.Errorf("overlay stack should be empty after Done, got %d", len(r.overlays))
	}
}

// TestWorkflow_RevokeChipClickPopulatesInput is the click-mouse variant
// of the above: instead of typing the reason, the operator clicks a
// suggestion chip → the input gets the chip's reason → enter confirms.
func TestWorkflow_RevokeChipClickPopulatesInput(t *testing.T) {
	o := newRevokeOverlay(uuid.New(), "test")
	_ = o.View() // populates chipRects + footerY

	if len(o.chipRects) == 0 {
		t.Fatal("chipRects empty")
	}
	// Pick a known chip (the first one — "key_compromised") and click its centre.
	chip := o.chipRects[0]
	clickX := (chip.x1 + chip.x2) / 2
	_, _ = o.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: clickX, Y: chip.y})
	if o.input.Value() != chip.reason {
		t.Errorf("after chip click: input = %q, want %q", o.input.Value(), chip.reason)
	}

	// Now click the Révoquer button (right side of footer).
	_, cmd := o.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 40, Y: o.footerY})
	done := cmd().(OverlayDoneMsg)
	res, ok := done.Result.(RevokeConfirmedMsg)
	if !ok {
		t.Fatalf("button click cmd produced %T, want OverlayDoneMsg{RevokeConfirmedMsg}", done.Result)
	}
	if res.Reason != chip.reason {
		t.Errorf("Reason = %q, want %q (chip's reason)", res.Reason, chip.reason)
	}
}
