package tui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/service"
)

// TestWorkflow_DeleteIssuerOrphan drives the issuers screen [D] path end-to-
// end against a real service. The seeded issuer has no licences, so Delete
// succeeds and the row disappears from the table.
func TestWorkflow_DeleteIssuerOrphan(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, err := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = driveRune(m, '3') // Issuers tab
	if cmd := listIssuersCmd(svc); cmd != nil {
		m, _ = m.Update(cmd())
	}
	if len(rootOf(t, m).issuers.rows) != 1 {
		t.Fatalf("issuers preload mismatch")
	}

	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	push, ok := cmd().(pushOverlayMsg)
	if !ok {
		t.Fatalf("'D' on issuers did not push overlay; got %T", cmd())
	}
	confirm, ok := push.overlay.(*confirmOverlay)
	if !ok || confirm.id != OverlayIDIssuerDelete {
		t.Fatalf("wrong overlay: %T id=%q", push.overlay, confirm.id)
	}
	mm, _ = mm.Update(push)
	mm, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	done := cmd().(OverlayDoneMsg)
	mm, cmd = mm.Update(done)
	for _, msg := range flattenCmd(cmd) {
		mm, _ = mm.Update(msg)
	}
	if _, err := svc.Issuer.Get(ctx, iss.ID); err == nil {
		t.Fatal("issuer row still present after delete workflow")
	}
}

// TestWorkflow_DeleteIssuerRefusedWhenLicencesExist exercises the service
// guard via the TUI: pressing [D] on an issuer with licences must surface a
// readable error overlay and leave the row intact.
func TestWorkflow_DeleteIssuerRefusedWhenLicencesExist(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, _ := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	if _, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "x",
		NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
	}); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = driveRune(m, '3')
	if cmd := listIssuersCmd(svc); cmd != nil {
		m, _ = m.Update(cmd())
	}
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	push := cmd().(pushOverlayMsg)
	mm, _ = mm.Update(push)
	mm, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	done := cmd().(OverlayDoneMsg)
	mm, cmd = mm.Update(done)
	// Drain — the service refusal returns a pushOverlayMsg{*errorOverlay}.
	for _, msg := range flattenCmd(cmd) {
		mm, _ = mm.Update(msg)
	}
	if _, err := svc.Issuer.Get(ctx, iss.ID); err != nil {
		t.Fatal("issuer was deleted despite licences referencing it")
	}
}

// TestWorkflow_DeleteRevokedFromRevocationScreen exercises the new [D] action
// on the dedicated Revocation view — direct hard-delete of the underlying
// licence, bypassing the unrevoke path.
func TestWorkflow_DeleteRevokedFromRevocationScreen(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, _ := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	issued, _ := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "rev-target",
		NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
	})
	if err := svc.Revoke.Revoke(ctx, issued.Row.ID, "cleanup", "op"); err != nil {
		t.Fatal(err)
	}

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = driveRune(m, '6') // Revocation tab
	if cmd := listRevocationCmd(svc); cmd != nil {
		m, _ = m.Update(cmd())
	}
	if got := len(rootOf(t, m).revocation.rows); got != 1 {
		t.Fatalf("revocation rows preload = %d, want 1", got)
	}
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	push, ok := cmd().(pushOverlayMsg)
	if !ok {
		t.Fatalf("'D' on revocation did not push overlay; got %T", cmd())
	}
	confirm, ok := push.overlay.(*confirmOverlay)
	if !ok || confirm.id != OverlayIDRevocationDelete {
		t.Fatalf("wrong overlay: %T id=%q", push.overlay, confirm.id)
	}
	mm, _ = mm.Update(push)
	mm, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	done := cmd().(OverlayDoneMsg)
	mm, cmd = mm.Update(done)
	for _, msg := range flattenCmd(cmd) {
		mm, _ = mm.Update(msg)
	}
	if _, err := svc.License.GetByUUID(ctx, issued.Row.LicenseUUID); err == nil {
		t.Fatal("licence row still present after revocation-screen [D]")
	}
}

// TestWorkflow_DeleteTOTPViaD proves [D] on the TOTP screen aliases the
// existing [x] flow — both paths call svc.TOTP.Delete via the same overlay.
func TestWorkflow_DeleteTOTPViaD(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	row, _, err := svc.TOTP.Generate(ctx, "demo@app")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = driveRune(m, '8') // TOTP tab

	// Manually inject the loaded row — listTOTPCmd path is exercised elsewhere
	// and the screen owns the rows slice. Use the public message type when
	// available; otherwise re-fetch via the service.
	rows, _ := svc.TOTP.List(ctx)
	root := rootOf(t, m)
	root.totp.rows = rows
	m = root

	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	push, ok := cmd().(pushOverlayMsg)
	if !ok {
		t.Fatalf("'D' on TOTP did not push overlay; got %T", cmd())
	}
	confirm, ok := push.overlay.(*confirmOverlay)
	if !ok || confirm.id != "totp-delete" {
		t.Fatalf("wrong overlay: %T id=%q (want totp-delete)", push.overlay, confirm.id)
	}
	mm, _ = mm.Update(push)
	mm, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	done := cmd().(OverlayDoneMsg)
	mm, cmd = mm.Update(done)
	for _, msg := range flattenCmd(cmd) {
		mm, _ = mm.Update(msg)
	}

	remaining, _ := svc.TOTP.List(ctx)
	for _, r := range remaining {
		if r.ID == row.ID {
			t.Fatal("TOTP row still present after [D] workflow")
		}
	}
}
