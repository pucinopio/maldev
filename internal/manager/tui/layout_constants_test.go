package tui

import "testing"

// TestClampTableHeight covers the three constraints the helper enforces.
func TestClampTableHeight(t *testing.T) {
	cases := []struct {
		name             string
		in               int
		detail, empty    bool
		want             int
	}{
		{"plain", 20, false, false, 20},
		{"halve on detail", 20, true, false, 10},
		{"min 3", 1, false, false, 3},
		{"empty collapses to 1", 20, false, true, 1},
		{"empty wins over detail+min", 1, true, true, 1},
		{"detail+min", 1, true, false, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampTableHeight(c.in, c.detail, c.empty); got != c.want {
				t.Errorf("clampTableHeight(%d,%v,%v) = %d, want %d", c.in, c.detail, c.empty, got, c.want)
			}
		})
	}
}

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
