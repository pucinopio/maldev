package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/service"
)

// TestWorkflow_LicensesTablesShowUUIDColumn — operator wanted a UUID column
// in every table that lists licences. Both the Licences screen and the
// Revocation screen render the short UUID form (12 chars + ellipsis) so
// rows can be cross-referenced with audit logs and the chain detail.
func TestWorkflow_LicensesTablesShowUUIDColumn(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, _ := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	out, _ := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "alice",
		NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
	})
	_ = svc.Revoke.Revoke(ctx, out.Row.ID, "test", "op")

	wantPrefix := out.Row.LicenseUUID[:12]

	t.Run("licences screen", func(t *testing.T) {
		var m tea.Model = New(svc, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
		m = driveRune(m, '2')
		if cmd := ListLicensesCmd(svc); cmd != nil {
			m, _ = m.Update(cmd())
		}
		view := m.View()
		if !strings.Contains(view, "UUID") {
			t.Error("licences screen missing UUID column header")
		}
		if !strings.Contains(view, wantPrefix) {
			t.Errorf("licences screen missing UUID prefix %q in rendered view", wantPrefix)
		}
	})

	t.Run("revocation screen", func(t *testing.T) {
		var m tea.Model = New(svc, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
		m = driveRune(m, '6')
		if cmd := listRevocationCmd(svc); cmd != nil {
			m, _ = m.Update(cmd())
		}
		view := m.View()
		if !strings.Contains(view, "UUID") {
			t.Error("revocation screen missing UUID column header")
		}
		if !strings.Contains(view, wantPrefix) {
			t.Errorf("revocation screen missing UUID prefix %q in rendered view", wantPrefix)
		}
	})
}

// TestWorkflow_ChainShowsFullUUIDs — operator complaint "on ne voit pas
// l'UID complet d'une licence" in the chain detail. The chain renderer
// now embeds the full 36-char UUID for every parent/this/successor entry
// so the operator can read it without truncation.
func TestWorkflow_ChainShowsFullUUIDs(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, _ := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	orig, _ := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "carol",
		NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
	})
	reissued, _ := svc.License.ReIssue(ctx, orig.Row.ID, service.ReIssueOptions{
		NotAfter: time.Now().Add(48 * time.Hour), Actor: "op",
	})

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	m = driveRune(m, '2')
	if cmd := ListLicensesCmd(svc); cmd != nil {
		m, _ = m.Update(cmd())
	}
	// Move to the re-issued licence row and open the chain tab.
	root := rootOf(t, m)
	for i, r := range root.licenses.visibleRows() {
		if r.LicenseUUID == reissued.Row.LicenseUUID {
			m, _ = m.Update(tableSelectRowMsg{row: i})
			break
		}
	}
	m = driveRune(m, 'C')
	if cmd := loadLicenseChainCmd(svc, reissued.Row); cmd != nil {
		m, _ = m.Update(cmd())
	}

	view := m.View()
	// Both UUIDs must appear in full (36 chars each).
	if !strings.Contains(view, orig.Row.LicenseUUID) {
		t.Errorf("parent UUID %q missing from chain view", orig.Row.LicenseUUID)
	}
	if !strings.Contains(view, reissued.Row.LicenseUUID) {
		t.Errorf("this licence UUID %q missing from chain view", reissued.Row.LicenseUUID)
	}
}

// TestWorkflow_ChainClickNavigatesToLicence — operator complaint "si l'on
// clic dessus, ça nous emmène sur cette licence". A click on a parent or
// successor chain entry must move the cursor to that licence, force
// filter→all if it was hidden, and reload the chain tab for the new row.
func TestWorkflow_ChainClickNavigatesToLicence(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, _ := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	orig, _ := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "parent-row",
		NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
	})
	reissued, _ := svc.License.ReIssue(ctx, orig.Row.ID, service.ReIssueOptions{
		NotAfter: time.Now().Add(48 * time.Hour), Actor: "op",
	})

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	m = driveRune(m, '2')
	if cmd := ListLicensesCmd(svc); cmd != nil {
		m, _ = m.Update(cmd())
	}
	root := rootOf(t, m)
	for i, r := range root.licenses.visibleRows() {
		if r.LicenseUUID == reissued.Row.LicenseUUID {
			m, _ = m.Update(tableSelectRowMsg{row: i})
			break
		}
	}
	m = driveRune(m, 'C')
	if cmd := loadLicenseChainCmd(svc, reissued.Row); cmd != nil {
		m, _ = m.Update(cmd())
	}
	// Force the View pipeline to populate chainHits.
	_ = m.View()

	root = rootOf(t, m)
	if root.licenses.chainHits == nil || len(root.licenses.chainHits.hits) == 0 {
		t.Fatal("chainHits not populated after rendering Chain tab")
	}
	// Dispatch the click message directly — equivalent to OnClick hitting
	// the parent row. Verifies the Update handler navigates correctly.
	m, _ = m.Update(licenseChainClickMsg{uuid: orig.Row.LicenseUUID})
	root = rootOf(t, m)
	selected := root.licenses.selectedRow()
	if selected == nil {
		t.Fatal("no row selected after chain click")
	}
	if selected.LicenseUUID != orig.Row.LicenseUUID {
		t.Errorf("after chain click cursor on %q, want %q",
			selected.LicenseUUID, orig.Row.LicenseUUID)
	}
	if root.licenses.detailTab != 4 {
		t.Errorf("after chain click detailTab=%d, want 4 (Chain)", root.licenses.detailTab)
	}
}
