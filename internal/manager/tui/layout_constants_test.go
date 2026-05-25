package tui

import "testing"

// TestLayoutConstants pins the values returned by the layout helpers so a
// future tweak to BoxStyle (extra padding, thicker border) shows up here
// before it propagates silently to every screen that subtracts magic numbers.
func TestLayoutConstants(t *testing.T) {
	if ChromeRows != 4 {
		t.Errorf("ChromeRows = %d, want 4 (title+tabs+breadcrumb+statusbar)", ChromeRows)
	}
	if w, h := BoxFrame(); w != 4 || h != 2 {
		t.Errorf("BoxFrame() = (%d, %d), want (4, 2) — BoxStyle border+padding changed?", w, h)
	}
	if w, h := ModalFrame(); w != 6 || h != 4 {
		t.Errorf("ModalFrame() = (%d, %d), want (6, 4) — Modal style changed?", w, h)
	}
	if got := BoxedInner(100); got != 96 {
		t.Errorf("BoxedInner(100) = %d, want 96 (100 - BoxFrame.h=4)", got)
	}
	if got := BoxedWidth(100); got != 98 {
		t.Errorf("BoxedWidth(100) = %d, want 98 (100 - border=2)", got)
	}
	// Edge: total smaller than the frame should clamp to 0, never go negative.
	if got := BoxedInner(2); got != 0 {
		t.Errorf("BoxedInner(2) = %d, want 0 (clamped)", got)
	}
	if got := BoxedWidth(1); got != 0 {
		t.Errorf("BoxedWidth(1) = %d, want 0 (clamped)", got)
	}
}
