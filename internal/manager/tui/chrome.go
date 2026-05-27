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
	{ViewTOTP, "TOTP"},
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
	appName := GlowCyan.Render(" license-manager ")
	ver := Dim.Render(appVersion)
	left := diamond + appName + ver
	leftW := lipgloss.Width(left)

	// Build progressively narrower right-side variants. Pick the longest
	// that fits in the budget = width - leftW - 1 (one cell of gap). At
	// very narrow widths the right side disappears entirely so the bar
	// never wraps to a second line.
	dt := now.Format("02/01/2006 15:04:05")
	full := fmt.Sprintf("db: %s | net: online | http: %d/3 ON | %s", dbName, httpOn, dt)
	medium := fmt.Sprintf("http: %d/3 ON | %s", httpOn, dt)
	short := dt
	budget := width - leftW - 1
	right := ""
	switch {
	case budget >= lipgloss.Width(full):
		right = full
	case budget >= lipgloss.Width(medium):
		right = medium
	case budget >= lipgloss.Width(short):
		right = short
	}
	var bar string
	if right == "" {
		bar = left
	} else {
		rightRendered := Dim.Render(right)
		gap := width - leftW - lipgloss.Width(rightRendered)
		if gap < 1 {
			gap = 1
		}
		bar = left + strings.Repeat(" ", gap) + rightRendered
	}
	return lipgloss.NewStyle().
		Background(Palette.Bg2).
		Width(width).
		Render(bar)
}

// renderTabStrip returns the tab strip using a TabBar widget.
func renderTabStrip(active ViewID, width int) string {
	return buildTabBar(active, width).View()
}

// renderBreadcrumb returns a single-row breadcrumb matching chrome.jsx Crumb.
// extras are additional path segments contributed by the active screen
// (e.g. selected row label) — last segment is highlighted as the "here" tag.
//
// Examples:
//
//	dashboard
//	licenses · filter:active · alice@research
//	identities · rshell-windows-amd64.bin
func renderBreadcrumb(active ViewID, licFilter licenseFilter, extras []string, width int) string {
	parts := []string{Dim.Render(string(active))}
	if active == ViewLicenses && licFilter != licFilterAll {
		parts = append(parts, Dim.Render("filter:"+licFilter.String()))
	}
	for i, e := range extras {
		if e == "" {
			continue
		}
		if i == len(extras)-1 {
			parts = append(parts, GlowMagent.Render(e))
		} else {
			parts = append(parts, Dim.Render(e))
		}
	}
	crumbText := strings.Join(parts, Mute.Render(" ▸ "))
	// lipgloss.Width sets the cell budget but DOES NOT TRUNCATE — when
	// crumbText is wider than (width - 2*padding), lipgloss soft-wraps to a
	// second row. That extra row pushes every subsequent screen element down
	// by 1, which is the silent cause of the "I can't see the bottom of the
	// detail box" complaint on narrow terminals. Pre-truncate to the cell
	// budget so the breadcrumb is guaranteed to render on a single line.
	const padding = 2 // 1 cell each side from Padding(0, 1)
	budget := width - padding
	if budget < 1 {
		budget = 1
	}
	if lipgloss.Width(crumbText) > budget {
		crumbText = truncate(crumbText, budget)
	}
	return lipgloss.NewStyle().
		Background(Palette.Bg1).
		Foreground(Palette.FgDim).
		Width(width).
		Padding(0, 1).
		Render(crumbText)
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

// ScreenWithHints is an optional interface that screens implement to supply
// their own key-hint slice for the bottom status bar.  The slice must have an
// even number of elements arranged as alternating key / description pairs,
// matching the format accepted by renderStatusBar.
//
// When the active screen implements ScreenWithHints, viewReady uses its hints
// instead of the global default set.
type ScreenWithHints interface {
	Hints() []string
}

// ScreenWithCrumb is optional — screens that have a selected-row or sub-mode
// can return extra segments to surface in the breadcrumb (e.g. the licence
// subject, the identity filename, etc.).
type ScreenWithCrumb interface {
	CrumbExtras() []string
}
