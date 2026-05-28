package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// TestWorkflow_IssuerActiveMarkerVisibleInRenderedView is the strict guard
// the operator asked for: the active-issuer marker must appear in the
// ACTUAL rendered View() string the terminal would print, not just in the
// m.table.Rows() data array. Earlier iterations passed the rows-level test
// but the terminal failed to render the Unicode glyph, so the operator saw
// nothing distinguishing the active row.
//
// Two scenarios are checked:
//  1. Active issuer at row 0 (default cursor) — the most common state when
//     opening the issuers screen. The bubbles/table Selected style applies
//     here and historically masked colour-based markers.
//  2. Active issuer at row 1 (cursor on a non-active row) — verifies the
//     marker is visible even when the active row is NOT the highlighted one.
func TestWorkflow_IssuerActiveMarkerVisibleInRenderedView(t *testing.T) {
	t.Run("active row at cursor", func(t *testing.T) {
		assertActiveMarkerVisible(t, 0)
	})
	t.Run("active row away from cursor", func(t *testing.T) {
		assertActiveMarkerVisible(t, 1)
	})
}

func assertActiveMarkerVisible(t *testing.T, activeIdx int) {
	t.Helper()
	rows := make([]*ent.Issuer, 2)
	for i := range rows {
		rows[i] = fakeIssuer()
		rows[i].Active = (i == activeIdx)
		rows[i].Name = []string{"lab", "old"}[i]
	}

	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = driveRune(m, '3') // Issuers tab
	m, _ = m.Update(IssuersLoadedMsg{Rows: rows})

	view := m.View()

	// The active marker must appear at least once in the rendered output.
	if !strings.Contains(view, ">>") {
		t.Fatalf("active marker '>>' missing from rendered view (activeIdx=%d)\n---VIEW---\n%s\n---END---",
			activeIdx, view)
	}

	// Locate the line containing the active issuer's name and assert the
	// marker is on THAT line (not just somewhere else in the chrome).
	lines := strings.Split(view, "\n")
	activeName := rows[activeIdx].Name
	var activeLine string
	for _, line := range lines {
		if strings.Contains(line, activeName) && strings.Contains(line, "active") {
			activeLine = line
			break
		}
	}
	if activeLine == "" {
		t.Fatalf("could not find the active issuer's row in rendered view\n---VIEW---\n%s",
			view)
	}
	if !strings.Contains(activeLine, ">>") {
		t.Errorf("active row missing marker '>>':\nline: %q", activeLine)
	}

	// The OTHER row must NOT carry the marker.
	otherName := rows[1-activeIdx].Name
	for _, line := range lines {
		if strings.Contains(line, otherName) && strings.Contains(line, "inactive") {
			if strings.Contains(line, ">>") {
				t.Errorf("inactive row %q contains marker '>>': %q", otherName, line)
			}
			break
		}
	}
}

// TestWorkflow_IssuerMarkerIsPureASCII guards against future Unicode-glyph
// regressions. The marker must stay ASCII-only so it renders on every
// terminal/font combination operators run the manager in.
func TestWorkflow_IssuerMarkerIsPureASCII(t *testing.T) {
	active := fakeIssuer()
	active.Active = true
	m := newIssuersModel(nil)
	m.rows = []*ent.Issuer{active}
	m.rebuildTable()

	cell := m.table.Rows()[0][0]
	for _, r := range cell {
		if r > 0x7E { // ASCII printable range ends at 0x7E (~)
			t.Errorf("active marker cell %q contains non-ASCII rune %U", cell, r)
		}
	}
}
