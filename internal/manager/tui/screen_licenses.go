package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	licenseent "github.com/oioio-space/maldev/internal/manager/store/ent/license"
)

// LicensesLoadedMsg carries the result of fetching all licenses.
type LicensesLoadedMsg struct {
	Rows []*ent.License
	Err  error
}

type licenseFilter int

const (
	licFilterAll licenseFilter = iota
	licFilterActive
	licFilterExpiring
	licFilterExpired
	licFilterRevoked
	licFilterSuperseded
	licFilterCount // sentinel
)

func (f licenseFilter) String() string {
	switch f {
	case licFilterAll:
		return "all"
	case licFilterActive:
		return "active"
	case licFilterExpiring:
		return "expiring"
	case licFilterExpired:
		return "expired"
	case licFilterRevoked:
		return "revoked"
	case licFilterSuperseded:
		return "superseded"
	}
	return "all"
}

// licensesModel is the view-model for the Licenses screen.
type licensesModel struct {
	svc    *service.Services
	rows   []*ent.License
	err    error
	filter licenseFilter
	search textinput.Model
	table  table.Model
	detail bool
	width  int
	hgt    int
}

func newLicensesModel(svc *service.Services) licensesModel {
	cols := []table.Column{
		{Title: "STATUS", Width: 11},
		{Title: "SUBJECT", Width: 22},
		{Title: "AUDIENCE", Width: 16},
		{Title: "KEYID", Width: 18},
		{Title: "EXPIRES", Width: 12},
		{Title: "FEATURES", Width: 20},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(false),
		table.WithHeight(15),
		table.WithStyles(licTableStyles()),
	)

	ti := textinput.New()
	ti.Placeholder = "search subject…"
	ti.CharLimit = 100
	ti.Width = 30

	return licensesModel{svc: svc, table: t, search: ti}
}

func licTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		Foreground(Palette.Cyan).
		Bold(true).
		BorderForeground(Palette.Border)
	s.Selected = s.Selected.
		Foreground(Palette.Fg).
		Background(Palette.Bg3).
		Bold(false)
	return s
}

// ListLicensesCmd fetches all licenses from the service.
func ListLicensesCmd(svc *service.Services) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return LicensesLoadedMsg{}
		}
		rows, err := svc.License.List(context.Background(), service.ListFilter{Limit: 500})
		return LicensesLoadedMsg{Rows: rows, Err: err}
	}
}

func (m licensesModel) Init() tea.Cmd {
	return ListLicensesCmd(m.svc)
}

func (m licensesModel) Update(msg tea.Msg) (licensesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.hgt = msg.Height
		m.rebuildTable()
		return m, nil

	case LicensesLoadedMsg:
		m.err = msg.Err
		m.rows = msg.Rows
		m.rebuildTable()
		return m, nil

	case tea.KeyMsg:
		// Search mode absorbs keys.
		if m.search.Focused() {
			switch msg.String() {
			case "esc", "enter":
				m.search.Blur()
				m.rebuildTable()
				return m, nil
			}
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.rebuildTable()
			return m, cmd
		}

		switch msg.String() {
		case "/":
			m.search.Focus()
			return m, textinput.Blink

		case "f":
			m.filter = (m.filter + 1) % licFilterCount
			m.rebuildTable()
			return m, nil

		case "d", "enter":
			m.detail = !m.detail
			return m, nil

		case "n":
			// Wizard not yet implemented — Phase 3.
			return m, func() tea.Msg {
				return pushOverlayMsg{newErrorOverlay("Phase 3", "License wizard not yet implemented.\nComing in Phase 3.")}
			}

		case "x":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return pushOverlayMsg{newRevokeOverlay(row.ID, row.Subject)}
			}

		case "c":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			if err := clipboard.WriteAll(string(row.Pem)); err != nil {
				return m, func() tea.Msg {
					return pushOverlayMsg{newErrorOverlay("Clipboard Error", err.Error())}
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// pushOverlayMsg asks app.go to push an overlay onto the stack.
type pushOverlayMsg struct{ overlay Overlay }

func (m *licensesModel) selectedRow() *ent.License {
	visible := m.visibleRows()
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(visible) {
		return nil
	}
	return visible[idx]
}

func (m *licensesModel) visibleRows() []*ent.License {
	query := strings.ToLower(m.search.Value())
	now := time.Now()
	expiringSoon := now.Add(30 * 24 * time.Hour)

	var out []*ent.License
	for _, r := range m.rows {
		// Filter by chip.
		switch m.filter {
		case licFilterActive:
			if r.Status != licenseent.StatusActive {
				continue
			}
		case licFilterExpiring:
			if r.Status != licenseent.StatusActive || r.NotAfter.After(expiringSoon) {
				continue
			}
		case licFilterExpired:
			if !r.NotAfter.Before(now) {
				continue
			}
		case licFilterRevoked:
			if r.Status != licenseent.StatusRevoked {
				continue
			}
		case licFilterSuperseded:
			if r.Status != licenseent.StatusSuperseded {
				continue
			}
		}
		// Search filter.
		if query != "" && !strings.Contains(strings.ToLower(r.Subject), query) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (m *licensesModel) rebuildTable() {
	visible := m.visibleRows()
	rows := make([]table.Row, 0, len(visible))
	now := time.Now()
	for _, r := range visible {
		status := string(r.Status)
		expires := r.NotAfter.Format("2006-01-02")
		if r.NotAfter.Before(now) && r.Status == licenseent.StatusActive {
			status = "expired"
		}
		audience := strings.Join(r.Audience, ",")
		if len(audience) > 14 {
			audience = audience[:13] + "…"
		}
		features := strings.Join(r.Features, ",")
		if len(features) > 18 {
			features = features[:17] + "…"
		}
		rows = append(rows, table.Row{
			status, r.Subject, audience, r.IssuerName, expires, features,
		})
	}

	// Compute table height: reserve space for header, filter bar, detail panel, status bar.
	tableH := m.hgt - 6 // title + tabs + filter + status
	if m.detail {
		tableH = tableH / 2
	}
	if tableH < 3 {
		tableH = 3
	}

	m.table.SetRows(rows)
	m.table.SetHeight(tableH)
}

func (m licensesModel) View() string {
	// Filter chips.
	chips := m.renderFilterBar()

	// Search bar.
	var searchBar string
	if m.search.Focused() {
		searchBar = " " + m.search.View()
	} else if m.search.Value() != "" {
		searchBar = " " + Dim.Render("search: ") + Base.Render(m.search.Value()) +
			" " + Dim.Render("(/ to edit, esc to clear)")
	} else {
		searchBar = " " + Dim.Render("/ to search")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		chips,
		searchBar,
		m.table.View(),
	)

	if m.detail {
		body = lipgloss.JoinVertical(lipgloss.Left, body, m.renderDetail())
	}

	if m.err != nil {
		body = GlowRed.Render("Error: "+m.err.Error()) + "\n" + body
	}

	hints := []string{
		"f", "filter",
		"/", "search",
		"d/↵", "detail",
		"n", "new",
		"x", "revoke",
		"c", "copy PEM",
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, renderStatusBar(hints, m.width))
}

func (m licensesModel) renderFilterBar() string {
	filters := []licenseFilter{licFilterAll, licFilterActive, licFilterExpiring, licFilterExpired, licFilterRevoked, licFilterSuperseded}
	var parts []string
	for _, f := range filters {
		label := f.String()
		if f == m.filter {
			parts = append(parts, PillActive.Render(label))
		} else {
			parts = append(parts, PillOff.Render(label))
		}
	}
	return " " + strings.Join(parts, " ")
}

func (m licensesModel) renderDetail() string {
	row := m.selectedRow()
	if row == nil {
		return Dim.Render("  no selection")
	}
	lines := []string{
		fmt.Sprintf("  %-18s %s", "status:", string(row.Status)),
		fmt.Sprintf("  %-18s %s", "subject:", row.Subject),
		fmt.Sprintf("  %-18s %s", "issuer:", row.IssuerName),
		fmt.Sprintf("  %-18s %s", "audience:", strings.Join(row.Audience, ", ")),
		fmt.Sprintf("  %-18s %s", "features:", strings.Join(row.Features, ", ")),
		fmt.Sprintf("  %-18s %s", "not-before:", row.NotBefore.Format(time.RFC3339)),
		fmt.Sprintf("  %-18s %s", "not-after:", row.NotAfter.Format(time.RFC3339)),
		fmt.Sprintf("  %-18s %s", "identity-sha256:", row.IdentitySha256),
		fmt.Sprintf("  %-18s %s", "binary-sha256:", row.BinarySha256),
		fmt.Sprintf("  %-18s %s", "license-uuid:", row.LicenseUUID),
		fmt.Sprintf("  %-18s %s", "db-id:", row.ID.String()),
	}
	detail := strings.Join(lines, "\n")
	return BoxStyle.Width(m.width - 2).Render(detail)
}

// handleRevokeResult applies the result of revokeOverlay back to this screen.
func (m licensesModel) handleRevokeResult(res RevokeConfirmedMsg) (licensesModel, tea.Cmd) {
	if m.svc == nil {
		return m, nil
	}
	id := res.LicenseID
	reason := res.Reason
	return m, func() tea.Msg {
		err := m.svc.Revoke.Revoke(context.Background(), id, reason, "operator")
		if err != nil {
			return LicensesLoadedMsg{Err: err}
		}
		rows, err := m.svc.License.List(context.Background(), service.ListFilter{Limit: 500})
		return LicensesLoadedMsg{Rows: rows, Err: err}
	}
}

