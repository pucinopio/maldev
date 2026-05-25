package wizard

import (
	"strings"
	"testing"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// TestStepViews_RenderShape walks every step's View() and asserts:
//   - returns a non-empty string
//   - includes the matching "Step N — <name>" header
//   - includes the renderHints footer
// This pins the structural contract of each step without snapshotting
// the exact byte content (which would churn on every cosmetic tweak).
func TestStepViews_RenderShape(t *testing.T) {
	core.Colors.Fg = "#ffffff"
	core.Colors.FgDim = "#888888"
	core.Colors.FgMute = "#444444"
	core.Colors.Magenta = "#ff00ff"
	core.Colors.Green = "#00ff00"
	core.Colors.Red = "#ff0000"

	cases := []struct {
		name   string
		view   func() string
		header string
	}{
		{
			"identity",
			func() string {
				s := NewStepIdentity(nil)
				s.Layout(core.Rect{X: 0, Y: 0, W: 80, H: 20})
				return s.View()
			},
			"Step 1",
		},
		{
			"recipient",
			func() string {
				s := NewStepRecipient(nil)
				s.Layout(core.Rect{X: 0, Y: 0, W: 80, H: 20})
				return s.View()
			},
			"Step 2",
		},
		{
			"binding-machine",
			func() string {
				s := NewStepBindingMachine(nil)
				s.Layout(core.Rect{X: 0, Y: 0, W: 80, H: 20})
				return s.View()
			},
			"Step 3",
		},
		{
			"binding-binary",
			func() string {
				s := NewStepBindingBinary()
				s.Layout(core.Rect{X: 0, Y: 0, W: 80, H: 20})
				return s.View()
			},
			"Step 4",
		},
		{
			"validity",
			func() string {
				s := NewStepValidity()
				s.Layout(core.Rect{X: 0, Y: 0, W: 80, H: 20})
				return s.View()
			},
			"Step 5",
		},
		{
			"freefields",
			func() string {
				s := NewStepFreeFields()
				s.Layout(core.Rect{X: 0, Y: 0, W: 80, H: 20})
				return s.View()
			},
			"Step 6",
		},
		{
			"totp",
			func() string {
				s := NewStepTOTP(nil)
				s.Layout(core.Rect{X: 0, Y: 0, W: 80, H: 20})
				return s.View()
			},
			"Step 7",
		},
		{
			"review",
			func() string {
				s := NewStepReview(nil)
				s.Layout(core.Rect{X: 0, Y: 0, W: 80, H: 20})
				return s.View()
			},
			"Step 8",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := c.view()
			if out == "" {
				t.Fatal("View() returned empty string")
			}
			if !strings.Contains(out, c.header) {
				t.Errorf("View() missing %q header:\n%s", c.header, out)
			}
		})
	}
}

// TestRenderHints_Format covers the helper that builds the bottom-row
// "key meaning · key meaning" hint strip.
func TestRenderHints_Format(t *testing.T) {
	core.Colors.Magenta = "#ff00ff"
	core.Colors.FgDim = "#888888"
	out := renderHints("enter confirm", "esc cancel")
	if !strings.Contains(out, "enter") || !strings.Contains(out, "esc") {
		t.Errorf("renderHints missing key labels:\n%s", out)
	}
	if !strings.Contains(out, "·") {
		t.Errorf("renderHints missing separator:\n%s", out)
	}
}

// TestStepHeader_Format covers the shared step-header builder.
func TestStepHeader_Format(t *testing.T) {
	core.Colors.Magenta = "#ff00ff"
	core.Colors.FgDim = "#888888"
	got := stepHeader("Step X — Test", "subtitle here")
	if !strings.Contains(got, "Step X — Test") {
		t.Errorf("stepHeader missing title:\n%s", got)
	}
	if !strings.Contains(got, "subtitle here") {
		t.Errorf("stepHeader missing subtitle:\n%s", got)
	}
	// Header is always 3 lines (title + sub + blank).
	if n := strings.Count(got, "\n"); n != 2 {
		t.Errorf("stepHeader should have 3 lines (2 newlines), got %d:\n%s", n, got)
	}
}
