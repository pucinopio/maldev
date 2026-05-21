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
		table.WithFocused(false),
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
		m.vp.Width = msg.Width - 4
		m.rebuildTable()
		return m, nil

	case AuditLoadedMsg:
		m.err = msg.Err
		m.rows = msg.Rows
		m.rebuildTable()
		return m, nil

	case tea.KeyMsg:
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

		switch msg.String() {
		case "d":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			m.detail = true
			m.vp.SetContent(m.renderPayload(row))
			return m, nil

		case "f":
			m.filter = auditFilterAll
			m.rebuildTable()
			return m, nil
		case "l":
			m.filter = auditFilterLicense
			m.rebuildTable()
			return m, nil
		case "k":
			m.filter = auditFilterKey
			m.rebuildTable()
			return m, nil
		case "s":
			m.filter = auditFilterServer
			m.rebuildTable()
			return m, nil
		case "i":
			m.filter = auditFilterIdentity
			m.rebuildTable()
			return m, nil
		case "p":
			m.filter = auditFilterProbe
			m.rebuildTable()
			return m, nil

		case "r":
			return m, listAuditCmd(m.svc)

		case "E":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("audit-export-csv", "Export CSV", "/path/to/audit.csv", 256)}
			}

		case "J":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("audit-export-json", "Export JSON", "/path/to/audit.json", 256)}
			}
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
	rows := make([]table.Row, 0, len(visible))
	for _, r := range visible {
		ts := r.CreatedAt.Format("2006-01-02 15:04:05")
		target := r.TargetKind + "/" + r.TargetID
		if len(target) > 14 {
			target = target[:13] + "…"
		}
		note := ""
		if n, ok := r.Payload["note"]; ok {
			note = fmt.Sprintf("%v", n)
		}
		if len(note) > 18 {
			note = note[:17] + "…"
		}
		rows = append(rows, table.Row{ts, r.Kind, r.Actor, target, note})
	}
	tableH := m.hgt - 6
	if m.detail {
		tableH = tableH / 2
	}
	if tableH < 3 {
		tableH = 3
	}
	m.table.SetRows(rows)
	m.table.SetHeight(tableH)
	if m.detail {
		m.vp.Height = m.hgt - tableH - 6
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
	var chips []string
	for _, f := range allFilters {
		label := HintKey.Render(f.hotkey()) + Dim.Render(f.label())
		if f == m.filter {
			chips = append(chips, lipgloss.NewStyle().
				Foreground(Palette.Magenta).
				Border(lipgloss.NormalBorder()).
				BorderForeground(Palette.Magenta).
				Padding(0, 1).
				Render(HintKey.Render(f.hotkey())+Base.Render(f.label())))
		} else {
			chips = append(chips, lipgloss.NewStyle().
				Foreground(Palette.FgDim).
				Border(lipgloss.NormalBorder()).
				BorderForeground(Palette.Border).
				Padding(0, 1).
				Render(label))
		}
	}
	exportHints := Dim.Render("E export CSV  J export JSON")
	chipBar := lipgloss.JoinHorizontal(lipgloss.Top,
		" ", filterLabel, "  ",
		strings.Join(chips, " "),
		"  ", exportHints,
	)

	// ── Table box with count in title ─────────────────────────────────────
	count := len(m.visibleRows())
	tableTitle := GlowCyan.Render(fmt.Sprintf("Audit (%d)", count)) +
		"  " + Dim.Render("[d] detail · [r] refresh · [pgup/pgdn] page")
	tableBox := lipgloss.JoinVertical(lipgloss.Left, tableTitle, m.table.View())

	body := lipgloss.JoinVertical(lipgloss.Left, chipBar, "", tableBox)

	if m.detail {
		detailBox := BoxFocused.Width(m.width - 4).Render(m.vp.View())
		body = lipgloss.JoinVertical(lipgloss.Left, body, detailBox)
	}

	if m.err != nil {
		body = GlowRed.Render("Error: "+m.err.Error()) + "\n" + body
	}

	hints := []string{"f", "all", "l", "license", "k", "key", "s", "server", "i", "identity", "p", "probe", "d", "detail", "r", "refresh"}
	return lipgloss.JoinVertical(lipgloss.Left, body, renderStatusBar(hints, m.width))
}

// handleAuditInputResult processes input overlay results for the audit screen.
func (m auditModel) handleAuditInputResult(res InputResultMsg) (auditModel, tea.Cmd) {
	switch res.ID {
	case "audit-export-csv":
		rows := m.visibleRows()
		path := res.Value
		return m, func() tea.Msg {
			if err := writeAuditCSV(path, rows); err != nil {
				return pushOverlayMsg{newErrorOverlay("Export Error", err.Error())}
			}
			return nil
		}

	case "audit-export-json":
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
