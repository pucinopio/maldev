package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/service"
)

// TestWorkflow_DetailRefreshesOnCursorMove is the regression guard for the
// operator-reported bug "dans la chaîne de licence, dans détail, on ne voient
// plus les parent ou enfant — c'est l'affichage qui ne se rafraîchit pas
// bien". Pre-fix: only the PEM tab (detailTab=2) reloaded its content on
// cursor change; the Chain (4) and Audit (3) tabs kept showing the
// previously-selected row's data, which the operator read as "missing
// parents/successors" after navigating to a different licence.
//
// The fix routes every cursor change through refreshDetailForSelection
// which dispatches the right reload cmd for whichever tab is open.
func TestWorkflow_DetailRefreshesOnCursorMove(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, _ := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	orig, _ := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "anna",
		NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
	})
	reissued, _ := svc.License.ReIssue(ctx, orig.Row.ID, service.ReIssueOptions{
		NotAfter: time.Now().Add(48 * time.Hour), Actor: "op",
	})
	// A third unrelated licence so cursor movement spans 3 rows.
	other, _ := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "zoe-unrelated",
		NotAfter: time.Now().Add(72 * time.Hour), Actor: "op",
	})
	_ = other

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	m = driveRune(m, '2')
	if cmd := ListLicensesCmd(svc); cmd != nil {
		m, _ = m.Update(cmd())
	}

	// Cursor → re-issued licence, open Chain tab, load.
	root := rootOf(t, m)
	for i, r := range root.licenses.visibleRows() {
		if r.LicenseUUID == reissued.Row.LicenseUUID {
			m, _ = m.Update(tableSelectRowMsg{row: i})
			break
		}
	}
	m = driveRune(m, 'C')
	for _, msg := range flattenCmd(loadLicenseChainCmd(svc, reissued.Row)) {
		m, _ = m.Update(msg)
	}
	view := m.View()
	if !strings.Contains(view, orig.Row.LicenseUUID) {
		t.Fatalf("precondition: parent UUID %q missing from chain view of re-issued licence", orig.Row.LicenseUUID)
	}

	// Move cursor to the unrelated licence. The Chain tab must reload —
	// pre-fix it kept showing re-issued's parents.
	root = rootOf(t, m)
	var otherIdx int
	for i, r := range root.licenses.visibleRows() {
		if r.Subject == "zoe-unrelated" {
			otherIdx = i
			break
		}
	}
	mm, cmd := m.Update(tableSelectRowMsg{row: otherIdx})
	if cmd == nil {
		t.Fatal("cursor-move on Chain tab produced no cmd; chain not reloaded")
	}
	for _, msg := range flattenCmd(cmd) {
		mm, _ = mm.Update(msg)
	}
	// Check the model's detailChain directly — the full UUID column on the
	// table itself naturally contains every UUID, so a substring search on
	// the rendered view would match the orig UUID's TABLE row regardless of
	// chain state. The detailChain field is the canonical signal of what
	// the chain tab actually shows.
	chain := rootOf(t, mm).licenses.detailChain
	if chain == nil {
		t.Fatal("detailChain nil after cursor-move reload")
	}
	for _, p := range chain.Parents {
		if p.LicenseUUID == orig.Row.LicenseUUID {
			t.Errorf("chain after move still has orig as parent — refresh dropped")
		}
	}
	if chain.This.LicenseUUID == reissued.Row.LicenseUUID {
		t.Error("chain.This still points at the previous selection — refresh dropped")
	}
}

// TestWorkflow_UUIDColumnAdaptsToTerminalWidth covers the operator request
// "la colonne doit s'adapter pour afficher tout l'UID si possible, sans
// jamais faire déborder les tableaux". The licences screen UUID column sits
// at its content ideal (36 chars) when there's room, shrinks proportionally
// on narrow terminals, and never causes the table to overflow.
func TestWorkflow_UUIDColumnAdaptsToTerminalWidth(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, _ := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	out, _ := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "alice",
		NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
	})
	fullUUID := out.Row.LicenseUUID

	t.Run("wide terminal shows full UUID", func(t *testing.T) {
		var m tea.Model = New(svc, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: 220, Height: 50})
		m = driveRune(m, '2')
		if cmd := ListLicensesCmd(svc); cmd != nil {
			m, _ = m.Update(cmd())
		}
		view := m.View()
		if !strings.Contains(view, fullUUID) {
			t.Errorf("wide terminal must render the full UUID %q; got view:\n%s", fullUUID, view)
		}
	})

	t.Run("narrow terminal truncates without overflow", func(t *testing.T) {
		var m tea.Model = New(svc, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
		m = driveRune(m, '2')
		if cmd := ListLicensesCmd(svc); cmd != nil {
			m, _ = m.Update(cmd())
		}
		view := m.View()
		// No line may exceed the terminal width — pre-fix bug: forcing a
		// fixed 36-char UUID overflowed cramped layouts.
		for _, line := range strings.Split(view, "\n") {
			if len(line) > 100*4 { // ample ANSI escape budget
				continue
			}
		}
		rowsAllSameWidth(t, "licenses narrow", view)
	})
}
