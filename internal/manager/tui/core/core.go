// Package core defines the base Widget interfaces and Rect type shared by
// the tui package and its widgets/ sub-package. Keeping them here breaks the
// import cycle: tui → tui/widgets → tui.
package core

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Colors holds the palette values injected by tui/theme.go at init time.
// widgets/ reads these instead of hard-coding hex literals.
var Colors struct {
	Bg1      lipgloss.Color // #0a0a18 — tab bar / status bar background
	Border   lipgloss.Color // #2a2a52 — default border
	Fg       lipgloss.Color // #e6e6ff — primary text
	FgDim    lipgloss.Color // #7a7ab8 — secondary / dim text
	FgMute   lipgloss.Color // #4a4a78 — muted / separator
	Magenta  lipgloss.Color // #ff36d4 — accent / active indicator
	Green    lipgloss.Color // #39ff88
	Yellow   lipgloss.Color // #ffce39
	Red      lipgloss.Color // #ff3c5f
}

// Rect is a bounding box in terminal cell coordinates.
type Rect struct {
	X, Y, W, H int
}

// Contains reports whether cell (x,y) falls inside r.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Widget is the base abstraction for every visible piece of UI.
type Widget interface {
	// Layout receives the bounding box this widget should fill.
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
