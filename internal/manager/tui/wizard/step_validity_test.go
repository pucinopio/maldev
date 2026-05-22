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

func TestStepValidityShortcutCtrlF(t *testing.T) {
	s := NewStepValidity()
	s.Focus()
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if got := s.endIn.Value(); !strings.EqualFold(got, "forever") {
		t.Fatalf("after ctrl+f, endIn = %q, want forever", got)
	}
}
