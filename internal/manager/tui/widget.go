// Package tui implements the bubbletea-based terminal UI for license-manager.
package tui

import tea "github.com/charmbracelet/bubbletea"

// Rect is a bounding box in terminal cell coordinates.
type Rect struct {
	X, Y, W, H int
}

// Contains reports whether cell (x,y) falls inside r.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Widget is the base abstraction for every visible piece of UI.
// Screens compose widgets into a tree; the root assigns bounds via Layout
// during WindowSizeMsg events, dispatches Msg to children, and aggregates
// View() strings.
type Widget interface {
	// Layout receives the bounding box this widget should fill.
	// Composite widgets recursively compute child bounds. Call again on resize.
	Layout(bounds Rect)

	// Bounds returns the rect assigned by the last Layout call.
	Bounds() Rect

	// Update receives any tea.Msg. Returns the widget (possibly updated)
	// plus any cmd to schedule.
	Update(msg tea.Msg) (Widget, tea.Cmd)

	// View renders the widget. Must respect Bounds() — never overflow.
	View() string
}

// Clickable is the optional interface for widgets that handle mouse clicks.
// The root dispatches MouseMsg to the deepest widget whose HitTest matches.
type Clickable interface {
	Widget
	// OnClick is called when the user releases a left button inside Bounds.
	// x, y are relative to Bounds() top-left (0,0 = widget's own origin).
	OnClick(x, y int, button tea.MouseButton) tea.Cmd
}

// Focusable widgets receive Tab focus and explicit key events.
type Focusable interface {
	Widget
	Focus()
	Blur()
	Focused() bool
}
