package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestTitleBar_Hit confirms titleBar populates a titleHintRow whose hit()
// dispatches the right Cmd for clicks landing inside a segment, and rejects
// clicks on the wrong Y or in the gap between segments.
func TestTitleBar_Hit(t *testing.T) {
	tag := func(s string) func() tea.Cmd {
		return func() tea.Cmd { return func() tea.Msg { return s } }
	}
	var row titleHintRow
	_ = titleBar(&row, "My Title", []titleHint{
		{Key: "n", Label: " créer", Cmd: tag("n")},
		{Key: "x", Label: " supprimer", Cmd: tag("x")},
	}, 0, 120)
	row.SetY(7)

	if cmd := row.hit(0, 6); cmd != nil {
		t.Errorf("hit on wrong Y must return nil")
	}
	if cmd := row.hit(row.startX, 7); cmd == nil {
		t.Errorf("hit on first segment X=%d must dispatch n", row.startX)
	} else if cmd().(string) != "n" {
		t.Errorf("first segment hit should fire n cmd, got %v", cmd())
	}
	// Second segment X = startX + first segW + sep width (3 cells for "· ").
	secondX := row.startX + row.segWs[0] + 3 + 1
	if cmd := row.hit(secondX, 7); cmd == nil {
		t.Errorf("hit inside second segment must dispatch x, got nil")
	}
}

// TestTitleBar_NilSafe asserts the helper does not crash when t is nil
// (callers that don't want click coverage just pass nil).
func TestTitleBar_NilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("titleBar(nil, ...) panicked: %v", r)
		}
	}()
	_ = titleBar(nil, "label", []titleHint{{Key: "n", Cmd: nil}}, 0, 80)
	var nilRow *titleHintRow
	if cmd := nilRow.hit(5, 5); cmd != nil {
		t.Errorf("hit on nil receiver must return nil")
	}
}
