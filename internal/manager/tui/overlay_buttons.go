package tui

import "github.com/charmbracelet/lipgloss"

// buttonKind selects the colour scheme for an overlay button.
type buttonKind int

const (
	btnNeutral buttonKind = iota
	btnPrimary
	btnDanger
)

// button is a clickable choice rendered in an overlay footer row.
// Focused buttons get a bold accent background to signal the default action.
type button struct {
	label   string
	hotkey  string
	kind    buttonKind
	focused bool
}

// renderButtons builds a right-aligned footer row of buttons separated by two
// spaces, padded to width. Matches the prototype's `Btn` component (overlays.jsx Btn).
func renderButtons(width int, btns ...button) string {
	if len(btns) == 0 {
		return ""
	}
	parts := make([]string, len(btns))
	for i, b := range btns {
		parts[i] = b.render()
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, joinSpaced(parts, "  ")...)
	if width <= 0 {
		return row
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(row)
}

func joinSpaced(parts []string, sep string) []string {
	if len(parts) <= 1 {
		return parts
	}
	out := make([]string, 0, len(parts)*2-1)
	for i, p := range parts {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, p)
	}
	return out
}

func (b button) render() string {
	accent := Palette.FgMute
	switch b.kind {
	case btnPrimary:
		accent = Palette.Magenta
	case btnDanger:
		accent = Palette.Red
	}
	hot := lipgloss.NewStyle().Foreground(accent).Bold(true).Render("[" + b.hotkey + "] ")
	labelStyle := lipgloss.NewStyle().Foreground(Palette.Fg)
	if b.focused {
		labelStyle = labelStyle.Foreground(accent).Bold(true)
	}
	return hot + labelStyle.Render(b.label)
}
