package tui

import (
	"fmt"
	"strings"
	"time"

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
	{ViewIssuers, "Issuer keys"},
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

// appVersion is shown in the title bar.
const appVersion = "v0.4.0-dev"

// titleBarClock is the clock used by renderTitleBar. Tests may replace it with
// a fixed-time func to produce deterministic golden output.
var titleBarClock = time.Now

// renderTitleBar returns the top title bar:
//
//	left:  ◆ license-manager v0.4.0-dev
//	right: db: <dbName> | net: online | http: N/3 ON | HH:MM:SS DD/MM/YYYY
func renderTitleBar(width int) string {
	return renderTitleBarWith(width, "db.sqlite", 0, titleBarClock())
}

// renderTitleBarWith is the testable core of renderTitleBar.
// dbName is the database filename; httpOn is the number of running HTTP servers.
func renderTitleBarWith(width int, dbName string, httpOn int, now time.Time) string {
	diamond := GlowMagent.Render("◆")
	appName := lipgloss.NewStyle().Foreground(Palette.Cyan).Bold(true).Render(" license-manager ")
	ver := Dim.Render(appVersion)
	left := diamond + appName + ver

	right := fmt.Sprintf("db: %s | net: online | http: %d/3 ON | %s",
		dbName,
		httpOn,
		now.Format("02/01/2006 15:04:05"),
	)
	rightRendered := Dim.Render(right)

	gap := width - lipgloss.Width(left) - lipgloss.Width(rightRendered)
	if gap < 1 {
		gap = 1
	}
	bar := left + strings.Repeat(" ", gap) + rightRendered
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
