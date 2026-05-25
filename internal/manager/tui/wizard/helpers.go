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

// Shared style values used by every step's View(). They depend on
// core.Colors which theme.go init() populates before any wizard step is
// constructed. lipgloss.Style is a value type so reading them at
// package-init time captures the colours once; if ApplyTheme runs later
// the wizard will keep its boot-time look until restart (same limitation
// as widgets/, deliberate).
var (
	wizSel = lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true)
	wizFg  = lipgloss.NewStyle().Foreground(core.Colors.Fg)
	wizDim = lipgloss.NewStyle().Foreground(core.Colors.FgDim)
)

// renderHints joins a set of "key  meaning" pairs separated by " · " with the
// key in magenta and the meaning in dim. Wizard steps use it to surface their
// keybindings consistently.
func renderHints(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		sp := strings.SplitN(p, " ", 2)
		if len(sp) == 2 {
			out = append(out, wizSel.Render(sp[0])+" "+wizDim.Render(sp[1]))
		} else {
			out = append(out, wizDim.Render(p))
		}
	}
	return "  " + strings.Join(out, wizDim.Render(" · "))
}
