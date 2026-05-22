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

func TestStepFreeFieldsEnterAfterTypingThenEnterSubmits(t *testing.T) {
	s := NewStepFreeFields()
	s.Focus()
	for _, r := range "k" {
		_, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// First Enter blurs the input
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		if _, ok := cmd().(FreeFieldsMsg); ok {
			t.Fatal("first Enter with typed value should NOT submit immediately")
		}
	}
	// Second Enter (input now blurred) submits
	_, cmd = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("second Enter should submit")
	}
	if _, ok := cmd().(FreeFieldsMsg); !ok {
		t.Fatal("second Enter should emit FreeFieldsMsg")
	}
}
