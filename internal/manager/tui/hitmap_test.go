package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHits_AddAndDispatch covers the basic add → dispatch cycle and the
// reverse-iteration stacking rule (last-registered wins for overlapping
// rects — painter's model).
func TestHits_AddAndDispatch(t *testing.T) {
	tag := func(s string) func() tea.Cmd {
		return func() tea.Cmd { return func() tea.Msg { return s } }
	}
	var h hits
	h.add(0, 0, 10, 5, tag("first"))
	h.add(5, 0, 10, 5, tag("second")) // overlaps the right half of first
	h.add(20, 0, 5, 5, tag("third"))

	cases := []struct {
		x, y int
		want string
	}{
		{2, 2, "first"},   // only first contains
		{7, 2, "second"},  // both contain — second wins (last registered)
		{22, 2, "third"},  // only third contains
		{30, 30, ""},      // nobody contains
	}
	for _, c := range cases {
		cmd := h.dispatch(c.x, c.y)
		if c.want == "" {
			if cmd != nil {
				t.Errorf("dispatch(%d,%d) returned cmd, want nil", c.x, c.y)
			}
			continue
		}
		if cmd == nil {
			t.Errorf("dispatch(%d,%d) returned nil, want %q", c.x, c.y, c.want)
			continue
		}
		if got := cmd().(string); got != c.want {
			t.Errorf("dispatch(%d,%d) = %q, want %q", c.x, c.y, got, c.want)
		}
	}
}

// TestHits_Reset drops every recorded rect.
func TestHits_Reset(t *testing.T) {
	var h hits
	h.add(0, 0, 1, 1, func() tea.Cmd { return nil })
	if len(h) != 1 {
		t.Fatalf("len after add = %d, want 1", len(h))
	}
	h.reset()
	if len(h) != 0 {
		t.Errorf("len after reset = %d, want 0", len(h))
	}
	if cap(h) == 0 {
		t.Error("reset should preserve underlying capacity for next frame")
	}
}
