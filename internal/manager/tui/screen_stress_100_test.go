package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// TestLicenses_Stress100Rows feeds the licenses model 100 rows and asserts the
// table renders cleanly: viewable height stays bounded, every row is reachable
// via cursor moves, and rebuildTable does not panic on the volume. Pre-2026-05
// no test exercised more than a handful of rows so any width/height overflow on
// realistic datasets slipped through.
func TestLicenses_Stress100Rows(t *testing.T) {
	rows := make([]*ent.License, 100)
	now := time.Now()
	for i := range rows {
		rows[i] = &ent.License{
			ID:          uuid.New(),
			LicenseUUID: uuid.NewString(),
			Subject:     "subject-" + strings.Repeat("x", i%17),
			IssuerName:  "issuer-prod",
			Audience:    []string{"aud"},
			Features:    []string{"f1", "f2"},
			Status:      "active",
			NotBefore:   now.Add(-24 * time.Hour),
			NotAfter:    now.Add(365 * 24 * time.Hour),
			Pem:         []byte("-----BEGIN LICENSE-----\nfake\n-----END LICENSE-----\n"),
		}
	}

	m := newLicensesModel(nil)
	m.width = 144
	m.hgt = 44
	m.rows = rows
	m.rebuildTable()

	if got := len(m.table.Rows()); got != 100 {
		t.Fatalf("table rows = %d, want 100", got)
	}
	// Cursor must be able to land on the last row without panic.
	m.table.SetCursor(99)
	if got := m.table.Cursor(); got != 99 {
		t.Fatalf("cursor = %d, want 99", got)
	}
	// View must produce non-empty output with the count reflected in the title.
	out := m.View()
	if !strings.Contains(out, "Licences (100)") {
		t.Errorf("view missing 'Licences (100)' title:\n%s", out)
	}
	// Visible height of the rendered view must fit within the screen budget;
	// table.SetHeight caps the table to (hgt - chrome - detail), so the whole
	// view should never exceed the terminal height by more than the detail box.
	if h := lipgloss.Height(out); h > m.hgt*3 {
		t.Errorf("rendered view height = %d on hgt=%d, blowing past 3× budget", h, m.hgt)
	}
}

// TestIssuers_Stress100Rows mirrors the licenses stress check for the issuers
// table.
func TestIssuers_Stress100Rows(t *testing.T) {
	rows := make([]*ent.Issuer, 100)
	now := time.Now()
	for i := range rows {
		rows[i] = &ent.Issuer{
			ID:        uuid.New(),
			Name:      "issuer-" + strings.Repeat("z", i%13),
			KeyID:     "key-" + uuid.NewString()[:8],
			Active:    i == 0,
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		}
	}
	m := newIssuersModel(nil)
	m.width = 144
	m.hgt = 44
	m.rows = rows
	m.rebuildTable()

	if got := len(m.table.Rows()); got != 100 {
		t.Fatalf("table rows = %d, want 100", got)
	}
	m.table.SetCursor(99)
	if got := m.table.Cursor(); got != 99 {
		t.Fatalf("cursor = %d, want 99", got)
	}
	out := m.View()
	if !strings.Contains(out, "Issuer keys Ed25519 (100)") {
		t.Errorf("view missing 'Issuer keys Ed25519 (100)' title:\n%s", out)
	}
}

// TestLicenses_StressViaUpdateMsg checks the WindowSizeMsg → LicensesLoadedMsg
// path under load (the same sequence the real app drives at startup).
func TestLicenses_StressViaUpdateMsg(t *testing.T) {
	m := newLicensesModel(nil)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	rows := make([]*ent.License, 100)
	now := time.Now()
	for i := range rows {
		rows[i] = &ent.License{
			ID:          uuid.New(),
			LicenseUUID: uuid.NewString(),
			Subject:     "subject",
			IssuerName:  "issuer",
			Status:      "active",
			NotBefore:   now,
			NotAfter:    now.Add(365 * 24 * time.Hour),
		}
	}
	mm, _ = mm.Update(LicensesLoadedMsg{Rows: rows})
	if len(mm.table.Rows()) != 100 {
		t.Fatalf("after LicensesLoadedMsg: rows=%d want 100", len(mm.table.Rows()))
	}
}
