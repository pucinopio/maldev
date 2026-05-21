package tui

import (
	"context"
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
	filter    licenseFilter
	detailTab int // 0=Ident, 1=Bind, 2=PEM, 3=Audit, 4=Chain
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

// stretchLastColumn resizes a table's trailing column so the row spans the
// available screen width. The bubbles/table package only highlights cells, so
// without this the selected-row background only covers the natural column sum
// (~50% of a 144-cell terminal). Safe to call when width == 0 (no-op).
func stretchLastColumn(t *table.Model, width int) {
	if width <= 0 {
		return
	}
	cols := t.Columns()
	if len(cols) == 0 {
		return
	}
	fixed := 0
	for i := 0; i < len(cols)-1; i++ {
		fixed += cols[i].Width
	}
	overhead := 2*len(cols) + 2 // padding (1 per col) + outer borders
	last := width - fixed - overhead
	if last < cols[len(cols)-1].Width {
		last = cols[len(cols)-1].Width
	}
	cols[len(cols)-1].Width = last
	t.SetColumns(cols)
}

// emptyTableHint returns a centered muted line shown under a table that has
// no rows, hinting at the keybind that creates one. Returns "" when rows > 0.
func emptyTableHint(rows int, width int, message string) string {
	if rows > 0 || width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Foreground(Palette.FgMute).
		Render(message)
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

	case licenseFilterClickMsg:
		m.filter = msg.f
		m.rebuildTable()
		return m, nil

	case tableSelectRowMsg:
		m.table.SetCursor(msg.row)
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

		case "I":
			m.detailTab = 0
			return m, nil
		case "B":
			m.detailTab = 1
			return m, nil
		case "P":
			m.detailTab = 2
			return m, nil
		case "A":
			m.detailTab = 3
			return m, nil
		case "C":
			m.detailTab = 4
			return m, nil

		case "n":
			return m, openWizardCmd(m.svc)

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
	if len(rows) == 0 {
		tableH = 1
	}

	m.table.SetRows(rows)
	m.table.SetHeight(tableH)
	stretchLastColumn(&m.table, m.width)
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

	tableView := m.table.View()
	if hint := emptyTableHint(len(m.rows), m.width, "aucune licence — n pour en émettre une, ? pour l'aide"); hint != "" {
		tableView = lipgloss.JoinVertical(lipgloss.Left, tableView, "", hint)
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		chips,
		searchBar,
		tableView,
	)

	if m.detail {
		body = lipgloss.JoinVertical(lipgloss.Left, body, m.renderDetail())
	}

	if m.err != nil {
		body = GlowRed.Render("Error: "+m.err.Error()) + "\n" + body
	}

	// Status bar rendered globally by the root chrome — don't duplicate here.
	return body
}

// OnClick handles mouse clicks on the licenses screen. Filter chip pills span
// Y=4..6 (3-row bordered pills); table header is at Y=7, data rows at Y=8+.
func (m licensesModel) OnClick(x, y, _ int) tea.Cmd {
	if y >= 4 && y <= 6 {
		// Hit-test the filter chip bar. Mirror renderFilterBar layout:
		// 1 PaddingLeft + each pill(label+4 border/padding) + 1 separator space.
		cursor := 1
		for _, f := range []licenseFilter{licFilterAll, licFilterActive, licFilterExpiring, licFilterExpired, licFilterRevoked, licFilterSuperseded} {
			w := lipgloss.Width(f.String()) + 4
			if x >= cursor && x < cursor+w {
				target := f
				return func() tea.Msg { return licenseFilterClickMsg{f: target} }
			}
			cursor += w + 1
		}
		return nil
	}

	// Table rows: header at Y=7, data at Y=8+.
	const tableHeaderY = 7
	if y > tableHeaderY {
		row := y - tableHeaderY - 1
		if row >= 0 && row < len(m.rows) {
			target := row
			return func() tea.Msg { return tableSelectRowMsg{row: target} }
		}
	}
	return nil
}

// licenseFilterClickMsg is dispatched from OnClick when the operator clicks a
// filter chip; the licenses model handles it in Update.
type licenseFilterClickMsg struct{ f licenseFilter }

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
		parts = append(parts, " ")
	}
	// PaddingLeft applies to every line of the multi-line chip block; a leading
	// space concat would only indent the first row (top border).
	return lipgloss.NewStyle().PaddingLeft(1).Render(lipgloss.JoinHorizontal(lipgloss.Top, parts...))
}

func (m licensesModel) renderDetail() string {
	row := m.selectedRow()
	if row == nil {
		return Dim.Render("  no selection")
	}

	// Header: license ID + subject + tab strip + action hints.
	// Matches licenses.jsx detail box title + tab strip (line 76-135).
	licID := row.LicenseUUID
	if len(licID) > 12 {
		licID = "lic:" + licID[:8] + "…"
	}
	title := Dim.Render("Détail · ") +
		GlowMagent.Render(licID) + Dim.Render(" · ") +
		Base.Render(row.Subject)

	tabStrip := strings.Join([]string{
		HintKey.Render("[I]") + Dim.Render("dent"),
		HintKey.Render("[B]") + Dim.Render("ind"),
		HintKey.Render("[P]") + Dim.Render("EM"),
		HintKey.Render("[A]") + Dim.Render("udit"),
		HintKey.Render("[C]") + Dim.Render("haîne"),
	}, "  ")
	actions := Dim.Render("[d] replier  [c] PEM  [e] re-émettre  [x] révoquer")
	headerW := m.width - 4
	gap := headerW - lipgloss.Width(tabStrip) - lipgloss.Width(actions)
	if gap < 1 {
		gap = 1
	}
	header := title + "\n" + tabStrip + strings.Repeat(" ", gap) + actions

	body := m.renderDetailBody(row)

	return BoxStyle.Width(m.width - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, "", body),
	)
}

// licStatusPill renders a colored status pill for the Licenses detail panel.
func licStatusPill(s licenseent.Status) string {
	switch s {
	case licenseent.StatusActive:
		return PillActive.Render("ACTIVE")
	case licenseent.StatusRevoked:
		return PillRevoked.Render("REVOKED")
	case licenseent.StatusExpired:
		return PillOff.Render("EXPIRED")
	case licenseent.StatusSuperseded:
		return PillSuperseded.Render("SUPERSEDED")
	default:
		// "expiring" is a computed status not stored in ent; render with yellow pill.
		if s == "expiring" {
			return PillExpiring.Render("EXPIRING")
		}
		return Dim.Render(string(s))
	}
}

// handleRevokeResult applies the result of revokeOverlay back to this screen.
// renderDetailBody dispatches to the per-tab body renderer based on detailTab.
// Identité (0), Bindings (1), PEM (2), Audit (3), Chaîne (4).
func (m licensesModel) renderDetailBody(row *ent.License) string {
	switch m.detailTab {
	case 1:
		return Dim.Render("  bindings — machine fingerprint, TOTP, IP allowlist…") + "\n" +
			Dim.Render("  (vue Bindings à implémenter)")
	case 2:
		return Dim.Render("  PEM blob — appuie sur [c] pour copier dans le presse-papier") + "\n" +
			Base.Render("  -----BEGIN LICENSE-----\n  ...\n  -----END LICENSE-----")
	case 3:
		return Dim.Render("  historique audit pour cette licence (timestamp, kind, actor)") + "\n" +
			Dim.Render("  (vue Audit chronologique à implémenter)")
	case 4:
		return Dim.Render("  chaîne de succession — re-émissions, parent, supersession") + "\n" +
			Dim.Render("  (graphe à implémenter)")
	}
	// Default: Identité tab.
	statusPill := licStatusPill(row.Status)
	return lipgloss.JoinVertical(lipgloss.Left,
		kvRow("status", statusPill, 14),
		kvRow("subject", row.Subject, 14),
		kvRow("issuer", row.IssuerName, 14),
		kvRow("audience", strings.Join(row.Audience, ", "), 14),
		kvRow("features", strings.Join(row.Features, ", "), 14),
		kvRow("not-before", row.NotBefore.Format("2006-01-02"), 14),
		kvRow("not-after", row.NotAfter.Format("2006-01-02"), 14),
		kvRow("uuid", GlowCyan.Render(row.LicenseUUID), 14),
	)
}

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

