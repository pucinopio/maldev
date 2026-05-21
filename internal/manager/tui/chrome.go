package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui/widgets"
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

// buildTabBar constructs a widgets.TabBar from tabDefs for the given active view.
func buildTabBar(active ViewID, width int) *widgets.TabBar {
	items := make([]widgets.TabItem, len(tabDefs))
	for i, td := range tabDefs {
		items[i] = widgets.TabItem{ID: string(td.ID), Label: td.Label}
	}
	tb := widgets.NewTabBar(items, string(active))
	tb.Layout(Rect{X: 0, Y: 1, W: width, H: 1})
	return tb
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

// renderTabStrip returns the tab strip using a TabBar widget.
func renderTabStrip(active ViewID, width int) string {
	return buildTabBar(active, width).View()
}

// renderStatusBar returns the bottom status/hint bar using a StatusBar widget.
func renderStatusBar(hints []string, width int) string {
	kh := make([]widgets.KeyHint, 0, len(hints)/2)
	for i := 0; i+1 < len(hints); i += 2 {
		kh = append(kh, widgets.KeyHint{Key: hints[i], Desc: hints[i+1]})
	}
	sb := widgets.NewStatusBar(kh...)
	sb.Layout(Rect{W: width, H: 1})
	return sb.View()
}
