package wizard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// parseUUID parses a UUID string, returning a descriptive error on failure.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// renderHints joins a set of "key  meaning" pairs separated by " · " with the
// key in magenta and the meaning in dim. Wizard steps use it to surface their
// keybindings consistently.
func renderHints(parts ...string) string {
	keyStyle := lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(core.Colors.FgDim)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		sp := strings.SplitN(p, " ", 2)
		if len(sp) == 2 {
			out = append(out, keyStyle.Render(sp[0])+" "+dimStyle.Render(sp[1]))
		} else {
			out = append(out, dimStyle.Render(p))
		}
	}
	return "  " + strings.Join(out, dimStyle.Render(" · "))
}
