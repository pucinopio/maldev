package tui

import "github.com/charmbracelet/lipgloss"

// ChromeRows is the number of rows reserved by the chrome shell — title bar,
// tab strip, breadcrumb, and status bar — around every Ready-session screen.
// Screens compute their content height as `m.hgt - ChromeRows`. Update this
// constant if the chrome layout changes; do not duplicate the literal locally.
const ChromeRows = 4

// BoxFrame returns the horizontal and vertical overhead introduced by the
// standard BoxStyle (border + padding + margin). Use this in size calculations
// instead of hard-coded `- 4` / `- 2` so layout stays correct if BoxStyle is
// retuned.
func BoxFrame() (w, h int) {
	return BoxStyle.GetHorizontalFrameSize(), BoxStyle.GetVerticalFrameSize()
}

// ModalFrame returns the horizontal and vertical overhead of the Modal style,
// for centered overlays whose inner content must fit in the remaining area.
func ModalFrame() (w, h int) {
	return Modal.GetHorizontalFrameSize(), Modal.GetVerticalFrameSize()
}

// FrameOf returns the horizontal and vertical overhead (border + padding +
// margin combined) of an arbitrary lipgloss style. Use it when a screen has
// its own one-off box style and still wants frame-aware sizing.
func FrameOf(s lipgloss.Style) (w, h int) {
	return s.GetHorizontalFrameSize(), s.GetVerticalFrameSize()
}
