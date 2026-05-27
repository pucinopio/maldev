package tui

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// AuditLoadedMsg carries the result of fetching audit events.
type AuditLoadedMsg struct {
	Rows []*ent.AuditEvent
	Err  error
}

type auditKindFilter int

const (
	auditFilterAll auditKindFilter = iota
	auditFilterLicense
	auditFilterKey
	auditFilterServer
	auditFilterIdentity
	auditFilterProbe
	auditFilterCount
)

// label returns the display label for the chip.
func (f auditKindFilter) label() string {
	switch f {
	case auditFilterAll:
		return "all"
	case auditFilterLicense:
		return "license"
	case auditFilterKey:
		return "key"
	case auditFilterServer:
		return "server"
	case auditFilterIdentity:
		return "identity"
	case auditFilterProbe:
		return "probe"
	}
	return "all"
}

// hotkey returns the single character shortcut shown on the chip.
func (f auditKindFilter) hotkey() string {
	switch f {
	case auditFilterAll:
		return "f"
	case auditFilterLicense:
		return "l"
	case auditFilterKey:
		return "k"
	case auditFilterServer:
		return "s"
	case auditFilterIdentity:
		return "i"
	case auditFilterProbe:
		return "p"
	}
	return "f"
}

func (f auditKindFilter) String() string { return f.label() }

type auditModel struct {
	svc    *service.Services
	rows   []*ent.AuditEvent
	err    error
	filter auditKindFilter
	table  table.Model
	detail bool
	vp     viewport.Model
	width  int
	hgt    int
}

func newAuditModel(svc *service.Services) auditModel {
	cols := []table.Column{
		{Title: "TIMESTAMP", Width: 20},
		{Title: "KIND", Width: 20},
		{Title: "ACTOR", Width: 14},
		{Title: "TARGET", Width: 16},
		{Title: "NOTE", Width: 20},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(15),
		table.WithStyles(licTableStyles()),
	)
	return auditModel{svc: svc, table: t, vp: viewport.New(80, 10)}
}

func listAuditCmd(svc *service.Services) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return AuditLoadedMsg{}
		}
		rows, err := svc.Audit.List(context.Background(), 200)
		return AuditLoadedMsg{Rows: rows, Err: err}
	}
}

func (m auditModel) Init() tea.Cmd { return listAuditCmd(m.svc) }

func (m auditModel) Update(msg tea.Msg) (auditModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.hgt = msg.Height
		m.vp.Width = BoxedInner(msg.Width)
		m.rebuildTable()
		return m, nil

	case AuditLoadedMsg:
		m.err = msg.Err
		m.rows = msg.Rows
		m.rebuildTable()
		return m, nil

	case auditFilterClickMsg:
		m.filter = msg.f
		m.rebuildTable()
		return m, nil

	case tea.KeyMsg:
		// r/E/J fire at all times — they must not be swallowed by the detail
		// viewport when detail is open, otherwise refresh and export become
		// unreachable with a panel on screen.
		switch msg.String() {
		case "r":
			return m, listAuditCmd(m.svc)
		case "E":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay(OverlayIDAuditExportCSV, "Export CSV", "/path/to/audit.csv", 256)}
			}
		case "J":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay(OverlayIDAuditExportJSON, "Export JSON", "/path/to/audit.json", 256)}
			}
		}

		if m.detail {
			switch msg.String() {
			case "esc", "d":
				m.detail = false
				return m, nil
			}
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}

		// Filter hotkeys mirror auditKindFilter.hotkey() — one key per filter value.
		filterByKey := map[string]auditKindFilter{
			"f": auditFilterAll, "l": auditFilterLicense, "k": auditFilterKey,
			"s": auditFilterServer, "i": auditFilterIdentity, "p": auditFilterProbe,
		}
		if f, ok := filterByKey[msg.String()]; ok {
			m.filter = f
			m.rebuildTable()
			return m, nil
		}

		switch msg.String() {
		case "d":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			m.detail = true
			m.vp.SetContent(m.renderPayload(row))
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *auditModel) selectedRow() *ent.AuditEvent {
	visible := m.visibleRows()
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(visible) {
		return nil
	}
	return visible[idx]
}

func (m *auditModel) visibleRows() []*ent.AuditEvent {
	var out []*ent.AuditEvent
	for _, r := range m.rows {
		switch m.filter {
		case auditFilterLicense:
			if !strings.HasPrefix(r.Kind, "license.") {
				continue
			}
		case auditFilterKey:
			if !strings.HasPrefix(r.Kind, "issuer.") && !strings.HasPrefix(r.Kind, "recipient.") {
				continue
			}
		case auditFilterServer:
			if !strings.HasPrefix(r.Kind, "server.") {
				continue
			}
		case auditFilterIdentity:
			if !strings.HasPrefix(r.Kind, "identity.") {
				continue
			}
		case auditFilterProbe:
			if !strings.HasPrefix(r.Kind, "probe.") {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

func (m *auditModel) rebuildTable() {
	visible := m.visibleRows()
	raw := make([][]string, 0, len(visible))
	for _, r := range visible {
		note := ""
		if n, ok := r.Payload["note"]; ok {
			note = fmt.Sprintf("%v", n)
		}
		raw = append(raw, []string{
			r.CreatedAt.Format("2006-01-02 15:04:05"),
			r.Kind,
			r.Actor,
			r.TargetKind + "/" + r.TargetID,
			note,
		})
	}
	// Weights: TIMESTAMP fixed; KIND/ACTOR modest; TARGET useful; NOTE biggest.
	setAutoFitRows(&m.table, m.width, []int{0, 1, 1, 2, 3}, raw, 60)

	// Magic 6 = empirically measured fixed-row overhead the audit screen
	// reserves above the table (chip bar + title + spacers + statusbar).
	const auditFixedOverhead = ChromeRows + 2 // 4 + 2 = 6
	tableH := clampTableHeight(m.hgt-auditFixedOverhead, m.detail, len(raw) == 0)
	m.table.SetHeight(tableH)
	if m.detail {
		m.vp.Height = m.hgt - tableH - auditFixedOverhead
		if m.vp.Height < 3 {
			m.vp.Height = 3
		}
	}
}

func (m auditModel) renderPayload(row *ent.AuditEvent) string {
	b, err := json.MarshalIndent(row.Payload, "  ", "  ")
	if err != nil {
		return fmt.Sprintf("  error: %v", err)
	}
	lines := []string{
		fmt.Sprintf("  %-12s %s", "id:", row.ID.String()),
		fmt.Sprintf("  %-12s %s", "kind:", row.Kind),
		fmt.Sprintf("  %-12s %s", "actor:", row.Actor),
		fmt.Sprintf("  %-12s %s/%s", "target:", row.TargetKind, row.TargetID),
		fmt.Sprintf("  %-12s %s", "timestamp:", row.CreatedAt.Format(time.RFC3339)),
		"",
		"  payload:",
		"  " + string(b),
	}
	return strings.Join(lines, "\n")
}

func (m auditModel) View() string {
	// ── Filter chip bar ───────────────────────────────────────────────────
	// Each chip shows its hotkey and label; active chip uses magenta border.
	allFilters := []auditKindFilter{
		auditFilterAll, auditFilterLicense, auditFilterKey,
		auditFilterServer, auditFilterIdentity, auditFilterProbe,
	}
	filterLabel := Dim.Render("filtres :")
	// Measure the bordered chip strip width. If it overflows m.width,
	// fall back to flat text chips (no border) so the chip-row top/
	// bottom borders don't fragment across multiple lines at 80×24.
	var chips []string
	for _, f := range allFilters {
		label := HintKey.Render(f.hotkey()) + Dim.Render(f.label())
		if f == m.filter {
			chips = append(chips, ChipActive.Render(HintKey.Render(f.hotkey())+Base.Render(f.label())))
		} else {
			chips = append(chips, ChipInactive.Render(label))
		}
	}
	exportHints := Dim.Render("E export CSV  J export JSON")
	chipRow := lipgloss.JoinHorizontal(lipgloss.Top, chips...)
	chipBar := lipgloss.JoinHorizontal(lipgloss.Top,
		" ", filterLabel, "  ",
		chipRow,
		"  ", exportHints,
	)
	if lipgloss.Width(chipBar) > m.width {
		// Compact: flat chips, comma-separated, fits on one line.
		var flatChips []string
		for _, f := range allFilters {
			seg := HintKey.Render(f.hotkey()) + " " + Dim.Render(f.label())
			if f == m.filter {
				seg = HintKey.Render(f.hotkey()) + " " + Base.Render(f.label())
			}
			flatChips = append(flatChips, seg)
		}
		chipBar = " " + filterLabel + " " + strings.Join(flatChips, Mute.Render(" · "))
	}

	// ── Table box with count in title ─────────────────────────────────────
	// Use table row count (already filtered by rebuildTable) rather than
	// calling visibleRows() again to avoid a second O(n) scan.
	tableTitle := GlowCyan.Render(fmt.Sprintf("Audit (%d)", len(m.table.Rows()))) +
		"  " + Dim.Render("[↑↓] nav · [d] detail · [r] refresh · [pgup/pgdn] page")
	tableBody := m.table.View()
	if hint := emptyTableHint(len(m.table.Rows()), m.width, "aucun évènement — l'historique s'enrichit à chaque action"); hint != "" {
		tableBody = lipgloss.JoinVertical(lipgloss.Left, tableBody, "", hint)
	}
	tableBox := lipgloss.JoinVertical(lipgloss.Left, tableTitle, tableBody)

	body := lipgloss.JoinVertical(lipgloss.Left, chipBar, "", tableBox)

	// Always render the detail card — prototype shows it even with no row
	// selected so the box position stays stable. When detail mode is on we
	// show the payload viewport; otherwise a short hint.
	selected := m.selectedRow() // cache: visibleRows() iterates all rows
	var detailBox string
	switch {
	case m.detail:
		detailBox = BoxFocused.Width(BoxedInner(m.width)).Render(m.vp.View())
	case selected != nil:
		row := selected
		title := Dim.Render("Detail · ") + GlowCyan.Render(row.Kind) + Dim.Render(" · ") + Base.Render(row.Actor)
		hint := HintKey.Render("[d]") + Dim.Render(" déplier le payload  ·  ") +
			HintKey.Render("[E/J]") + Dim.Render(" exporter")
		detailBox = BoxStyle.Width(BoxedInner(m.width)).Render(
			lipgloss.JoinVertical(lipgloss.Left, title, "", hint),
		)
	default:
		title := Dim.Render("Detail")
		hint := Dim.Render("aucune sélection — utilise ↑/↓ pour choisir une ligne puis ") +
			HintKey.Render("[d]") + Dim.Render(" pour le détail")
		detailBox = BoxStyle.Width(BoxedInner(m.width)).Render(
			lipgloss.JoinVertical(lipgloss.Left, title, "", hint),
		)
	}
	body = lipgloss.JoinVertical(lipgloss.Left, body, detailBox)

	if m.err != nil {
		body = GlowRed.Render("Error: "+m.err.Error()) + "\n" + body
	}

	// Status bar rendered globally by the root chrome — don't duplicate here.
	return body
}

// handleAuditInputResult processes input overlay results for the audit screen.
func (m auditModel) handleAuditInputResult(res InputResultMsg) (auditModel, tea.Cmd) {
	switch res.ID {
	case OverlayIDAuditExportCSV:
		rows := m.visibleRows()
		path := res.Value
		return m, func() tea.Msg {
			if err := writeAuditCSV(path, rows); err != nil {
				return pushOverlayMsg{newErrorOverlay("Export Error", err.Error())}
			}
			return nil
		}

	case OverlayIDAuditExportJSON:
		rows := m.visibleRows()
		path := res.Value
		return m, func() tea.Msg {
			if err := writeAuditJSON(path, rows); err != nil {
				return pushOverlayMsg{newErrorOverlay("Export Error", err.Error())}
			}
			return nil
		}
	}
	return m, nil
}

func writeAuditCSV(path string, rows []*ent.AuditEvent) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"timestamp", "kind", "actor", "target_kind", "target_id", "payload"})
	for _, r := range rows {
		payload, _ := json.Marshal(r.Payload)
		_ = w.Write([]string{
			r.CreatedAt.Format(time.RFC3339),
			r.Kind,
			r.Actor,
			r.TargetKind,
			r.TargetID,
			string(payload),
		})
	}
	w.Flush()
	return w.Error()
}

func writeAuditJSON(path string, rows []*ent.AuditEvent) error {
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// OnClick handles filter-chip clicks. Chip bar sits immediately below the
// chrome (TopChromeRows..TopChromeRows+2 — 3-row bordered pill block:
// top border, label row, bottom border). Mirrors chipBar layout:
// " filtres :  " prefix then chips back-to-back.
func (m auditModel) OnClick(x, y, _ int) tea.Cmd {
	const chipTopY = TopChromeRows
	if y < chipTopY || y > chipTopY+2 {
		return nil
	}
	allFilters := []auditKindFilter{
		auditFilterAll, auditFilterLicense, auditFilterKey,
		auditFilterServer, auditFilterIdentity, auditFilterProbe,
	}
	// 1 leading space + "filtres :" (9 chars) + 2 spaces = 12 cells before chips.
	cursor := 12
	for _, f := range allFilters {
		// Pill = HintKey.Render(hotkey) + Base.Render(label) wrapped in a
		// Border+Padding(0,1) style. HintKey itself has Padding(0,1) so its
		// rendered width is 1+1+1=3 cells. Then label cells. Then 4 cells of
		// outer border+padding (1 border + 1 padding on each side).
		pillW := lipgloss.Width(HintKey.Render(f.hotkey())) + lipgloss.Width(f.label()) + 4
		if x >= cursor && x < cursor+pillW {
			target := f
			return func() tea.Msg { return auditFilterClickMsg{f: target} }
		}
		cursor += pillW
	}
	return nil
}

type auditFilterClickMsg struct{ f auditKindFilter }
