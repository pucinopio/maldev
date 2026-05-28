package wizard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStepFreeFieldsEnterOnEmptySubmits(t *testing.T) {
	s := NewStepFreeFields()
	s.Focus()
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on empty should return a cmd that emits FreeFieldsMsg")
	}
	msg := cmd()
	if _, ok := msg.(FreeFieldsMsg); !ok {
		t.Fatalf("Enter cmd returned %T, want FreeFieldsMsg", msg)
	}
}

func TestStepFreeFieldsCtrlSSubmits(t *testing.T) {
	s := NewStepFreeFields()
	s.Focus()
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("ctrl+s should submit")
	}
	if _, ok := cmd().(FreeFieldsMsg); !ok {
		t.Fatal("ctrl+s should emit FreeFieldsMsg")
	}
}

// TestStepFreeFieldsEnterAlwaysSubmits — operator-reported uniformity
// requirement "la touche entrée sert à passer à l'étape suivante sauf
// pour une étape — uniformise". Enter on FreeFields now always submits
// (advances to the next step) regardless of whether an input is focused
// or has content. Tab/⇧Tab still cycle between fields inside the step.
func TestStepFreeFieldsEnterAlwaysSubmits(t *testing.T) {
	s := NewStepFreeFields()
	s.Focus()
	for _, r := range "key1" {
		_, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Enter on a focused, non-empty input must submit on the FIRST press —
	// pre-fix this required two Enter presses (advance field, then submit).
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on typed value must submit, got nil cmd")
	}
	if _, ok := cmd().(FreeFieldsMsg); !ok {
		t.Fatalf("Enter cmd produced %T, want FreeFieldsMsg", cmd())
	}
}
