package tui

import "github.com/charmbracelet/lipgloss"

// ChromeRows is the row budget subtracted from m.hgt to compute the
// available content height. Screens use `m.hgt - ChromeRows` for sizing.
// Update this constant if the chrome layout changes; do not duplicate the
// literal locally.
const ChromeRows = 4

// TopChromeRows is the number of rows the top chrome occupies on the screen
// BEFORE the content area starts. = title(1) + tab strip(2 — active tab
// underline counts as a second row) + breadcrumb(1). Use this for absolute Y
// calculations of elements rendered inside the screen body; ChromeRows is
// for HEIGHT budgeting only.
const TopChromeRows = 4

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

// BoxedInner returns the content width available inside BoxStyle when the
// outer block is `total` columns wide — i.e. `total` minus border AND padding.
// Use this for tables, viewports, or any text laid out inside the box.
func BoxedInner(total int) int {
	w := total - BoxStyle.GetHorizontalFrameSize()
	if w < 0 {
		return 0
	}
	return w
}

// BoxedWidth returns the value to pass to `BoxStyle.Width(...)` so the box's
// outer rendered width is exactly `total` columns. lipgloss `.Width()` sets
// the content area (padding included, border excluded); this helper subtracts
// only the border so callers don't have to remember which lipgloss component
// `.Width()` is measuring.
func BoxedWidth(total int) int {
	w := total - BoxStyle.GetHorizontalBorderSize()
	if w < 0 {
		return 0
	}
	return w
}
