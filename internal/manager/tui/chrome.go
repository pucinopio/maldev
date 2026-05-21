package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// tabDefs is the single source of truth for tab order, IDs, and labels.
// app.go's viewOrder is derived from this slice at init time.
var tabDefs = []struct {
	ID    ViewID
	Label string
}{
	{ViewDashboard, "Dashboard"},
	{ViewLicenses, "Licenses"},
	{ViewIssuers, "Issuers"},
	{ViewRecipients, "Recipients"},
	{ViewIdentities, "Identities"},
	{ViewRevocation, "Revocation"},
	{ViewServers, "Servers"},
	{ViewAudit, "Audit"},
	{ViewSettings, "Settings"},
}

func init() {
	// Keep viewOrder in sync with tabDefs so keyboard navigation (1-9) matches the strip.
	viewOrder = make([]ViewID, len(tabDefs))
	for i, td := range tabDefs {
		viewOrder[i] = td.ID
	}
}

// renderTitleBar returns the top title bar string.
func renderTitleBar(width int) string {
	title := GlowMagent.Render(" license-manager ")
	sub := Dim.Render(" oioio-space/maldev ")
	pad := width - lipgloss.Width(title) - lipgloss.Width(sub)
	if pad < 0 {
		pad = 0
	}
	bar := title + strings.Repeat(" ", pad) + sub
	return lipgloss.NewStyle().
		Background(Palette.Bg2).
		Width(width).
		Render(bar)
}

// renderTabStrip returns the tab strip for the given active view and total width.
func renderTabStrip(active ViewID, width int) string {
	var parts []string
	for i, td := range tabDefs {
		label := td.Label
		if i < 9 {
			label = Dim.Render(string(rune('1'+i))) + " " + label
		}
		if td.ID == active {
			parts = append(parts, TabActive.Render(label))
		} else {
			parts = append(parts, TabInactive.Render(label))
		}
	}
	strip := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	return lipgloss.NewStyle().
		Background(Palette.Bg1).
		Width(width).
		Render(strip)
}

// renderStatusBar returns the bottom status/hint bar.
func renderStatusBar(hints []string, width int) string {
	var parts []string
	for i := 0; i+1 < len(hints); i += 2 {
		k := HintKey.Render(hints[i])
		v := HintText.Render(hints[i+1])
		parts = append(parts, k+v)
	}
	bar := strings.Join(parts, Dim.Render("  "))
	return lipgloss.NewStyle().
		Background(Palette.Bg1).
		Width(width).
		Render(bar)
}
