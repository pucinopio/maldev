package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	licenseent "github.com/oioio-space/maldev/internal/manager/store/ent/license"
)

// clipboardWriteAll is the clipboard write function. Tests swap it to spy on
// what gets written without touching the real system clipboard.
var clipboardWriteAll = func(s string) error { return clipboard.WriteAll(s) }

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
	svc                *service.Services
	rows               []*ent.License
	err                error
	filter             licenseFilter
	detailTab          int               // 0=Ident, 1=Bind, 2=PEM, 3=Audit, 4=Chain
	detailAuditRows    []*ent.AuditEvent // lazy-loaded when A tab opens
	detailAuditLoading bool              // true between tab-open and loaded msg
	detailAuditErr     error             // surfaced in renderDetailAudit when the load fails
	search             textinput.Model
	table              table.Model
	// pemViewport scrolls the raw PEM text in the PEM detail tab.
	// ↑/↓ are routed to it when detailTab == 2.
	pemViewport viewport.Model
	detail      bool
	width       int
	hgt         int
	titleHints  *titleHintRow
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

	// Default detail panel open (prototype shows it always-visible);
	// the operator can collapse it with [d].
	return licensesModel{
		svc:         svc,
		table:       t,
		search:      ti,
		detail:      true,
		titleHints:  &titleHintRow{},
		pemViewport: viewport.New(80, 10),
	}
}

// stretchLastColumn / emptyTableHint live in layout.go — shared by every
// list screen.

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
		m.pemViewport.Width = BoxedInner(msg.Width)
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

	case licensesSetFilterMsg:
		// Cross-screen entry from a dashboard hotkey (a/e/w/u).
		m.filter = msg.f
		m.rebuildTable()
		return m, nil

	case licensesFocusSearchMsg:
		// Cross-screen entry from dashboard '/' shortcut.
		m.search.Focus()
		return m, textinput.Blink

	case licenseDetailTabClickMsg:
		m.detailTab = msg.tab
		// Audit tab loads lazily — same path as the keyboard 'A' shortcut.
		if msg.tab == 3 {
			if row := m.selectedRow(); row != nil {
				m.detailAuditRows = nil
				m.detailAuditLoading = true
				return m, loadLicenseAuditCmd(m.svc, row)
			}
		}
		return m, nil

	case licenseAuditLoadedMsg:
		m.detailAuditRows = msg.rows
		m.detailAuditErr = msg.err
		m.detailAuditLoading = false
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
			if row := m.selectedRow(); row != nil {
				m.pemViewport.SetContent(string(row.Pem))
				m.pemViewport.GotoTop()
			}
			return m, nil
		case "A":
			m.detailTab = 3
			m.detailAuditRows = nil
			m.detailAuditLoading = true
			return m, loadLicenseAuditCmd(m.svc, m.selectedRow())
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
			if err := clipboardWriteAll(string(row.Pem)); err != nil {
				return m, func() tea.Msg {
					return pushOverlayMsg{newErrorOverlay("Clipboard Error", err.Error())}
				}
			}
			return m, nil

		case "e":
			// Re-issue the selected licence: push a confirm overlay.
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			sub := fmt.Sprintf("Re-émettre la licence pour %q?\nUne nouvelle licence sera créée avec les mêmes bindings.", row.Subject)
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay("license-reissue", "Re-émettre la licence", sub, "re-émettre", "annuler", false)}
			}

		case "r":
			// When the Audit detail tab is active, 'r' refreshes the audit list
			// for the selected row rather than falling through to the global
			// dashboard refresh that chrome.go handles.
			if m.detail && m.detailTab == 3 {
				row := m.selectedRow()
				if row != nil {
					m.detailAuditRows = nil
					m.detailAuditLoading = true
					return m, loadLicenseAuditCmd(m.svc, row)
				}
			}
		}
	}

	// PEM tab arrow-key scroll: when detail is open on the PEM tab (2), up/down
	// are intercepted here so the table below doesn't consume them as cursor moves.
	if msg, ok := msg.(tea.KeyMsg); ok && m.detail && m.detailTab == 2 {
		switch msg.Type {
		case tea.KeyUp:
			if m.pemViewport.Height > 0 {
				m.pemViewport.LineUp(1)
			}
			return m, nil
		case tea.KeyDown:
			if m.pemViewport.Height > 0 {
				m.pemViewport.LineDown(1)
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

	// Fixed overhead = ChromeRows + box vertical frame (border above/below).
	_, boxV := BoxFrame()
	tableH := clampTableHeight(m.hgt-ChromeRows-boxV, m.detail, len(rows) == 0)

	m.table.SetRows(rows)
	m.table.SetHeight(tableH)
	stretchLastColumn(&m.table, BoxedInner(m.width))
}

func (m licensesModel) View() string {
	// Search bar — when focused or non-empty, show the input; otherwise hint.
	var searchInput string
	if m.search.Focused() {
		searchInput = m.search.View()
	} else if m.search.Value() != "" {
		searchInput = Dim.Render("search: ") + Base.Render(m.search.Value()) +
			" " + Dim.Render("(/ to edit, esc to clear)")
	} else {
		searchInput = Dim.Render("/ rechercher dans subject…")
	}
	// Count + chips on the same row as the search (prototype layout).
	count := fmt.Sprintf("%d/%d", len(m.rows), len(m.rows))
	chips := m.renderFilterBar()
	// Lay out: search | count | chips, with search flex-growing.
	rowW := BoxedInner(m.width)
	chipsW := lipgloss.Width(chips)
	countW := lipgloss.Width(count)
	searchW := rowW - chipsW - countW - 4
	if searchW < 20 {
		searchW = 20
	}
	searchSegment := lipgloss.NewStyle().Width(searchW).Render(searchInput)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		" ", searchSegment, "  ", Dim.Render(count), "  ", chips)

	// Titled box wrapping the table. [↑↓] is informational (keyboard nav)
	// and not exposed as a clickable target — its synthesised KeyMsg would
	// move the cursor on whatever has focus, which isn't the intent.
	titleLabel := fmt.Sprintf("Licences (%d)", len(m.rows))
	title := titleBar(m.titleHints, titleLabel, []titleHint{
		{Key: "↑↓", Label: " nav ", Cmd: func() tea.Cmd { return nil }},
		{Key: "d", Label: " détail ", Cmd: keyCmd("d")},
		{Key: "n", Label: " nouvelle ", Cmd: keyCmd("n")},
		{Key: "x", Label: " révoquer ", Cmd: keyCmd("x")},
		{Key: "e", Label: " re-émettre", Cmd: keyCmd("e")},
	}, 0, BoxedInner(m.width))
	// Title Y = TopChromeRows + blank + topRowH + blank + box border. topRow
	// renders ~3 lines because the filter chips have a bordered pill style;
	// measure live so a narrower terminal that wraps the row stays accurate.
	topRowH := lipgloss.Height(topRow)
	m.titleHints.SetY(TopChromeRows + 1 + topRowH + 1 + 1)
	tableBody := m.table.View()
	if h := emptyTableHint(len(m.rows), m.width, "aucune licence — n pour en émettre une, ? pour l'aide"); h != "" {
		tableBody = lipgloss.JoinVertical(lipgloss.Left, tableBody, "", h)
	}
	boxed := BoxStyle.Width(BoxedWidth(m.width)).Render(title + "\n" + tableBody)

	body := lipgloss.JoinVertical(lipgloss.Left, "", topRow, "", boxed)

	if m.detail {
		body = lipgloss.JoinVertical(lipgloss.Left, body, m.renderDetail())
	}

	if m.err != nil {
		body = GlowRed.Render("Error: "+m.err.Error()) + "\n" + body
	}

	return body
}

// OnClick handles mouse clicks on the licenses screen. Title bar hint chips
// take priority, then filter chip pills (3 rows tall under the search row),
// then the table rows + detail tab strip. Chip-bar Y is derived from
// TopChromeRows so it follows any future chrome resize.
func (m licensesModel) OnClick(x, y, _ int) tea.Cmd {
	if cmd := m.titleHints.hit(x, y); cmd != nil {
		return cmd
	}
	const chipBarTopY = TopChromeRows + 1 // chrome rows + 1 leading blank
	if y >= chipBarTopY && y <= chipBarTopY+2 {
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

	// Table rows: header sits one line below the box title row recorded by
	// View() — same convention as every other list screen via titleHints.
	// Data rows start at headerY + 1; box bottom border at headerY + 1 + table.Height().
	tableHeaderY := m.titleHints.y + 1
	tableEndY := tableHeaderY + m.table.Height() + 1 // +1 for the box bottom border
	if y > tableHeaderY && y < tableEndY {
		row := y - tableHeaderY - 1
		if row >= 0 && row < len(m.rows) {
			target := row
			return func() tea.Msg { return tableSelectRowMsg{row: target} }
		}
		return nil
	}
	// Detail [I/B/P/A/C] tab strip: detail box renders BELOW the table box.
	//   tableEndY     box bottom border ─
	//   tableEndY + 1 detail box top border ─
	//   tableEndY + 2 detail title row ("Détail · lic:… · subject")
	//   tableEndY + 3 tab strip ("[I]dent  [B]ind  [P]EM  [A]udit  [C]haîne")
	if m.detail && m.selectedRow() != nil {
		tabStripY := tableEndY + 3
		if y == tabStripY {
			// Layout mirrors renderDetail's tabStrip: "[I]dent  [B]ind  [P]EM  [A]udit  [C]haîne"
			// Each cell = 3 (key) + label + 2 (spacing). Walk by widths.
			cursor := 0
			cells := []struct {
				tab   int
				label string
			}{
				{0, "[I]dent"}, {1, "[B]ind"}, {2, "[P]EM"}, {3, "[A]udit"}, {4, "[C]haîne"},
			}
			for _, c := range cells {
				w := lipgloss.Width(c.label) + 2 // 2-cell gap between cells
				if x >= cursor && x < cursor+w {
					target := c.tab
					return func() tea.Msg { return licenseDetailTabClickMsg{tab: target} }
				}
				cursor += w
			}
		}
	}
	return nil
}

// licenseDetailTabClickMsg is dispatched when the operator clicks the
// [I/B/P/A/C] tab strip in the license detail panel.
type licenseDetailTabClickMsg struct{ tab int }

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
	bordered := lipgloss.NewStyle().PaddingLeft(1).Render(lipgloss.JoinHorizontal(lipgloss.Top, parts...))
	// Compact fallback for narrow terminals: bordered pills are 3 rows tall
	// and break alignment when the topRow (search + count + chips) overflows
	// m.width — lipgloss wraps each row independently → chip rows split
	// across 4 lines. The topRow reserves ~30 cells for search+count, so
	// the chip budget is roughly m.width - 35.
	const topRowReserve = 35
	if m.width > 0 && lipgloss.Width(bordered) > m.width-topRowReserve {
		var flat []string
		for _, f := range filters {
			seg := f.String()
			if f == m.filter {
				seg = GlowGreen.Render(seg)
			} else {
				seg = Mute.Render(seg)
			}
			flat = append(flat, seg)
		}
		return " " + strings.Join(flat, Mute.Render(" · "))
	}
	return bordered
}

func (m licensesModel) renderDetail() string {
	row := m.selectedRow()
	if row == nil {
		title := Dim.Render("Détail licence")
		hint := Dim.Render("aucune sélection — émets une licence avec ") + HintKey.Render("[n]") +
			Dim.Render(" ou sélectionne une ligne pour voir les onglets ") +
			HintKey.Render("[I/B/P/A/C]")
		return BoxStyle.Width(BoxedWidth(m.width)).Render(
			lipgloss.JoinVertical(lipgloss.Left, title, "", hint),
		)
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
	headerW := BoxedInner(m.width)
	gap := headerW - lipgloss.Width(tabStrip) - lipgloss.Width(actions)
	if gap < 1 {
		gap = 1
	}
	header := title + "\n" + tabStrip + strings.Repeat(" ", gap) + actions

	body := m.renderDetailBody(row)

	return BoxStyle.Width(BoxedWidth(m.width)).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, "", body),
	)
}

// licStatusPill renders a flat, one-line coloured status tag for the
// Licenses detail panel. The bordered Pill* styles render on 3 rows which
// breaks kvRow's single-line baseline (the key label would align with the
// pill's top border instead of the centred text).
func licStatusPill(s licenseent.Status) string {
	switch s {
	case licenseent.StatusActive:
		return GlowGreen.Render("● ACTIVE")
	case licenseent.StatusRevoked:
		return GlowRed.Render("● REVOKED")
	case licenseent.StatusExpired:
		return Mute.Bold(true).Render("● EXPIRED")
	case licenseent.StatusSuperseded:
		return GlowViolet.Render("● SUPERSEDED")
	default:
		if s == "expiring" {
			return GlowYellow.Render("● EXPIRING")
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
		return m.renderDetailBindings(row)
	case 2:
		return m.renderDetailPEM(row)
	case 3:
		return m.renderDetailAudit(row)
	case 4:
		return m.renderDetailChain(row)
	}
	statusPill := licStatusPill(row.Status)

	// valueW is the cell budget for the right-hand value column (box-inner
	// area minus the 14-cell label gutter).
	const labelGutter = 14
	valueW := BoxedInner(m.width) - labelGutter
	if valueW < 12 {
		valueW = 12
	}

	// Validity health bar: 100 % at issue, 0 % at expiry.
	barW := valueW
	if barW < 8 {
		barW = 8
	}
	span := row.NotAfter.Sub(row.NotBefore).Seconds()
	remaining := time.Until(row.NotAfter).Seconds()
	pct := 0.0
	if span > 0 {
		pct = remaining / span
	}
	bar := renderHealthBar(barW, pct)

	return lipgloss.JoinVertical(lipgloss.Left,
		kvRow("status", statusPill, 14),
		kvRow("subject", truncate(row.Subject, valueW), 14),
		kvRow("issuer", truncate(row.IssuerName, valueW), 14),
		kvRow("audience", truncate(strings.Join(row.Audience, ", "), valueW), 14),
		kvRow("features", truncate(strings.Join(row.Features, ", "), valueW), 14),
		kvRow("not-before", row.NotBefore.Format("2006-01-02"), 14),
		kvRow("not-after", row.NotAfter.Format("2006-01-02"), 14),
		kvRow("validity", bar, 14),
		kvRow("uuid", GlowCyan.Render(truncate(row.LicenseUUID, valueW)), 14),
	)
}

// truncate clips s to maxW visible cells, appending "…" when it had to cut.
// Operates on display width via lipgloss.Width so ANSI-wrapped strings still
// measure correctly. Pure ASCII assumed for slicing — fine for the values
// fed here (subject/issuer/audience/UUID are all single-byte char).
func truncate(s string, maxW int) string {
	if lipgloss.Width(s) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "…"
	}
	return s[:maxW-1] + "…"
}

// renderDetailBindings shows the licence's bindings (machine fingerprint,
// password preset, TOTP, k/v) and pinning fields. Reads from BindingsMeta +
// IdentitySha256 + BinarySha256 + PayloadKind on the row.
func (m licensesModel) renderDetailBindings(row *ent.License) string {
	var lines []string
	lines = append(lines, GlowCyan.Render("Bindings"))
	if len(row.BindingsMeta) == 0 {
		lines = append(lines, Dim.Render("  (no bindings)"))
	} else {
		for k, v := range row.BindingsMeta {
			lines = append(lines, kvRow(k, fmt.Sprintf("%v", v), 14))
		}
	}
	lines = append(lines, "", GlowCyan.Render("Pinning"))
	if row.IdentitySha256 != "" {
		lines = append(lines, kvRow("identity", GlowCyan.Render(row.IdentitySha256[:min(16, len(row.IdentitySha256))]+"…"), 14))
	} else {
		lines = append(lines, kvRow("identity", Dim.Render("—"), 14))
	}
	if row.BinarySha256 != "" {
		lines = append(lines, kvRow("binary", GlowCyan.Render(row.BinarySha256[:min(16, len(row.BinarySha256))]+"…"), 14))
	} else {
		lines = append(lines, kvRow("binary", Dim.Render("—"), 14))
	}
	lines = append(lines, "", GlowCyan.Render("Sealed payload"))
	lines = append(lines, kvRow("kind", string(row.PayloadKind), 14))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderDetailPEM displays the licence PEM text in a scrollable viewport.
// ↑/↓ are routed to pemViewport by Update() when this tab is active.
func (m licensesModel) renderDetailPEM(row *ent.License) string {
	hint := HintKey.Render("[c]") + Dim.Render(" copier · ") +
		HintKey.Render("↑↓") + Dim.Render(" scroll")
	header := GlowCyan.Render("PEM signé") + "  " + hint

	pem := string(row.Pem)
	if pem == "" {
		return header + "\n" + Dim.Render("(PEM absent côté store — vérifie l'intégrité de la base ou re-émets la licence)")
	}

	// Keep the viewport content in sync with the selected row. The viewport is
	// loaded on tab-open ('P' key) and here as a fallback for row switches.
	if m.pemViewport.Height == 0 {
		// Viewport not yet sized (e.g. no WindowSizeMsg received yet) — render plain.
		return header + "\n" + Base.Render(pem)
	}
	// Re-set content if it has drifted (row selection changed while PEM tab open).
	if m.pemViewport.TotalLineCount() == 0 {
		m.pemViewport.SetContent(Base.Render(pem))
	}
	return header + "\n" + m.pemViewport.View()
}

// renderDetailAudit lists audit events for the selected licence. The list is
// populated on tab-open by loadLicenseAuditCmd; until that completes we show
// a one-line "loading" hint.
func (m licensesModel) renderDetailAudit(row *ent.License) string {
	hint := HintKey.Render("[r]") + Dim.Render(" refresh")
	header := GlowCyan.Render("Audit · "+row.LicenseUUID[:8]+"…") + "  " + hint
	if m.detailAuditLoading {
		return header + "\n" + Dim.Render("  chargement de l'historique…")
	}
	if m.detailAuditErr != nil {
		return header + "\n" + GlowRed.Render("  erreur : "+m.detailAuditErr.Error())
	}
	if len(m.detailAuditRows) == 0 {
		return header + "\n" + Dim.Render("  (aucun évènement pour cette licence)")
	}
	lines := []string{header}
	for _, e := range m.detailAuditRows {
		ts := e.CreatedAt.Format("2006-01-02 15:04:05")
		lines = append(lines, "  "+
			Mute.Render(ts)+"  "+
			GlowCyan.Render(e.Kind)+"  "+
			Dim.Render(e.Actor))
	}
	return strings.Join(lines, "\n")
}

// licenseAuditLoadedMsg carries the audit history for the selected licence.
type licenseAuditLoadedMsg struct {
	rows []*ent.AuditEvent
	err  error
}

// loadLicenseAuditCmd fetches the audit events for one licence via
// svc.Audit.ListForTarget. Returns a no-op when svc or row is nil.
func loadLicenseAuditCmd(svc *service.Services, row *ent.License) tea.Cmd {
	if svc == nil || row == nil {
		return nil
	}
	licID := row.ID
	return func() tea.Msg {
		rows, err := svc.Audit.ListForTarget(context.Background(),
			service.Target{Kind: "License", ID: licID.String()}, 50)
		return licenseAuditLoadedMsg{rows: rows, err: err}
	}
}

// renderDetailChain renders the parent → here → successors lineage.
// The ent schema does not model parent/successor links yet; this render
// surfaces what IS known (uuid, subject, status) inside a structured
// placeholder so the tab is informative rather than an empty stub.
func (m licensesModel) renderDetailChain(row *ent.License) string {
	header := GlowCyan.Render("Chaîne de succession") + "  " +
		Mute.Render("(liens parent/successeur — à venir)")

	// Skeleton table: PARENT / THIS / SUCCESSORS rows.
	// labelW matches the 14-cell gutter used by renderDetailBody's kvRows.
	const labelW = 14
	divider := Dim.Render(strings.Repeat("─", BoxedInner(m.width)))
	parentRow := kvRow("parent", Mute.Render("aucun (racine)"), labelW)
	thisRow := kvRow("cette lic.", GlowMagent.Render(row.LicenseUUID[:8]+"…")+" "+Dim.Render(row.Subject), labelW)
	succRow := kvRow("successeurs", Mute.Render("aucun enregistré"), labelW)

	lines := []string{header, "", parentRow, divider, thisRow, divider, succRow}

	if row.Status == licenseent.StatusSuperseded {
		lines = append(lines, "",
			GlowYellow.Render("cette licence est SUPERSEDED — re-émettre le successeur le plus récent"))
	}
	lines = append(lines, "",
		Dim.Render("  Liens parent/successeur seront peuplés une fois le champ successor_id ajouté au schéma ent."))

	return strings.Join(lines, "\n")
}

// handleLicenseReissueConfirm is called when the operator confirms the
// "license-reissue" overlay (D-S16). It calls svc.License.ReIssue and reloads
// the list, surfacing an OK overlay on success or an error overlay on failure.
func (m licensesModel) handleLicenseReissueConfirm(res ConfirmResultMsg) (licensesModel, tea.Cmd) {
	if !res.Confirm {
		return m, nil
	}
	row := m.selectedRow()
	if row == nil || m.svc == nil {
		return m, func() tea.Msg {
			return pushOverlayMsg{NewOKOverlay("Re-émettre", "Re-émission (stub — aucun service ou aucune sélection).")}
		}
	}
	svc := m.svc
	oldID := row.ID
	subject := row.Subject
	return m, func() tea.Msg {
		newLic, err := svc.License.ReIssue(context.Background(), oldID, service.ReIssueOptions{Actor: "operator"})
		if err != nil {
			return pushOverlayMsg{newErrorOverlay("Re-émission échouée", err.Error())}
		}
		return pushOverlayMsg{NewOKOverlay("Re-émission OK",
			fmt.Sprintf("Nouvelle licence pour %q\nUUID: %s\n\nPress 'r' to refresh the list.",
				subject, newLic.Row.LicenseUUID))}
	}
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

