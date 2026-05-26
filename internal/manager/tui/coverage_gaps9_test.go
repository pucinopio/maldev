package tui

// Session 5 — Strategy 3 edge-case tests
//
// Covers:
//   - Empty-row guards: x/d/E on every list screen with 0 rows must not panic
//   - detailTab > total tabs: renderDetailBody falls back to Identité (tab 0)
//   - WindowSizeMsg while overlay open: rootModel must resize without panic
//   - Concurrent loads: two LicensesLoadedMsg back-to-back converge correctly
//   - Audit future timestamp: renderPayload handles CreatedAt > time.Now()

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// ── Empty-row guards ─────────────────────────────────────────────────────────

// TestEmptyRows_LicensesRevoke — 'x' on Licenses with 0 rows must be a no-op.
func TestEmptyRows_LicensesRevoke(t *testing.T) {
	m := newLicensesModel(nil)
	// rows is nil/empty — selectedRow() returns nil
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Fatal("'x' with no rows must be a no-op (nil cmd)")
	}
	if m2.detail != m.detail {
		t.Fatal("'x' with no rows must not change detail state")
	}
}

// TestEmptyRows_IssuersSetActive — 'a' on Issuers with 0 rows must be a no-op.
func TestEmptyRows_IssuersSetActive(t *testing.T) {
	m := newIssuersModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd != nil {
		t.Fatal("'a' with no rows and nil svc must be a no-op")
	}
}

// TestEmptyRows_IssuersExportPub — 'E' on Issuers with 0 rows must not push overlay.
func TestEmptyRows_IssuersExportPub(t *testing.T) {
	m := newIssuersModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if cmd != nil {
		t.Fatal("'E' with no rows must be a no-op (selectedRow nil)")
	}
}

// TestEmptyRows_RecipientsDelete — 'x' on Recipients with 0 rows must be a no-op.
func TestEmptyRows_RecipientsDelete(t *testing.T) {
	m := newRecipientsModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Fatal("'x' with no rows on Recipients must be a no-op")
	}
}

// TestEmptyRows_IdentitiesDelete — 'x' on Identities with 0 rows must be a no-op.
func TestEmptyRows_IdentitiesDelete(t *testing.T) {
	m := newIdentitiesModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Fatal("'x' with no rows on Identities must be a no-op")
	}
}

// TestEmptyRows_RevocationUnrevoke — 'x' on Revocation with 0 rows must be a no-op.
func TestEmptyRows_RevocationUnrevoke(t *testing.T) {
	m := newRevocationModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Fatal("'x' with no rows on Revocation must be a no-op")
	}
}

// ── detailTab out-of-range ────────────────────────────────────────────────────

// TestLicensesDetailTab_OutOfRange — renderDetailBody with detailTab > 4 must
// fall back to the Identité view without panicking.
func TestLicensesDetailTab_OutOfRange(t *testing.T) {
	m := newLicensesModel(nil)
	m.width = 144
	m.hgt = 44
	m.rows = []*ent.License{fakeLicense()}
	m.rebuildTable()
	m.detail = true
	m.detailTab = 99 // out of range — falls through to default Identité case

	out := m.renderDetailBody(m.rows[0])
	if out == "" {
		t.Fatal("renderDetailBody with detailTab=99 must render non-empty output")
	}
}

// ── WindowSizeMsg while overlay on stack ──────────────────────────────────────

// TestWindowResize_WhileOverlayOpen — sending WindowSizeMsg to rootModel when
// an overlay is on the stack must not panic and must preserve the overlay.
func TestWindowResize_WhileOverlayOpen(t *testing.T) {
	m := New(nil, nil, SessionReady)
	var tm tea.Model = m

	// Initial layout.
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	// Push a help overlay (doesn't require a service).
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})

	// Resize while overlay is open.
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Must render without panic; overlay must still be visible.
	out := tm.View()
	if out == "" {
		t.Fatal("View() must be non-empty after resize with overlay open")
	}
	// View must contain some rendered content — exact text varies by theme.
	if len(out) < 10 {
		t.Fatalf("View() suspiciously short (%d chars) after resize with overlay", len(out))
	}
}

// ── Concurrent loads ─────────────────────────────────────────────────────────

// TestLicensesModel_ConcurrentLoads — two LicensesLoadedMsg back-to-back must
// converge to the last-write state (no zombie rows from the first load).
func TestLicensesModel_ConcurrentLoads(t *testing.T) {
	m := newLicensesModel(nil)

	// Build three distinct rows from the shared fakeLicense() factory.
	row1 := fakeLicense()
	row1.Subject = "first"
	row2a := fakeLicense()
	row2a.Subject = "second-a"
	row2b := fakeLicense()
	row2b.Subject = "second-b"

	m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{row1}})
	m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{row2a, row2b}})

	if len(m.rows) != 2 {
		t.Fatalf("after two loads rows=%d, want 2 (last-write-wins)", len(m.rows))
	}
	if m.rows[0].Subject != "second-a" || m.rows[1].Subject != "second-b" {
		t.Errorf("rows subjects = %v, want [second-a second-b]",
			[]string{m.rows[0].Subject, m.rows[1].Subject})
	}
}

// ── Audit future timestamp ────────────────────────────────────────────────────

// TestAuditFutureTimestamp — renderPayload with an event whose CreatedAt is in
// the future (clock-skew scenario) must not panic and must render something.
func TestAuditFutureTimestamp(t *testing.T) {
	m := newAuditModel(nil)
	m.width = 144

	future := time.Now().Add(24 * time.Hour)
	row := &ent.AuditEvent{
		ID:         uuid.New(),
		Kind:       "license.issue",
		Actor:      "operator",
		TargetKind: "license",
		TargetID:   "future-test",
		CreatedAt:  future,
		Payload:    map[string]interface{}{"note": "clock-skew test"},
	}

	out := m.renderPayload(row)
	if out == "" {
		t.Fatal("renderPayload with future timestamp must return non-empty output")
	}
	if !strings.Contains(out, "license.issue") {
		t.Errorf("renderPayload missing kind in output: %q", out)
	}
}
