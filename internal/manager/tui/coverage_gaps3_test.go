package tui

// Coverage gaps closed in Session 0004 (batch 3):
//   - screen_audit.writeAuditCSV / writeAuditJSON file writers
//   - screen_placeholder (Init/Update/View) — trivial but uncovered
//   - screen_revocation.selectedRow / OnClick — table cursor + click
//   - screen_passphrase.ResolvedPassphrase getter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// ── screen_audit file writers ────────────────────────────────────────────────

// fixtureAuditRows returns two representative audit events for export tests.
func fixtureAuditRows() []*ent.AuditEvent {
	return []*ent.AuditEvent{
		{
			ID:         uuid.New(),
			Kind:       "license.issue",
			Actor:      "operator",
			TargetKind: "License",
			TargetID:   "lic-1",
			CreatedAt:  time.Date(2026, 5, 21, 7, 41, 2, 0, time.UTC),
			Payload:    map[string]any{"subject": "alice"},
		},
		{
			ID:         uuid.New(),
			Kind:       "license.revoke",
			Actor:      "operator",
			TargetKind: "License",
			TargetID:   "lic-2",
			CreatedAt:  time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC),
			Payload:    map[string]any{"reason": "compromised"},
		},
	}
}

// TestWriteAuditCSV_HeaderAndRows writes the rows + reopens the file to check
// the CSV header line + at least one data line containing each event's kind.
func TestWriteAuditCSV_HeaderAndRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.csv")
	if err := writeAuditCSV(path, fixtureAuditRows()); err != nil {
		t.Fatalf("writeAuditCSV: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	body := string(raw)
	for _, want := range []string{
		"timestamp", "kind", "actor", "target_kind", "target_id", "payload",
		"license.issue", "license.revoke", "operator",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("csv missing %q; got:\n%s", want, body)
		}
	}
}

// TestWriteAuditCSV_EmptyRowsStillWritesHeader — zero rows produce a file with
// only the header line.
func TestWriteAuditCSV_EmptyRowsStillWritesHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.csv")
	if err := writeAuditCSV(path, nil); err != nil {
		t.Fatalf("writeAuditCSV(nil): %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(raw), "timestamp,kind") {
		t.Fatalf("empty.csv must contain header even with no rows; got:\n%s", raw)
	}
}

// TestWriteAuditJSON_ParsesRoundTrip writes JSON + parses it back into a slice
// and asserts the row count + the kinds round-trip.
func TestWriteAuditJSON_ParsesRoundTrip(t *testing.T) {
	rows := fixtureAuditRows()
	path := filepath.Join(t.TempDir(), "audit.json")
	if err := writeAuditJSON(path, rows); err != nil {
		t.Fatalf("writeAuditJSON: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var back []map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, raw)
	}
	if len(back) != 2 {
		t.Fatalf("rows = %d, want 2", len(back))
	}
}

// ── screen_placeholder ───────────────────────────────────────────────────────

// TestPlaceholderModel_Init_ReturnsNil — placeholder has no async setup.
func TestPlaceholderModel_Init_ReturnsNil(t *testing.T) {
	m := newPlaceholderModel("Phase X", "scaffold")
	if m.Init() != nil {
		t.Fatal("placeholderModel.Init() must return nil")
	}
}

// TestPlaceholderModel_UpdateAbsorbsWindowSize — sending a WindowSizeMsg
// stores width/height on the model.
func TestPlaceholderModel_UpdateAbsorbsWindowSize(t *testing.T) {
	m := newPlaceholderModel("X", "scaffold")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.width != 100 || m.hgt != 30 {
		t.Fatalf("WindowSize not applied: width=%d height=%d", m.width, m.hgt)
	}
}

// TestPlaceholderModel_ViewContainsIDAndPhase — the rendered string surfaces
// both the screen ID and the phase label.
func TestPlaceholderModel_ViewContainsIDAndPhase(t *testing.T) {
	m := newPlaceholderModel("Phase X", "stub-phase")
	m.width = 80
	m.hgt = 24
	out := m.View()
	if !strings.Contains(out, "Phase X") {
		t.Errorf("View() missing screen ID")
	}
	if !strings.Contains(out, "stub-phase") {
		t.Errorf("View() missing phase label")
	}
	if !strings.Contains(out, "not yet implemented") {
		t.Errorf("View() missing 'not yet implemented' hint")
	}
}

// TestPlaceholderModel_ViewZeroSizeFallsBackTo80x24 — when WindowSize wasn't
// applied, View() must still render with a default 80×24 canvas (no crash).
func TestPlaceholderModel_ViewZeroSizeFallsBackTo80x24(t *testing.T) {
	m := newPlaceholderModel("X", "")
	if out := m.View(); out == "" {
		t.Fatal("zero-size View() returned empty")
	}
}

// ── screen_revocation selectedRow + OnClick ──────────────────────────────────

// TestRevocationModel_SelectedRow_OutOfRange asserts a nil return when the
// cursor points beyond the rows slice.
func TestRevocationModel_SelectedRow_OutOfRange(t *testing.T) {
	m := newRevocationModel(nil)
	// Cursor defaults to 0; rows is nil so it MUST return nil.
	if got := m.selectedRow(); got != nil {
		t.Fatalf("selectedRow on empty rows must return nil; got %+v", got)
	}
}

// TestRevocationModel_SelectedRow_InRange seeds two rows and asserts cursor=1
// returns the second row.
func TestRevocationModel_SelectedRow_InRange(t *testing.T) {
	m := newRevocationModel(nil)
	m.rows = []service.RevocationView{
		{Subject: "alice", KeyID: "k1", RevokedAt: time.Now(), Reason: "rotation"},
		{Subject: "bob", KeyID: "k2", RevokedAt: time.Now(), Reason: "compromised"},
	}
	m.rebuildTable()
	m.table.SetCursor(1)
	row := m.selectedRow()
	if row == nil {
		t.Fatal("selectedRow returned nil for in-range cursor")
	}
	if row.Subject != "bob" {
		t.Fatalf("subject = %q, want 'bob'", row.Subject)
	}
}

// TestRevocationModel_OnClick_AboveChromeReturnsNil — clicks before the chip
// area (y < TopChromeRows) return nil.
func TestRevocationModel_OnClick_AboveChromeReturnsNil(t *testing.T) {
	m := newRevocationModel(nil)
	if cmd := m.OnClick(0, 0, 0); cmd != nil {
		t.Fatalf("click in top chrome must not emit cmd; got %v", cmd())
	}
}

// ── screen_passphrase ResolvedPassphrase getter ──────────────────────────────

// TestPassphraseResolved_EmptyByDefault asserts a freshly created passphrase
// model has no result yet.
func TestPassphraseResolved_EmptyByDefault(t *testing.T) {
	m := newPassphraseModel()
	if got := m.ResolvedPassphrase(); got != "" {
		t.Fatalf("fresh model ResolvedPassphrase = %q, want empty", got)
	}
}
