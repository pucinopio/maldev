package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// composeOverlay paints overlay centered over body, dimming the surrounding
// body cells to produce a scrim effect. body and overlay are multi-line ANSI
// strings; their visual sizes are inferred from lipgloss.Width / Height.
//
// Cells outside the overlay rectangle are wrapped in a dim style so the
// operator's eye is drawn to the modal without losing the underlying context.
// Cells inside the overlay rectangle are replaced with the overlay's own ANSI.
func composeOverlay(body, overlay string, totalW, totalH int) string {
	bodyLines := strings.Split(body, "\n")
	for len(bodyLines) < totalH {
		bodyLines = append(bodyLines, "")
	}
	bodyLines = bodyLines[:totalH]

	ovLines := strings.Split(overlay, "\n")
	ovH := len(ovLines)
	ovW := 0
	for _, l := range ovLines {
		if w := ansi.StringWidth(l); w > ovW {
			ovW = w
		}
	}
	top := (totalH - ovH) / 2
	if top < 0 {
		top = 0
	}
	left := (totalW - ovW) / 2
	if left < 0 {
		left = 0
	}

	scrim := lipgloss.NewStyle().Foreground(Palette.FgMute).Faint(true)

	out := make([]string, totalH)
	for i, line := range bodyLines {
		line = ansi.Truncate(line, totalW, "")
		if i < top || i >= top+ovH {
			out[i] = scrim.Render(line)
			continue
		}
		ovLine := ovLines[i-top]
		ovLineW := ansi.StringWidth(ovLine)
		leftPart := ansi.Cut(line, 0, left)
		rightStart := left + ovLineW
		rightPart := ""
		if rightStart < totalW {
			rightPart = ansi.Cut(line, rightStart, totalW)
		}
		out[i] = scrim.Render(leftPart) + ovLine + scrim.Render(rightPart)
	}
	return strings.Join(out, "\n")
}
