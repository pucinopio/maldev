package tui

import tea "github.com/charmbracelet/bubbletea"

// hitRect maps a rectangular click region (in absolute terminal cells) to the
// command it should emit. Screens that don't have a full widget tree can still
// expose clickable elements by appending hitRects to their model as they render.
type hitRect struct {
	x, y, w, h int
	cmd        func() tea.Cmd
}

// hits is a slice of hitRect; methods make the screen models terser.
type hits []hitRect

func (h *hits) reset() { *h = (*h)[:0] }

func (h *hits) add(x, y, w, hgt int, cmd func() tea.Cmd) {
	*h = append(*h, hitRect{x: x, y: y, w: w, h: hgt, cmd: cmd})
}

// dispatch returns the command for the topmost hit rect containing (x,y), or
// nil when nothing matches. Iterates in reverse so the last-registered region
// wins when regions overlap (painter's-model stacking order).
func (h hits) dispatch(x, y int) tea.Cmd {
	for i := len(h) - 1; i >= 0; i-- {
		r := h[i]
		if x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h {
			return r.cmd()
		}
	}
	return nil
}
