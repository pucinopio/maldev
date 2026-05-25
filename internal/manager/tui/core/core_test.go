package core

import "testing"

func TestRectContains(t *testing.T) {
	r := Rect{X: 5, Y: 10, W: 4, H: 3}
	cases := []struct {
		x, y int
		want bool
	}{
		{5, 10, true},   // top-left corner
		{8, 12, true},   // bottom-right inside (W=4 → X in [5,9), H=3 → Y in [10,13))
		{9, 12, false},  // X just past right edge
		{8, 13, false},  // Y just past bottom edge
		{4, 10, false},  // X just left of left edge
		{5, 9, false},   // Y just above top edge
		{0, 0, false},   // far origin
		{100, 100, false}, // far away
		{5, 12, true},   // left edge, mid-height
		{7, 10, true},   // top edge, mid-width
	}
	for _, c := range cases {
		if got := r.Contains(c.x, c.y); got != c.want {
			t.Errorf("(%d,%d): got %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

// TestRectContains_EmptyRect documents that a zero-W or zero-H Rect contains
// nothing — the half-open interval x < X+W collapses when W==0.
func TestRectContains_EmptyRect(t *testing.T) {
	zeroW := Rect{X: 0, Y: 0, W: 0, H: 5}
	if zeroW.Contains(0, 0) {
		t.Error("empty W rect should contain nothing")
	}
	zeroH := Rect{X: 0, Y: 0, W: 5, H: 0}
	if zeroH.Contains(0, 0) {
		t.Error("empty H rect should contain nothing")
	}
}
