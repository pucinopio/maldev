package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// composeOverlay centers overlay over body and dims the surrounding body cells
// to produce a scrim effect. body and overlay are multi-line ANSI strings;
// their visual sizes are inferred via ansi.StringWidth.
func composeOverlay(body, overlay string, totalW, totalH int) string {
	bodyLines := strings.Split(body, "\n")
	for len(bodyLines) < totalH {
		bodyLines = append(bodyLines, "")
	}
	bodyLines = bodyLines[:totalH]

	overlayLines := strings.Split(overlay, "\n")
	overlayH := len(overlayLines)
	overlayW := 0
	for _, l := range overlayLines {
		if w := ansi.StringWidth(l); w > overlayW {
			overlayW = w
		}
	}
	top := max(0, (totalH-overlayH)/2)
	left := max(0, (totalW-overlayW)/2)

	scrim := Mute.Faint(true)

	out := make([]string, totalH)
	for i, line := range bodyLines {
		line = ansi.Truncate(line, totalW, "")
		if i < top || i >= top+overlayH {
			out[i] = scrim.Render(line)
			continue
		}
		ol := overlayLines[i-top]
		olW := ansi.StringWidth(ol)
		leftPart := ansi.Cut(line, 0, left)
		rightStart := left + olW
		rightPart := ""
		if rightStart < totalW {
			rightPart = ansi.Cut(line, rightStart, totalW)
		}
		out[i] = scrim.Render(leftPart) + ol + scrim.Render(rightPart)
	}
	return strings.Join(out, "\n")
}
