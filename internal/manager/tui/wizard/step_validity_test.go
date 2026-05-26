package wizard

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStepValidityShortcutCtrlW(t *testing.T) {
	s := NewStepValidity()
	s.Focus()
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	want := time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02")
	if got := s.endIn.Value(); got != want {
		t.Fatalf("after ctrl+w, endIn = %q, want %q", got, want)
	}
}

// TestStepValidityShortcutCtrlD guards D-S20: ctrl+d applies +30d shortcut
// (was ctrl+m which is identical to Enter in bubbletea's keyNames mapping).
func TestStepValidityShortcutCtrlD(t *testing.T) {
	s := NewStepValidity()
	s.Focus()
	before := s.startIn.Value()
	// Parse start so we can predict expected end.
	start, _ := time.ParseInLocation("2006-01-02", before, time.Local)
	want := start.Add(30 * 24 * time.Hour).Format("2006-01-02")

	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if got := s.endIn.Value(); got != want {
		t.Fatalf("after ctrl+d, endIn = %q, want %q", got, want)
	}
}

func TestStepValidityShortcutCtrlF(t *testing.T) {
	s := NewStepValidity()
	s.Focus()
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if got := s.endIn.Value(); !strings.EqualFold(got, "forever") {
		t.Fatalf("after ctrl+f, endIn = %q, want forever", got)
	}
}
