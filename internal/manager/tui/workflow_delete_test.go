package tui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/service"
)

// TestWorkflow_DeleteFromLicensesScreen drives the full end-to-end delete flow
// against a real *service.Services + in-memory store: issue a licence, press
// 'D' on the licences screen, confirm the danger overlay, observe the row
// disappear from the DB, and finally re-import the same PEM to prove the
// unique license_uuid was freed.
//
// Pre-feature gap: there was no delete path — operators could only Revoke,
// which left the row in place and made the PEM impossible to re-import
// (UNIQUE constraint on license_uuid). This E2E guards every layer of the
// new path: key dispatch [D], confirm overlay round-trip, app.go
// ConfirmResultMsg routing, handleLicenseDeleteConfirm cmd, licenseDeletedMsg
// → rebuildTable, and the post-delete re-import.
func TestWorkflow_DeleteFromLicensesScreen(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()

	iss, err := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	if err != nil {
		t.Fatalf("Issuer.Generate: %v", err)
	}
	issued, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "alice@research",
		NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
	})
	if err != nil {
		t.Fatalf("License.Issue: %v", err)
	}
	licUUID := issued.Row.LicenseUUID
	pem := issued.PEM

	// Boot TUI with live services, size window, switch to licences tab, then
	// load the real row via ListLicensesCmd just like Init() would.
	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = driveRune(m, '2')
	if cmd := ListLicensesCmd(svc); cmd != nil {
		m, _ = m.Update(cmd())
	}

	root := rootOf(t, m)
	if got := len(root.licenses.rows); got != 1 {
		t.Fatalf("licences screen rows = %d, want 1 after seed", got)
	}

	// Press 'D' — should emit a pushOverlayMsg{*confirmOverlay} with the
	// OverlayIDLicenseDelete identifier.
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if cmd == nil {
		t.Fatal("'D' on licences produced no cmd; expected delete-confirm overlay push")
	}
	push, ok := cmd().(pushOverlayMsg)
	if !ok {
		t.Fatalf("'D' cmd produced %T, want pushOverlayMsg", cmd())
	}
	confirm, ok := push.overlay.(*confirmOverlay)
	if !ok {
		t.Fatalf("pushed overlay = %T, want *confirmOverlay", push.overlay)
	}
	if confirm.id != OverlayIDLicenseDelete {
		t.Errorf("confirm overlay id = %q, want %q", confirm.id, OverlayIDLicenseDelete)
	}
	if !confirm.danger {
		t.Error("delete confirm should be marked danger=true (red styling)")
	}

	// Stack the overlay, then press 'y' to confirm. Overlay emits
	// OverlayDoneMsg{ConfirmResultMsg{Confirm:true}}; rootModel routes it
	// through dispatchOverlayResult → handleLicenseDeleteConfirm, draining
	// pendingCmd into the returned tea.Cmd.
	mm, _ = mm.Update(push)
	mm, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatal("'y' on confirm overlay produced no cmd")
	}
	done, ok := cmd().(OverlayDoneMsg)
	if !ok {
		t.Fatalf("'y' cmd produced %T, want OverlayDoneMsg", cmd())
	}

	// Feed the OverlayDoneMsg back — this pops the overlay AND batches the
	// pendingCmd (which actually performs svc.License.Delete + List).
	mm, cmd = mm.Update(done)
	if cmd == nil {
		t.Fatal("rootModel did not return a follow-up cmd after delete-confirm; pendingCmd was dropped")
	}
	// Drain the batched cmd. It may produce a tea.BatchMsg of multiple sub-cmds
	// — execute every leaf to surface the licenseDeletedMsg.
	for _, msg := range flattenCmd(cmd) {
		mm, _ = mm.Update(msg)
	}

	// Sanity at the DB layer: the row must be gone and re-import must succeed.
	if _, err := svc.License.GetByUUID(ctx, licUUID); err == nil {
		t.Fatal("DB row still present after delete workflow")
	}
	if _, err := svc.License.Import(ctx, pem, "reimport", "op"); err != nil {
		t.Fatalf("re-import after delete failed: %v", err)
	}

	// The TUI table should now also be empty (before re-import) or carry the
	// re-imported row. We reload via the same cmd the UI would issue.
	if cmd := ListLicensesCmd(svc); cmd != nil {
		mm, _ = mm.Update(cmd())
	}
	root = rootOf(t, mm)
	if got := len(root.licenses.rows); got != 1 {
		t.Fatalf("after delete + reimport, rows = %d, want 1", got)
	}
	if root.licenses.rows[0].LicenseUUID != licUUID {
		t.Fatalf("post-reimport UUID = %q, want %q (PEM should be reusable)",
			root.licenses.rows[0].LicenseUUID, licUUID)
	}
}

// flattenCmd executes cmd, unwrapping any tea.BatchMsg into a flat slice of
// concrete messages so the test can feed each one back through Update.
// Mirrors what the bubbletea runtime does between ticks. Sibling of the
// one-shot drainCmd helper in interactions_live_test.go (which keeps a
// model-threaded form for that file's pattern).
func flattenCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, sub := range batch {
			out = append(out, flattenCmd(sub)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// TestWorkflow_DeleteRevokedLicence is the regression guard for the operator
// report "on ne peut pas supprimer une licence révoquée". A revoked row is
// reachable under filter=all OR filter=revoked, and [D] must succeed on it.
func TestWorkflow_DeleteRevokedLicence(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()

	iss, _ := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	issued, _ := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "mallory",
		NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
	})
	if err := svc.Revoke.Revoke(ctx, issued.Row.ID, "key compromise", "op"); err != nil {
		t.Fatalf("Revoke seed: %v", err)
	}
	licUUID := issued.Row.LicenseUUID

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = driveRune(m, '2')
	if cmd := ListLicensesCmd(svc); cmd != nil {
		m, _ = m.Update(cmd())
	}

	// Cycle filter [f] until we land on "revoked" — proves the revoked row is
	// addressable through the filter chip too, not just under "all".
	for i := 0; i < 10; i++ {
		root := rootOf(t, m)
		if root.licenses.filter.String() == "revoked" {
			break
		}
		m = driveRune(m, 'f')
	}
	root := rootOf(t, m)
	if root.licenses.filter.String() != "revoked" {
		t.Fatal("could not cycle filter to revoked")
	}
	if len(root.licenses.visibleRows()) != 1 {
		t.Fatalf("revoked filter visible rows = %d, want 1", len(root.licenses.visibleRows()))
	}

	// Drive [D] → 'y' → drain → assert deletion.
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	push := cmd().(pushOverlayMsg)
	mm, _ = mm.Update(push)
	mm, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	done := cmd().(OverlayDoneMsg)
	mm, cmd = mm.Update(done)
	for _, msg := range flattenCmd(cmd) {
		mm, _ = mm.Update(msg)
	}

	if _, err := svc.License.GetByUUID(ctx, licUUID); err == nil {
		t.Fatal("revoked row still present after [D] workflow")
	}
}
