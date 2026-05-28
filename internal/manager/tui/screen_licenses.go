package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	// detailChain is the resolved lineage for the Chain tab (lazy-loaded when C opens).
	detailChain        *service.LicenseChain
	detailChainLoading bool
	detailChainErr     error
	search             textinput.Model
	table              table.Model
	// pemViewport scrolls the raw PEM text in the PEM detail tab.
	// ↑/↓ are routed to it when detailTab == 2.
	pemViewport viewport.Model
	detail      bool
	width       int
	hgt         int
	titleHints  *titleHintRow
	// chipHits records the absolute X-range of each filter chip after the most
	// recent View() so OnClick can hit-test against the geometry that was
	// actually rendered — including the compact-fallback inline layout used on
	// narrow terminals. Pre-2026-05 the click handler reverse-engineered the
	// formula and the matching test mirrored the same broken formula, so neither
	// of them caught the real off-by-tens drift caused by the search/count
	// columns sitting to the left of the chips.
	chipHits *chipHitRow
	// chainHits is populated by renderDetailChain so OnClick can translate
	// a click inside the chain body back to the licence the operator
	// pointed at. Pointer-held so value-receiver renders survive.
	chainHits *chainHitRow
}

// chipHit is one (filter, rendered span) record produced by renderFilterBar.
type chipHit struct {
	f      licenseFilter
	x0, x1 int
}

// chipHitRow stores the chip-bar Y range plus per-chip X-ranges. Like
// titleHintRow it's held by pointer so the populated layout survives the
// value-receiver Update/View round-trip.
type chipHitRow struct {
	y0, y1 int
	hits   []chipHit
}

func (c *chipHitRow) hit(x, y int) (licenseFilter, bool) {
	if c == nil || y < c.y0 || y > c.y1 {
		return 0, false
	}
	for _, h := range c.hits {
		if x >= h.x0 && x < h.x1 {
			return h.f, true
		}
	}
	return 0, false
}

func newLicensesModel(svc *service.Services) licensesModel {
	cols := []table.Column{
		{Title: "STATUS", Width: 11},
		// UUID column: licences are identified by their UUID, so every table
		// listing licences must surface it. Short form (12 chars + ellipsis)
		// keeps the column compact while still distinctive enough for
		// cross-referencing with audit logs and the chain detail.
		{Title: "UUID", Width: 13},
		{Title: "SUBJECT", Width: 22},
		{Title: "AUDIENCE", Width: 16},
		{Title: "KEYID", Width: 18},
		{Title: "EXPIRES", Width: 12},
		{Title: "FEATURES", Width: 20},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
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
		chipHits:    &chipHitRow{},
		chainHits:   &chainHitRow{},
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
		// Viewport height MUST sit inside the detail panel's slice of the
		// remaining screen budget — otherwise the bottom of the box renders
		// beyond row m.hgt-1 and the root chrome's clampToHeight truncates it
		// silently, making any scroll invisible. Reservation breakdown
		// (licences screen with detail open):
		//   ChromeRows(4) + leading blank(1) + topRow(3, bordered chips) +
		//   blank(1) + table-box overhead (top+title+bottom = 3) +
		//   detail-box overhead (top+header+blank+bottom = 4) = 16
		// Then split the remaining vertical space 50/50 between table and
		// detail body, and reserve 1 line inside the detail body for the
		// "PEM signé [c] copier ..." header.
		const layoutReservation = 17
		half := (msg.Height - layoutReservation) / 2
		if half < 4 {
			half = 4
		}
		m.pemViewport.Height = half - 1
		if m.pemViewport.Height < 3 {
			m.pemViewport.Height = 3
		}
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
		// PEM (2) / Audit (3) / Chain (4) load lazily — same paths as the
		// keyboard shortcuts. Without this case 2 a mouse-click on [P] used
		// to show an empty viewport because renderDetailPEM's SetContent
		// fallback ran on a value-receiver and the mutation was discarded.
		switch msg.tab {
		case 2:
			if row := m.selectedRow(); row != nil {
				m.pemViewport.SetContent(Base.Render(string(row.Pem)))
				m.pemViewport.GotoTop()
			}
		case 3:
			if row := m.selectedRow(); row != nil {
				m.detailAuditRows = nil
				m.detailAuditLoading = true
				return m, loadLicenseAuditCmd(m.svc, row)
			}
		case 4:
			if row := m.selectedRow(); row != nil {
				m.detailChain = nil
				m.detailChainLoading = true
				m.detailChainErr = nil
				return m, loadLicenseChainCmd(m.svc, row)
			}
		}
		return m, nil

	case licenseAuditLoadedMsg:
		m.detailAuditRows = msg.rows
		m.detailAuditErr = msg.err
		m.detailAuditLoading = false
		return m, nil

	case licenseChainLoadedMsg:
		m.detailChain = msg.chain
		m.detailChainErr = msg.err
		m.detailChainLoading = false
		return m, nil

	case licenseChainClickMsg:
		// Operator clicked a chain entry — navigate to that licence.
		// Force filter→all so the target row is reachable even if it was
		// hidden by the current filter, then move the cursor and reload
		// the Chain tab for the new selection.
		m.filter = licFilterAll
		m.rebuildTable()
		visible := m.visibleRows()
		for i, r := range visible {
			if r.LicenseUUID == msg.uuid {
				m.table.SetCursor(i)
				m.detail = true
				m.detailTab = 4
				m.detailChain = nil
				m.detailChainLoading = true
				m.detailChainErr = nil
				return m, loadLicenseChainCmd(m.svc, r)
			}
		}
		return m, nil

	case licenseImportPickedMsg:
		svc := m.svc
		path := msg.path
		return m, func() tea.Msg {
			pem, err := os.ReadFile(path)
			if err != nil {
				return pushOverlayMsg{newErrorOverlay("Import — read", err.Error())}
			}
			if svc == nil {
				return pushOverlayMsg{newErrorOverlay("Import", "service indisponible")}
			}
			row, err := svc.License.Import(context.Background(), pem, filepath.Base(path), "operator")
			if err != nil {
				return pushOverlayMsg{newErrorOverlay("Import licence", err.Error())}
			}
			return licenseImportedMsg{row: row}
		}

	case licenseImportedMsg:
		// Trigger reload so the table shows the newly imported row.
		return m, ListLicensesCmd(m.svc)

	case licenseDeletedMsg:
		m.err = msg.err
		if msg.err == nil {
			m.rows = msg.rows
			m.rebuildTable()
			subject := msg.subject
			return m, func() tea.Msg {
				return pushOverlayMsg{NewOKOverlay("Suppression OK",
					fmt.Sprintf("Licence %q supprimée — son UUID est libre, le PEM est réimportable.", subject))}
			}
		}
		return m, nil

	case tableSelectRowMsg:
		m.table.SetCursor(msg.row)
		// Refresh whichever detail tab is open against the newly-selected
		// row. Without this the chain/audit/PEM tab keeps showing the
		// previous licence's data after a mouse click on a different row.
		return m, m.refreshDetailForSelection()

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
				m.pemViewport.SetContent(Base.Render(string(row.Pem)))
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
			// Lazy-load on every open so the chain is fresh after a re-issue.
			m.detailChain = nil
			m.detailChainLoading = true
			m.detailChainErr = nil
			return m, loadLicenseChainCmd(m.svc, m.selectedRow())

		case "n":
			return m, openWizardCmd(m.svc)

		case "i":
			// Import a PEM-encoded licence from disk. The picker emits a
			// licenseImportPickedMsg that Update handles below.
			return m, func() tea.Msg {
				return pushOverlayMsg{newFilePickerOverlay(func(path string) tea.Cmd {
					return func() tea.Msg { return licenseImportPickedMsg{path: path} }
				})}
			}

		case "E":
			// Export the selected licence to a .pem file. Symmetrical to [E]
			// on the issuers screen which exports the public key.
			if m.selectedRow() == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay(OverlayIDLicenseExport, "Export licence (.pem)", "/path/to/license.pem", 256)}
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
			if err := clipboardWriteAll(string(row.Pem)); err != nil {
				return m, func() tea.Msg {
					return pushOverlayMsg{newErrorOverlay("Clipboard Error", err.Error())}
				}
			}
			return m, nil

		case "e":
			// Re-issue the selected licence: open the wizard pre-populated
			// from the original. Identity/recipient/bindings/totp are
			// inherited; validity, audience/free-fields and payload are
			// editable. The previous confirm-only flow accepted no input
			// and emitted a licence with NotAfter=zero (i.e. expired) —
			// the new wizard makes the edited fields explicit.
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			return m, openReissueWizardCmd(m.svc, row)

		case "D":
			// Hard-delete the selected licence (License + Revocation + TOTPSecret).
			// Frees the unique license_uuid so a previously-exported PEM can be
			// re-imported. Destructive — confirm before calling.
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			short := row.LicenseUUID
			if len(short) > 12 {
				short = short[:8] + "…"
			}
			sub := fmt.Sprintf(
				"Supprimer définitivement la licence %q (uuid %s) ?\n"+
					"La ligne, sa révocation éventuelle et tout secret TOTP associé\n"+
					"seront effacés. L'audit conserve la trace de l'opération.\n"+
					"Le PEM exporté reste réimportable.",
				row.Subject, short)
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay(OverlayIDLicenseDelete, "Supprimer la licence", sub, "supprimer", "annuler", true)}
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

	// PEM tab key dispatch — TWO concurrent affordances per the truth table
	// in screen_licenses_keytable_test.go:
	//
	//   1) ↑/↓ STILL navigate the licences table (consistent with every
	//      other tab — operator expectation is preserved). The PEM viewport
	//      is auto-reloaded on cursor change below, so the preview tracks
	//      the selection.
	//   2) j/k/space/b/pgup/pgdown/g/G scroll the PEM viewport. Multiple
	//      bindings are layered: Windows Terminal and tmux often capture
	//      PgUp/PgDn for their own scrollback, but the alphabetic vim/less
	//      keys always reach the app.
	//
	// Crucially this block does NOT intercept ↑/↓ — those fall through to
	// the table.Update call below so the cursor moves.
	if msg, ok := msg.(tea.KeyMsg); ok && m.detail && m.detailTab == 2 {
		switch msg.String() {
		case "k":
			m.pemViewport.LineUp(3)
			return m, nil
		case "j":
			m.pemViewport.LineDown(3)
			return m, nil
		case "pgup", "b":
			m.pemViewport.HalfViewUp()
			return m, nil
		case "pgdown", " ":
			m.pemViewport.HalfViewDown()
			return m, nil
		case "home", "g":
			m.pemViewport.GotoTop()
			return m, nil
		case "end", "G":
			m.pemViewport.GotoBottom()
			return m, nil
		}
	}

	prevCursor := m.table.Cursor()
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	if m.detail && m.table.Cursor() != prevCursor {
		// Keyboard cursor moved (↑/↓/j/k) — refresh the open detail tab
		// against the new selection. Same path the click handler takes.
		refresh := m.refreshDetailForSelection()
		if refresh != nil {
			cmd = tea.Batch(cmd, refresh)
		}
	}
	return m, cmd
}

// refreshDetailForSelection re-loads whichever detail tab is currently
// open against the row at the table cursor. Returns nil when nothing
// needs loading (detail closed, no selection, or tab is the static
// Identité/Bindings views which derive entirely from the row in memory).
// The PEM viewport is updated synchronously since its content lives on
// the model; Audit and Chain need an async fetch.
func (m *licensesModel) refreshDetailForSelection() tea.Cmd {
	if !m.detail {
		return nil
	}
	row := m.selectedRow()
	if row == nil {
		return nil
	}
	switch m.detailTab {
	case 2: // PEM
		m.pemViewport.SetContent(Base.Render(string(row.Pem)))
		m.pemViewport.GotoTop()
		return nil
	case 3: // Audit
		m.detailAuditRows = nil
		m.detailAuditLoading = true
		m.detailAuditErr = nil
		return loadLicenseAuditCmd(m.svc, row)
	case 4: // Chain
		m.detailChain = nil
		m.detailChainLoading = true
		m.detailChainErr = nil
		return loadLicenseChainCmd(m.svc, row)
	}
	return nil
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
	now := time.Now()
	raw := make([][]string, 0, len(visible))
	for _, r := range visible {
		status := string(r.Status)
		if r.NotAfter.Before(now) && r.Status == licenseent.StatusActive {
			status = "expired"
		}
		raw = append(raw, []string{
			status,
			// Feed the full 36-char UUID so setAutoFitRows can grow the
			// column up to its full ideal when there's slack; on narrow
			// terminals the per-cell truncate inside setAutoFitRows clips
			// it to "<prefix>…" so nothing overflows.
			r.LicenseUUID,
			r.Subject,
			strings.Join(r.Audience, ","),
			r.IssuerName,
			r.NotAfter.Format("2006-01-02"),
			strings.Join(r.Features, ","),
		})
	}

	// Auto-size: ideals derived from header+content (cap 60). Weights:
	// STATUS/UUID/EXPIRES fixed-format → 0 so they stay at the content-
	// derived ideal (36 chars for a full UUID, never overshooting into
	// wasted padding); SUBJECT biggest share; AUDIENCE+FEATURES grow on
	// lists; KEYID grows modestly. On narrow terminals the shrink phase
	// in fitColumns proportionally clips UUID along with the others.
	setAutoFitRows(&m.table, BoxedInner(m.width), []int{0, 0, 3, 2, 1, 0, 2}, raw, 60)

	// Use the same layout-reservation budget as WindowSizeMsg uses for the
	// PEM viewport so table + detail body together fit exactly under the
	// root chrome's clampToHeight ceiling (m.hgt - ChromeRows). Pre-fix
	// clampTableHeight subtracted only ChromeRows + boxV (= 6) which is way
	// less than the real overhead (= 16 with detail open), so the table
	// over-reserved and pushed the detail panel past the screen bottom.
	const layoutReservation = 17
	var tableH int
	switch {
	case len(raw) == 0:
		tableH = 1
	case m.detail:
		half := (m.hgt - layoutReservation) / 2
		if half < 3 {
			half = 3
		}
		tableH = half
	default:
		// No detail panel — let the table take the full body minus chrome
		// and the table-box frame (top border + title + bottom border = 3).
		_, boxV := BoxFrame()
		tableH = m.hgt - ChromeRows - boxV - 3 - 4 // 4 = topRow(3) + leading blank(1)
		if tableH < 3 {
			tableH = 3
		}
	}
	m.table.SetHeight(tableH)
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
	// Record the absolute X-offset of the chip block so OnClick can map the
	// chip-local hit X-ranges produced by renderFilterBar back to screen
	// coordinates. Mirrors the JoinHorizontal layout: 1 leading space +
	// searchW + 2 + countW + 2.
	if m.chipHits != nil {
		offset := 1 + searchW + 2 + countW + 2
		for i := range m.chipHits.hits {
			m.chipHits.hits[i].x0 += offset
			m.chipHits.hits[i].x1 += offset
		}
		// topRow starts at TopChromeRows + 1 (leading blank); chip block is at
		// the same Y. In bordered mode it spans 3 lines (border + label + border);
		// in compact mode it's a single row.
		topY := TopChromeRows + 1
		m.chipHits.y0 = topY
		m.chipHits.y1 = topY + lipgloss.Height(chips) - 1
	}

	// Titled box wrapping the table. [↑↓] is informational (keyboard nav)
	// and not exposed as a clickable target — its synthesised KeyMsg would
	// move the cursor on whatever has focus, which isn't the intent.
	titleLabel := fmt.Sprintf("Licences (%d)", len(m.rows))
	title := titleBar(m.titleHints, titleLabel, []titleHint{
		{Key: "↑↓", Label: " nav ", Cmd: func() tea.Cmd { return nil }},
		{Key: "d", Label: " détail ", Cmd: keyCmd("d")},
		{Key: "n", Label: " nouvelle ", Cmd: keyCmd("n")},
		{Key: "i", Label: " importer ", Cmd: keyCmd("i")},
		{Key: "E", Label: " exporter ", Cmd: keyCmd("E")},
		{Key: "x", Label: " révoquer ", Cmd: keyCmd("x")},
		{Key: "e", Label: " re-émettre ", Cmd: keyCmd("e")},
		{Key: "D", Label: " supprimer", Cmd: keyCmd("D")},
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
		// Compute how many lines remain under the body before chrome's
		// clampToHeight bites. lipgloss.Height(body) under-counts when topRow
		// is wider than m.width and gets soft-wrapped at chrome level, so
		// subtract a 2-line safety margin. Clip the detail to that budget so
		// its bottom border always stays visible. On very small terminals
		// (< 6 lines remaining) the detail panel is suppressed entirely
		// rather than rendering a half-truncated frame.
		remaining := m.hgt - ContentReservedRows - lipgloss.Height(body) - 4
		if remaining >= 6 {
			body = lipgloss.JoinVertical(lipgloss.Left, body, clipDetailBox(m.renderDetail(), remaining))
		}
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
	if f, ok := m.chipHits.hit(x, y); ok {
		target := f
		return func() tea.Msg { return licenseFilterClickMsg{f: target} }
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
	// Chain detail body click: when on the Chain tab, the body content
	// starts 5 rows below the table bottom border (detail box top border +
	// title row + tab strip + blank row). A click on a parent or successor
	// chain entry dispatches licenseChainClickMsg to navigate there.
	if m.detail && m.detailTab == 4 && m.selectedRow() != nil {
		chainBaseY := tableEndY + 5
		if uuid, ok := m.chainHits.hit(y, chainBaseY); ok {
			target := uuid
			return func() tea.Msg { return licenseChainClickMsg{uuid: target} }
		}
	}
	// Detail [I/B/P/A/C] tab strip: detail box renders BELOW the table box.
	//   tableEndY     box bottom border ─
	//   tableEndY + 1 detail box top border ─
	//   tableEndY + 2 detail title row ("Détail · lic:… · subject")
	//   tableEndY + 3 tab strip ("[I]dent  [B]ind  [P]EM  [A]udit  [C]haîne")
	if m.detail && m.selectedRow() != nil {
		tabStripY := tableEndY + 3
		if y == tabStripY {
			// Tab strip renders inside BoxStyle; absolute X of the first character
			// is boxLeft = (total horizontal frame) / 2 (left side only).
			// Without this offset clicks at X<2 miss every tab (DS-L01).
			// Layout: "[I]dent  [B]ind  [P]EM  [A]udit  [C]haîne", gap=2 between cells.
			boxLeft := BoxStyle.GetHorizontalFrameSize() / 2
			cursor := boxLeft
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

// chainHit records the body-local row where one chain entry (parent, this,
// or successor) was rendered along with its licence UUID. chainHitRow.baseY
// holds the absolute terminal Y of the chain header so OnClick can
// translate clicks into body-local rows and dispatch licenseChainClickMsg.
type chainHit struct {
	licUUID string
	row     int
}

// chainHitRow is held by pointer on licensesModel so the value-receiver
// render path can mutate it without losing the writes — same pattern as
// chipHitRow and titleHintRow.
type chainHitRow struct {
	hits []chainHit
}

// hit translates an absolute click Y to a body-local chain row using the
// caller-supplied baseY (= absolute Y of the chain "header" row). OnClick
// owns the geometry so the renderer doesn't have to second-guess where the
// detail box landed on screen.
func (c *chainHitRow) hit(y, baseY int) (string, bool) {
	if c == nil {
		return "", false
	}
	local := y - baseY
	for _, h := range c.hits {
		if h.row == local {
			return h.licUUID, true
		}
	}
	return "", false
}

// licenseChainClickMsg asks the licences screen to navigate to the licence
// with the given UUID (filter→all, cursor set to that row, chain tab kept).
type licenseChainClickMsg struct{ uuid string }

// licenseImportPickedMsg carries the path returned by the file picker when
// the operator selected a PEM to import. Handled in Update.
type licenseImportPickedMsg struct{ path string }

// licenseImportedMsg signals a successful import; Update triggers a list
// reload so the new row appears.
type licenseImportedMsg struct{ row *ent.License }

// licenseFilterClickMsg is dispatched from OnClick when the operator clicks a
// filter chip; the licenses model handles it in Update.
type licenseFilterClickMsg struct{ f licenseFilter }

// renderFilterBar renders the chip bar AND records each chip's rendered
// X-span into chipHits. The recorded ranges are chip-local (origin = left
// edge of the returned string); the caller offsets by the chip block's
// absolute X in topRow.
func (m licensesModel) renderFilterBar() string {
	filters := []licenseFilter{licFilterAll, licFilterActive, licFilterExpiring, licFilterExpired, licFilterRevoked, licFilterSuperseded}
	pillWs := make([]int, len(filters))
	var parts []string
	for i, f := range filters {
		label := f.String()
		var seg string
		if f == m.filter {
			seg = PillActive.Render(label)
		} else {
			seg = PillOff.Render(label)
		}
		pillWs[i] = lipgloss.Width(seg)
		parts = append(parts, seg, " ")
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
	compact := m.width > 0 && lipgloss.Width(bordered) > m.width-topRowReserve
	hits := make([]chipHit, len(filters))
	if compact {
		sepW := lipgloss.Width(Mute.Render(" · "))
		cursor := 1 // leading " "
		for i, f := range filters {
			w := lipgloss.Width(f.String())
			hits[i] = chipHit{f: f, x0: cursor, x1: cursor + w}
			cursor += w + sepW
		}
		if m.chipHits != nil {
			m.chipHits.hits = hits
		}
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
	// Bordered mode: PaddingLeft adds 1 column; pills then sit shoulder-to-
	// shoulder with a 1-cell space between them. Each pill exposes label + 4
	// (2 border + 2 padding) horizontal cells, but we use the live measured
	// width so any future PillActive/PillOff style change stays correct.
	cursor := 1
	for i, f := range filters {
		hits[i] = chipHit{f: f, x0: cursor, x1: cursor + pillWs[i]}
		cursor += pillWs[i] + 1
	}
	if m.chipHits != nil {
		m.chipHits.hits = hits
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
	actions := Dim.Render("[d] replier  [c] PEM  [e] re-émettre  [x] révoquer  [D] supprimer")
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
// shortUUID returns the first 12 chars of a licence UUID followed by "…",
// or the value verbatim if it is shorter. Used in every list table that
// displays licences so operators can cross-reference rows with audit
// entries and the chain detail without seeing the full 36-char string.
func shortUUID(s string) string {
	const cut = 12
	if len(s) <= cut {
		return s
	}
	return s[:cut] + "…"
}

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
// Pure: any viewport content loading happens in Update() — the [P] key path,
// the licenseDetailTabClickMsg case 2, and tableSelectRowMsg / arrow-cursor
// changes. Calling SetContent here would be a no-op because this is a
// value-receiver method and the mutation never propagates back to the root.
func (m licensesModel) renderDetailPEM(row *ent.License) string {
	hint := HintKey.Render("[c]") + Dim.Render(" copier · ") +
		HintKey.Render("↑↓/jk") + Dim.Render(" scroll · ") +
		HintKey.Render("space/b") + Dim.Render(" page · ") +
		HintKey.Render("g/G") + Dim.Render(" début/fin")
	header := GlowCyan.Render("PEM signé") + "  " + hint

	pem := string(row.Pem)
	if pem == "" {
		return header + "\n" + Dim.Render("(PEM absent côté store — vérifie l'intégrité de la base ou re-émets la licence)")
	}
	// Viewport not yet sized (e.g. no WindowSizeMsg received yet) — render plain.
	if m.pemViewport.Height == 0 {
		return header + "\n" + Base.Render(pem)
	}
	// Scroll indicator: "lignes 5-16/47 · 25 %". Without this feedback the
	// operator can't tell whether their key is being captured or whether the
	// content simply fits in the viewport — exactly the "scroll doesn't
	// work" perception even when scrolling is mechanically functional.
	total := m.pemViewport.TotalLineCount()
	first := m.pemViewport.YOffset + 1
	last := m.pemViewport.YOffset + m.pemViewport.Height
	if last > total {
		last = total
	}
	pct := 0
	if total > m.pemViewport.Height {
		// Percent of the *scrollable* range, not of total lines.
		scrollable := total - m.pemViewport.Height
		if scrollable > 0 {
			pct = m.pemViewport.YOffset * 100 / scrollable
		}
	}
	var indicator string
	if total <= m.pemViewport.Height {
		indicator = Dim.Render(fmt.Sprintf("[%d lignes · tout visible]", total))
	} else {
		indicator = Dim.Render(fmt.Sprintf("[lignes %d-%d/%d · %d %%]", first, last, total, pct))
	}
	return header + "  " + indicator + "\n" + m.pemViewport.View()
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

// licenseChainLoadedMsg carries the resolved lineage for the Chain tab.
type licenseChainLoadedMsg struct {
	chain *service.LicenseChain
	err   error
}

// loadLicenseChainCmd resolves the full parent/successor chain for one licence.
// Returns nil when svc or row is nil (nothing to load).
func loadLicenseChainCmd(svc *service.Services, row *ent.License) tea.Cmd {
	if svc == nil || row == nil {
		return nil
	}
	id := row.ID
	return func() tea.Msg {
		chain, err := svc.License.GetChain(context.Background(), id)
		return licenseChainLoadedMsg{chain: chain, err: err}
	}
}

// renderDetailChain renders the real parent → this → successor lineage using
// ReplacesLicenseID edges resolved by loadLicenseChainCmd.
//
// Side-effect: appends to m.chainHits.hits (one entry per parent/this/
// successor row, body-local Y) so OnClick can dispatch a click on a chain
// row to licenseChainClickMsg{uuid}. View() is responsible for storing the
// absolute Y of the chain header in m.chainHits.baseY so OnClick can
// translate. The pointer-struct pattern mirrors chipHits.
func (m licensesModel) renderDetailChain(row *ent.License) string {
	// Reset hits BEFORE any early return so a click landing during the
	// loading/error states cannot dispatch a stale UUID inherited from
	// the previous chain render. Empty hits = OnClick falls through.
	if m.chainHits != nil {
		m.chainHits.hits = m.chainHits.hits[:0]
	}

	hint := HintKey.Render("[C]") + Dim.Render(" chaîne")
	header := GlowCyan.Render("Chaîne de succession") + "  " + hint

	if m.detailChainLoading {
		return header + "\n" + Dim.Render("  chargement de la chaîne…")
	}
	if m.detailChainErr != nil {
		return header + "\n" + GlowRed.Render("  erreur : "+m.detailChainErr.Error())
	}

	// When chain has not been loaded yet, show the current row so the UUID and
	// subject are always visible, with a prompt to load the full lineage.
	if m.detailChain == nil {
		const labelW = 14
		thisVal := GlowMagent.Render(row.LicenseUUID) + " " +
			Base.Render(row.Subject) + " " + licStatusPill(row.Status)
		prompt := Dim.Render("  (appuie sur [C] pour charger la chaîne complète)")
		if row.Status == licenseent.StatusSuperseded {
			prompt = GlowYellow.Render("cette licence est SUPERSEDED — re-émettre le successeur le plus récent")
		}
		return strings.Join([]string{
			header, "",
			kvRow("cette lic.", thisVal, labelW), "",
			prompt,
		}, "\n")
	}

	const labelW = 14
	divider := Dim.Render(strings.Repeat("─", BoxedInner(m.width)))

	var lines []string
	// row 0 = header, row 1 = blank, parents start at row 2.
	lines = append(lines, header, "")
	rowIdx := 2

	// Parents section (oldest first).
	if len(m.detailChain.Parents) == 0 {
		lines = append(lines, kvRow("parents", Mute.Render("aucun (racine de la chaîne)"), labelW))
		rowIdx++
	} else {
		for i, p := range m.detailChain.Parents {
			label := fmt.Sprintf("parent %d", i+1)
			val := GlowCyan.Render(p.LicenseUUID) + " " + Dim.Render(p.Subject) +
				" " + licStatusPill(p.Status) + " " + Dim.Render("(clic pour ouvrir)")
			lines = append(lines, kvRow(label, val, labelW))
			if m.chainHits != nil {
				m.chainHits.hits = append(m.chainHits.hits, chainHit{licUUID: p.LicenseUUID, row: rowIdx})
			}
			rowIdx++
		}
	}

	lines = append(lines, divider)
	rowIdx++

	// This licence. Not clickable — it's the row the operator is already on.
	thisVal := GlowMagent.Render(row.LicenseUUID) + " " +
		Base.Render(row.Subject) + " " + licStatusPill(row.Status)
	lines = append(lines, kvRow("cette lic.", thisVal, labelW))
	rowIdx++

	lines = append(lines, divider)
	rowIdx++

	// Successors section.
	if len(m.detailChain.Successors) == 0 {
		lines = append(lines, kvRow("successeurs", Mute.Render("aucun (extrémité de la chaîne)"), labelW))
	} else {
		for i, s := range m.detailChain.Successors {
			label := fmt.Sprintf("successeur %d", i+1)
			val := GlowCyan.Render(s.LicenseUUID) + " " + Dim.Render(s.Subject) +
				" " + licStatusPill(s.Status) + " " + Dim.Render("(clic pour ouvrir)")
			lines = append(lines, kvRow(label, val, labelW))
			if m.chainHits != nil {
				m.chainHits.hits = append(m.chainHits.hits, chainHit{licUUID: s.LicenseUUID, row: rowIdx})
			}
			rowIdx++
		}
	}

	return strings.Join(lines, "\n")
}

// handleLicenseInputResult processes InputResultMsg payloads routed from
// app.go. Today only the [E] export path lands here; future input-style
// actions on the licences screen will share this switch.
func (m licensesModel) handleLicenseInputResult(res InputResultMsg) (licensesModel, tea.Cmd) {
	if res.ID != OverlayIDLicenseExport {
		return m, nil
	}
	row := m.selectedRow()
	if row == nil || m.svc == nil {
		return m, nil
	}
	id := row.ID
	path := ensureExtension(res.Value, ".pem")
	svc := m.svc
	return m, func() tea.Msg {
		pem, err := svc.License.ExportPEM(context.Background(), id)
		if err != nil {
			return pushOverlayMsg{newErrorOverlay("Export Error", err.Error())}
		}
		if err := os.WriteFile(path, pem, 0o600); err != nil {
			return pushOverlayMsg{newErrorOverlay("Write Error", err.Error())}
		}
		return pushOverlayMsg{NewOKOverlay("Export OK", "Wrote "+path)}
	}
}

// handleLicenseDeleteConfirm processes the operator's reply to the
// OverlayIDLicenseDelete confirm overlay. On Confirm=true it calls
// svc.License.Delete and reloads the list; cancellation is a no-op.
func (m licensesModel) handleLicenseDeleteConfirm(res ConfirmResultMsg) (licensesModel, tea.Cmd) {
	if res.ID != OverlayIDLicenseDelete || !res.Confirm {
		return m, nil
	}
	row := m.selectedRow()
	if row == nil || m.svc == nil {
		return m, nil
	}
	svc := m.svc
	id := row.ID
	subject := row.Subject
	return m, func() tea.Msg {
		if err := svc.License.Delete(context.Background(), id, "operator"); err != nil {
			return pushOverlayMsg{newErrorOverlay("Suppression échouée", err.Error())}
		}
		rows, err := svc.License.List(context.Background(), service.ListFilter{Limit: 500})
		return licenseDeletedMsg{rows: rows, err: err, subject: subject}
	}
}

// licenseDeletedMsg carries the post-delete list reload + the subject of the
// row that was removed. The licences screen Update handles it and pushes the
// confirmation toast — app.go only routes it through.
type licenseDeletedMsg struct {
	rows    []*ent.License
	err     error
	subject string
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

