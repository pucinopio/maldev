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
	filter         licenseFilter
	detailTab         int               // 0=Ident, 1=Bind, 2=PEM, 3=Audit, 4=Chain
	detailAuditRows   []*ent.AuditEvent // lazy-loaded when A tab opens
	detailAuditLoading bool             // true between tab-open and loaded msg
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

	// Default detail panel open (prototype shows it always-visible);
	// the operator can collapse it with [d].
	return licensesModel{svc: svc, table: t, search: ti, detail: true}
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

// titledBoxRow lays out a box title row: cyan-bold label flush-left,
// right-aligned hint, single line. innerW is the available content width
// excluding the bordered frame's own padding.
//
// Mirrors the prototype Box title pattern, e.g.:
//
//	Identities (4)                    [n] créer · [E] export .bin · [R] régénérer
func titledBoxRow(label, hint string, innerW int) string {
	if innerW < 1 {
		innerW = 1
	}
	left := GlowCyan.Render(label)
	gap := innerW - lipgloss.Width(left) - lipgloss.Width(hint)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + hint
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

	// Compute table height: chrome (4) + box vertical frame (2) = 6 rows of
	// fixed overhead before the table itself; ChromeRows already includes the
	// status bar, the +2 accounts for the BoxStyle borders above and below.
	_, boxV := BoxFrame()
	tableH := m.hgt - ChromeRows - boxV
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

	// Titled box wrapping the table.
	titleLabel := fmt.Sprintf("Licences (%d)", len(m.rows))
	hint := HintKey.Render("[↑↓]") + Dim.Render(" nav ") +
		Mute.Render("· ") + HintKey.Render("[d]") + Dim.Render(" détail ") +
		Mute.Render("· ") + HintKey.Render("[n]") + Dim.Render(" nouvelle ") +
		Mute.Render("· ") + HintKey.Render("[x]") + Dim.Render(" révoquer ") +
		Mute.Render("· ") + HintKey.Render("[e]") + Dim.Render(" re-émettre")
	title := titledBoxRow(titleLabel, hint, BoxedInner(m.width))
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
	tableEndY := tableHeaderY + m.table.Height() + 1 // +1 for the box bottom border
	if y > tableHeaderY && y < tableEndY {
		row := y - tableHeaderY - 1
		if row >= 0 && row < len(m.rows) {
			target := row
			return func() tea.Msg { return tableSelectRowMsg{row: target} }
		}
		return nil
	}
	// Detail [I/B/P/A/C] tab strip: lives at Y = tableEndY + 2 (top box border
	// at tableEndY+1, then the tab strip is the title row's second line).
	if m.detail && m.selectedRow() != nil {
		tabStripY := tableEndY + 2
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
	return lipgloss.NewStyle().PaddingLeft(1).Render(lipgloss.JoinHorizontal(lipgloss.Top, parts...))
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

// renderDetailPEM displays the licence PEM text + a hint to copy.
func (m licensesModel) renderDetailPEM(row *ent.License) string {
	hint := HintKey.Render("[c]") + Dim.Render(" copier · ") +
		HintKey.Render("↑↓") + Dim.Render(" scroll")
	pem := string(row.Pem)
	if pem == "" {
		pem = Dim.Render("(PEM absent côté store — vérifie l'intégrité de la base ou re-émets la licence)")
	}
	return GlowCyan.Render("PEM signé") + "  " + hint + "\n" + Base.Render(pem)
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
}

// loadLicenseAuditCmd fetches the audit events for one licence via
// svc.Audit.ListForTarget. Returns a no-op when svc or row is nil.
func loadLicenseAuditCmd(svc *service.Services, row *ent.License) tea.Cmd {
	if svc == nil || row == nil {
		return nil
	}
	licID := row.ID
	return func() tea.Msg {
		rows, _ := svc.Audit.ListForTarget(context.Background(),
			service.Target{Kind: "License", ID: licID.String()}, 50)
		return licenseAuditLoadedMsg{rows: rows}
	}
}

// renderDetailChain renders the parent → here → successors lineage. The ent
// schema doesn't model parent/successor links yet, so this is a stub that
// surfaces what we DO know (PayloadKind + status superseded warning).
func (m licensesModel) renderDetailChain(row *ent.License) string {
	var b strings.Builder
	b.WriteString(GlowCyan.Render("Chaîne de succession") + "\n\n")
	b.WriteString(Dim.Render("  parent → ") + Mute.Render("aucun (racine)") + "\n")
	b.WriteString("  " + GlowMagent.Render(row.LicenseUUID[:8]+"…") + " " +
		Dim.Render("("+row.Subject+")") + "\n")
	b.WriteString(Dim.Render("  successeurs → ") + Mute.Render("aucun") + "\n")
	if row.Status == "superseded" {
		b.WriteString("\n" + GlowYellow.Render("⚠ cette licence est SUPERSEDED — re-émets le successeur le plus récent."))
	}
	b.WriteString("\n" + Mute.Render("  (parent/successor links pas encore modélisés dans le schéma ent — stub)"))
	return b.String()
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

