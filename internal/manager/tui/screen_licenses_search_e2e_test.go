package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// ═══════════════════════════════════════════════════════════════════════════
// LICENSES SEARCH INPUT — E2E STATE-MACHINE TEST
// ═══════════════════════════════════════════════════════════════════════════
//
// Covers the search lifecycle that pre-existing tests left implicit:
//   '/'                 → focus search
//   type chars          → narrow visible rows
//   Esc                 → blur search, KEEP last query (so the operator can
//                          re-edit) — verified by re-focusing and reading
//                          back the input value
//   '/'+Backspace clears query → visible rows return to full set
//
// This is the kind of flow VHS tapes record but Go E2E must guard against
// regression at every commit.
// ═══════════════════════════════════════════════════════════════════════════

func TestLicensesSearchE2E_FocusTypeBlurFilter(t *testing.T) {
	rows := []*ent.License{
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "alice@research.example",
			Status: "active", NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)},
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "bob@research.example",
			Status: "active", NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)},
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "carol@prod.example",
			Status: "active", NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)},
	}

	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = drive(m, '2')
	m, _ = m.Update(LicensesLoadedMsg{Rows: rows})

	// Step 1: '/' focuses search.
	m = drive(m, '/')
	if r := rootOf(t, m); !r.licenses.search.Focused() {
		t.Fatalf("after '/': search must be focused")
	}

	// Step 2: type "bob" — visible rows shrink to just bob.
	for _, r := range "bob" {
		m = drive(m, r)
	}
	if r := rootOf(t, m); r.licenses.search.Value() != "bob" {
		t.Errorf("search.Value()=%q want \"bob\"", r.licenses.search.Value())
	}
	lic := rootOf(t, m).licenses
	visible := (&lic).visibleRows()
	if len(visible) != 1 || !strings.Contains(visible[0].Subject, "bob") {
		t.Errorf("after typing 'bob': visible=%d rows (want 1 containing 'bob')", len(visible))
	}

	// Step 3: Esc blurs but keeps the query.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if r := rootOf(t, m); r.licenses.search.Focused() {
		t.Errorf("after Esc: search still focused")
	}
	if r := rootOf(t, m); r.licenses.search.Value() != "bob" {
		t.Errorf("after Esc: search.Value()=%q want \"bob\" preserved", r.licenses.search.Value())
	}
	lic = rootOf(t, m).licenses
	visible = (&lic).visibleRows()
	if len(visible) != 1 {
		t.Errorf("after Esc with active filter: visible=%d want 1 (filter must persist)", len(visible))
	}

	// Step 4: '/' re-focuses; backspace ×3 clears.
	m = drive(m, '/')
	for i := 0; i < 3; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	if r := rootOf(t, m); r.licenses.search.Value() != "" {
		t.Errorf("after 3× backspace: search.Value()=%q want empty", r.licenses.search.Value())
	}
	lic = rootOf(t, m).licenses
	visible = (&lic).visibleRows()
	if len(visible) != 3 {
		t.Errorf("empty query: visible=%d want 3 (all rows)", len(visible))
	}

	// Step 5: Enter blurs search (alternative exit, same persistence).
	m = drive(m, 'a')
	m = drive(m, 'l')
	m = drive(m, 'i')
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if r := rootOf(t, m); r.licenses.search.Focused() {
		t.Errorf("after Enter: search still focused")
	}
	lic = rootOf(t, m).licenses
	visible = (&lic).visibleRows()
	if len(visible) != 1 || !strings.Contains(visible[0].Subject, "ali") {
		t.Errorf("after typing 'ali'+Enter: visible=%d want 1 (alice match)", len(visible))
	}
}
