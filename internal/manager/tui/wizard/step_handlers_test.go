package wizard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// TestStepBindingBinary_Update_OnEscSkips — esc/ctrl+s emits an empty
// BinaryBindingMsg (skip path). The step gate keys behind `s.focused`
// so we Focus() it first (mirrors what the wizard root does on
// initStep before dispatching keys).
func TestStepBindingBinary_Update_OnEscSkips(t *testing.T) {
	for _, key := range []string{"esc", "ctrl+s"} {
		s := NewStepBindingBinary()
		s.Layout(core.Rect{W: 80, H: 20})
		s.Focus()
		_, cmd := s.Update(tea.KeyMsg{Type: keyForString(key)})
		if cmd == nil {
			t.Errorf("%s: nil cmd, want BinaryBindingMsg{}", key)
			continue
		}
		if _, ok := cmd().(BinaryBindingMsg); !ok {
			t.Errorf("%s: cmd produced %T, want BinaryBindingMsg", key, cmd())
		}
	}
}

// TestStepBindingBinary_FocusBlur cycles focus state.
func TestStepBindingBinary_FocusBlur(t *testing.T) {
	s := NewStepBindingBinary()
	if s.Focused() {
		t.Error("fresh step should not be focused")
	}
	s.Focus()
	if !s.Focused() {
		t.Error("Focus() did not set Focused")
	}
	s.Blur()
	if s.Focused() {
		t.Error("Blur() did not clear Focused")
	}
}

// TestStepBindingBinary_SetPath populates the text input.
func TestStepBindingBinary_SetPath(t *testing.T) {
	s := NewStepBindingBinary()
	s.SetPath("/tmp/binary.bin")
	// The path is stored in the model's pathIn textinput — re-render and
	// confirm the path shows up in the view body.
	s.Layout(core.Rect{W: 80, H: 20})
	out := s.View()
	if !contains(out, "/tmp/binary.bin") {
		t.Errorf("View after SetPath missing path:\n%s", out)
	}
}

// TestStepBindingMachine_FocusBlur cycles focus state. Focus() sets the
// flag; FocusCmd() is a separate hook the wizard calls after Focus to
// kick off the probed-machine loader, so we verify both surfaces.
func TestStepBindingMachine_FocusBlur(t *testing.T) {
	s := NewStepBindingMachine(nil)
	if s.Focused() {
		t.Error("fresh step should not be focused")
	}
	s.Focus()
	if !s.Focused() {
		t.Error("Focus() did not set Focused")
	}
	if cmd := s.FocusCmd(); cmd == nil {
		t.Error("FocusCmd() returned nil; expected the probed-machine loader cmd")
	}
	s.Blur()
	if s.Focused() {
		t.Error("Blur() did not clear Focused")
	}
}

// TestStepValidity_Update_TabCyclesField — tab should swap the active
// field between start and end inputs.
func TestStepValidity_Update_TabCyclesField(t *testing.T) {
	s := NewStepValidity()
	s.Focus()
	s.Layout(core.Rect{W: 80, H: 20})
	if s.active != validityFieldStart {
		t.Fatalf("fresh step: active = %v, want validityFieldStart", s.active)
	}
	s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if s.active != validityFieldEnd {
		t.Errorf("after tab: active = %v, want validityFieldEnd", s.active)
	}
	s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if s.active != validityFieldStart {
		t.Errorf("after second tab: active = %v, want validityFieldStart", s.active)
	}
}

// TestStepValidity_FocusBlur cycles focus state.
func TestStepValidity_FocusBlur(t *testing.T) {
	s := NewStepValidity()
	if s.Focused() {
		t.Error("fresh step should not be focused")
	}
	s.Focus()
	if !s.Focused() {
		t.Error("Focus() did not set Focused")
	}
	s.Blur()
	if s.Focused() {
		t.Error("Blur() did not clear Focused")
	}
}

// TestStepReview_FocusBlur cycles focus state.
func TestStepReview_FocusBlur(t *testing.T) {
	s := NewStepReview(nil)
	if s.Focused() {
		t.Error("fresh step should not be focused")
	}
	s.Focus()
	if !s.Focused() {
		t.Error("Focus() did not set Focused")
	}
	s.Blur()
	if s.Focused() {
		t.Error("Blur() did not clear Focused")
	}
}

// TestStepReview_SetState replaces the displayed summary.
func TestStepReview_SetState(t *testing.T) {
	s := NewStepReview(nil)
	s.SetState(WizardState{Subject: "alice@research", Audience: "team-a"})
	s.Layout(core.Rect{W: 80, H: 20})
	out := s.View()
	if !contains(out, "alice@research") {
		t.Errorf("SetState subject not reflected in View:\n%s", out)
	}
}

// TestStepIdentity_FocusBlur cycles focus state.
func TestStepIdentity_FocusBlur(t *testing.T) {
	s := NewStepIdentity(nil)
	if s.Focused() {
		t.Error("fresh step should not be focused")
	}
	s.Focus()
	if !s.Focused() {
		t.Error("Focus() did not set Focused")
	}
	s.Blur()
	if s.Focused() {
		t.Error("Blur() did not clear Focused")
	}
}

// TestStepRecipient_FocusBlur cycles focus state.
func TestStepRecipient_FocusBlur(t *testing.T) {
	s := NewStepRecipient(nil)
	if s.Focused() {
		t.Error("fresh step should not be focused")
	}
	s.Focus()
	if !s.Focused() {
		t.Error("Focus() did not set Focused")
	}
	s.Blur()
	if s.Focused() {
		t.Error("Blur() did not clear Focused")
	}
}

// TestStepFreeFields_FocusBlur cycles focus state.
func TestStepFreeFields_FocusBlur(t *testing.T) {
	s := NewStepFreeFields()
	if s.Focused() {
		t.Error("fresh step should not be focused")
	}
	s.Focus()
	if !s.Focused() {
		t.Error("Focus() did not set Focused")
	}
	s.Blur()
	if s.Focused() {
		t.Error("Blur() did not clear Focused")
	}
}

// TestStepTOTP_FocusBlur cycles focus state.
func TestStepTOTP_FocusBlur(t *testing.T) {
	s := NewStepTOTP(nil)
	if s.Focused() {
		t.Error("fresh step should not be focused")
	}
	s.Focus()
	if !s.Focused() {
		t.Error("Focus() did not set Focused")
	}
	s.Blur()
	if s.Focused() {
		t.Error("Blur() did not clear Focused")
	}
}

// TestParseUUID covers the wizard helper that wraps uuid.Parse.
func TestParseUUID(t *testing.T) {
	if _, err := parseUUID("00000000-0000-0000-0000-000000000001"); err != nil {
		t.Errorf("valid uuid rejected: %v", err)
	}
	if _, err := parseUUID("not-a-uuid"); err == nil {
		t.Error("invalid uuid accepted")
	}
}

// keyForString converts a key spec like "esc" / "ctrl+s" / "tab" to the
// corresponding tea.KeyType so tests can drive Update without manually
// building each KeyMsg.
func keyForString(s string) tea.KeyType {
	switch s {
	case "esc":
		return tea.KeyEsc
	case "tab":
		return tea.KeyTab
	case "enter":
		return tea.KeyEnter
	case "ctrl+s":
		return tea.KeyCtrlS
	}
	return tea.KeyRunes
}

// contains is a small wrapper around strings.Contains so tests don't have
// to import strings just for that.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
