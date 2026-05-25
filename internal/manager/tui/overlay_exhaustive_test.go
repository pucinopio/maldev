package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

// TestOverlayHotkeys_ExhaustivelyDispatch fires every documented hotkey
// against every overlay and asserts the resulting OverlayDoneMsg payload
// (or absence) matches the spec. Guards against silent regressions where
// a key handler stops emitting the right msg type.
func TestOverlayHotkeys_ExhaustivelyDispatch(t *testing.T) {
	t.Run("confirm-y-confirms", func(t *testing.T) {
		o := newConfirmOverlay("test", "Title", "Body", "OK", "Cancel", false)
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
		res := mustOverlayDone(t, cmd).Result.(ConfirmResultMsg)
		if !res.Confirm {
			t.Errorf("y should confirm, got Confirm=false")
		}
	})
	t.Run("confirm-Y-uppercase-confirms", func(t *testing.T) {
		o := newConfirmOverlay("test", "Title", "Body", "OK", "Cancel", false)
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")})
		res := mustOverlayDone(t, cmd).Result.(ConfirmResultMsg)
		if !res.Confirm {
			t.Errorf("uppercase Y should confirm, got Confirm=false")
		}
	})
	t.Run("confirm-enter-confirms", func(t *testing.T) {
		o := newConfirmOverlay("test", "Title", "Body", "OK", "Cancel", false)
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEnter})
		res := mustOverlayDone(t, cmd).Result.(ConfirmResultMsg)
		if !res.Confirm {
			t.Errorf("enter should confirm, got Confirm=false")
		}
	})
	t.Run("confirm-n-cancels", func(t *testing.T) {
		o := newConfirmOverlay("test", "Title", "Body", "OK", "Cancel", false)
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
		res := mustOverlayDone(t, cmd).Result.(ConfirmResultMsg)
		if res.Confirm {
			t.Errorf("n should cancel, got Confirm=true")
		}
	})
	t.Run("confirm-esc-cancels", func(t *testing.T) {
		o := newConfirmOverlay("test", "Title", "Body", "OK", "Cancel", false)
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEsc})
		res := mustOverlayDone(t, cmd).Result.(ConfirmResultMsg)
		if res.Confirm {
			t.Errorf("esc should cancel, got Confirm=true")
		}
	})

	// Revoke overlay — user reported this was the buggy one.
	t.Run("revoke-enter-empty-input-noop", func(t *testing.T) {
		o := newRevokeOverlay(uuid.New(), "subject")
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			t.Errorf("enter on empty input should be no-op, got cmd %T", cmd())
		}
	})
	t.Run("revoke-enter-with-input-confirms", func(t *testing.T) {
		id := uuid.New()
		o := newRevokeOverlay(id, "subject")
		o.input.SetValue("key_compromised")
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEnter})
		res := mustOverlayDone(t, cmd).Result.(RevokeConfirmedMsg)
		if res.LicenseID != id {
			t.Errorf("LicenseID = %v, want %v", res.LicenseID, id)
		}
		if res.Reason != "key_compromised" {
			t.Errorf("Reason = %q, want key_compromised", res.Reason)
		}
	})
	t.Run("revoke-esc-cancels", func(t *testing.T) {
		o := newRevokeOverlay(uuid.New(), "subject")
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEsc})
		done := mustOverlayDone(t, cmd)
		if done.Result != nil {
			t.Errorf("esc should produce nil Result, got %T", done.Result)
		}
	})
	t.Run("revoke-chip-click-populates-input", func(t *testing.T) {
		o := newRevokeOverlay(uuid.New(), "subject")
		// Render once so chipRects populates.
		_ = o.View()
		if len(o.chipRects) == 0 {
			t.Fatal("chipRects empty after View")
		}
		// Click middle of first chip.
		c := o.chipRects[0]
		_, cmd := o.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: c.x1 + (c.x2-c.x1)/2, Y: c.y})
		if cmd != nil {
			t.Errorf("chip click should populate input without emitting cmd, got %T", cmd())
		}
		if o.input.Value() != c.reason {
			t.Errorf("input value = %q, want %q", o.input.Value(), c.reason)
		}
	})
	t.Run("revoke-button-click-revoque", func(t *testing.T) {
		id := uuid.New()
		o := newRevokeOverlay(id, "subject")
		o.input.SetValue("test-reason")
		_ = o.View() // populates footerY
		_, cmd := o.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 40, Y: o.footerY})
		res := mustOverlayDone(t, cmd).Result.(RevokeConfirmedMsg)
		if res.Reason != "test-reason" {
			t.Errorf("Reason = %q, want test-reason", res.Reason)
		}
	})
	t.Run("revoke-button-click-annuler", func(t *testing.T) {
		o := newRevokeOverlay(uuid.New(), "subject")
		_ = o.View()
		_, cmd := o.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 5, Y: o.footerY})
		done := mustOverlayDone(t, cmd)
		if done.Result != nil {
			t.Errorf("Annuler click should produce nil Result, got %T", done.Result)
		}
	})

	// Input overlay — empty enter should noop.
	t.Run("input-enter-empty-noop", func(t *testing.T) {
		o := newInputOverlay("id", "title", "ph", 100)
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			t.Errorf("enter on empty input should noop, got cmd %T", cmd())
		}
	})
	t.Run("input-enter-typed-confirms", func(t *testing.T) {
		o := newInputOverlay("id", "title", "ph", 100)
		o.input.SetValue("value")
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEnter})
		res := mustOverlayDone(t, cmd).Result.(InputResultMsg)
		if res.Value != "value" || res.ID != "id" {
			t.Errorf("got %+v", res)
		}
	})
	t.Run("input-esc-cancels", func(t *testing.T) {
		o := newInputOverlay("id", "title", "ph", 100)
		o.input.SetValue("value")
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEsc})
		done := mustOverlayDone(t, cmd)
		if done.Result != nil {
			t.Errorf("esc should produce nil Result, got %T", done.Result)
		}
	})

	// Quit overlay
	t.Run("quit-y-quits", func(t *testing.T) {
		o := newQuitOverlay(true) // servers running
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
		done := mustOverlayDone(t, cmd)
		if b, ok := done.Result.(bool); !ok || !b {
			t.Errorf("y should produce bool=true, got %T %v", done.Result, done.Result)
		}
	})
	t.Run("quit-n-cancels", func(t *testing.T) {
		o := newQuitOverlay(true)
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
		done := mustOverlayDone(t, cmd)
		if b, ok := done.Result.(bool); !ok || b {
			t.Errorf("n should produce bool=false, got %T %v", done.Result, done.Result)
		}
	})

	// OK + Error: any key dismisses
	t.Run("ok-dismiss", func(t *testing.T) {
		o := NewOKOverlay("title", "body")
		for _, k := range []tea.KeyType{tea.KeyEnter, tea.KeyEsc, tea.KeyRunes} {
			fresh := NewOKOverlay("title", "body")
			_, cmd := fresh.Update(tea.KeyMsg{Type: k, Runes: []rune("q")})
			if _, ok := cmd().(OverlayDoneMsg); !ok {
				t.Errorf("OK %v: cmd = %T, want OverlayDoneMsg", k, cmd())
			}
		}
		_ = o
	})
	t.Run("error-dismiss", func(t *testing.T) {
		o := newErrorOverlay("title", "body")
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if _, ok := cmd().(OverlayDoneMsg); !ok {
			t.Errorf("error enter: cmd = %T, want OverlayDoneMsg", cmd())
		}
	})

	// Help overlay
	t.Run("help-dismiss-on-?", func(t *testing.T) {
		o := NewHelpOverlay()
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
		if _, ok := cmd().(OverlayDoneMsg); !ok {
			t.Errorf("help ?: cmd = %T, want OverlayDoneMsg", cmd())
		}
	})
}

// mustOverlayDone asserts cmd produces an OverlayDoneMsg and returns it.
func mustOverlayDone(t *testing.T, cmd tea.Cmd) OverlayDoneMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("cmd is nil; expected OverlayDoneMsg")
	}
	msg := cmd()
	done, ok := msg.(OverlayDoneMsg)
	if !ok {
		t.Fatalf("cmd produced %T, want OverlayDoneMsg", msg)
	}
	return done
}
