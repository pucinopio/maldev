package tui

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// IssuersLoadedMsg carries the result of fetching all issuers.
type IssuersLoadedMsg struct {
	Rows []*ent.Issuer
	Err  error
}

type issuersModel struct {
	svc    *service.Services
	rows   []*ent.Issuer
	err    error
	table  table.Model
	detail bool
	width  int
	hgt    int
}

func newIssuersModel(svc *service.Services) issuersModel {
	cols := []table.Column{
		{Title: "KEYID", Width: 20},
		{Title: "NAME", Width: 24},
		{Title: "STATUS", Width: 10},
		{Title: "CREATED", Width: 12},
		{Title: "#SIGNED", Width: 8},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(false),
		table.WithHeight(15),
		table.WithStyles(licTableStyles()),
	)
	return issuersModel{svc: svc, table: t}
}

func listIssuersCmd(svc *service.Services) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return IssuersLoadedMsg{}
		}
		rows, err := svc.Issuer.List(context.Background())
		return IssuersLoadedMsg{Rows: rows, Err: err}
	}
}

func (m issuersModel) Init() tea.Cmd { return listIssuersCmd(m.svc) }

func (m issuersModel) Update(msg tea.Msg) (issuersModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.hgt = msg.Height
		m.rebuildTable()
		return m, nil

	case IssuersLoadedMsg:
		m.err = msg.Err
		m.rows = msg.Rows
		m.rebuildTable()
		return m, nil

	case tableSelectRowMsg:
		m.table.SetCursor(msg.row)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "d":
			m.detail = !m.detail
			return m, nil

		case "a":
			row := m.selectedRow()
			if row == nil || m.svc == nil {
				return m, nil
			}
			id := row.ID
			return m, func() tea.Msg {
				err := m.svc.Issuer.SetActive(context.Background(), id, "operator")
				if err != nil {
					return IssuersLoadedMsg{Err: err}
				}
				rows, err := m.svc.Issuer.List(context.Background())
				return IssuersLoadedMsg{Rows: rows, Err: err}
			}

		case "n":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("issuer-name", "New Issuer", "name (e.g. prod-2026)", 80)}
			}

		case "E":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("issuer-export-pub", "Export Public Key", "/path/to/issuer.pub.pem", 256)}
			}

		case "K":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			sub := fmt.Sprintf("Export private key for %q?\nThis reveals the signing key — store securely.", row.Name)
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay("issuer-export-priv", "Export Private Key", sub, "export", "cancel", true)}
			}

		case "x":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			sub := fmt.Sprintf("Retire issuer %q?\nIt will no longer be usable for new licences.", row.Name)
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay("issuer-retire", "Retire Issuer", sub, "retire", "cancel", true)}
			}

		case "r":
			return m, listIssuersCmd(m.svc)
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *issuersModel) selectedRow() *ent.Issuer {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rows) {
		return nil
	}
	return m.rows[idx]
}

func (m *issuersModel) rebuildTable() {
	rows := make([]table.Row, 0, len(m.rows))
	for _, r := range m.rows {
		status := "inactive"
		if r.Active {
			status = "active"
		} else if r.RetiredAt != nil {
			status = "retired"
		}
		created := r.CreatedAt.Format("2006-01-02")
		rows = append(rows, table.Row{
			r.KeyID, r.Name, status, created, "—",
		})
	}
	tableH := m.hgt - 6
	if m.detail {
		tableH = tableH / 2
	}
	if tableH < 3 {
		tableH = 3
	}
	// Collapse the table to its header row when empty so the empty-state hint
	// added by View() sits directly under the header instead of being pushed
	// off-screen by a full-height grid of blank rows.
	if len(rows) == 0 {
		tableH = 1
	}
	m.table.SetRows(rows)
	m.table.SetHeight(tableH)
	stretchLastColumn(&m.table, m.width-4) // -4 = box border(2) + padding(2)
}

// OnClick selects the clicked table row. Chrome occupies Y=0..3; table header
// is at Y=4, data rows start at Y=5.
func (m issuersModel) OnClick(x, y, _ int) tea.Cmd {
	const headerY = 4
	if y <= headerY {
		return nil
	}
	row := y - headerY - 1
	if row < 0 || row >= len(m.rows) {
		return nil
	}
	target := row
	return func() tea.Msg { return tableSelectRowMsg{row: target} }
}

// tableSelectRowMsg asks the active screen to move its table cursor to row.
type tableSelectRowMsg struct{ row int }

func (m issuersModel) View() string {
	intro := Dim.Render(" Les ") + GlowCyan.Render("issuer keys") +
		Dim.Render(" sont les clés Ed25519 qui signent tes licences. Une seule clé est active à la fois ; les autres sont retraitées (retired).")

	titleLabel := fmt.Sprintf("Issuer keys Ed25519 (%d)", len(m.rows))
	hint := HintKey.Render("[n]") + Dim.Render(" générer ") +
		Mute.Render("· ") + HintKey.Render("[a]") + Dim.Render(" activer ") +
		Mute.Render("· ") + HintKey.Render("[E]") + Dim.Render(" export .pub ") +
		Mute.Render("· ") + HintKey.Render("[x]") + Dim.Render(" retraiter")
	title := titledBoxRow(titleLabel, hint, m.width-4)

	tableBody := m.table.View()
	if h := emptyTableHint(len(m.rows), m.width, "aucune clé d'émission — n pour créer la première"); h != "" {
		tableBody = lipgloss.JoinVertical(lipgloss.Left, tableBody, "", h)
	}
	boxed := BoxStyle.Width(m.width - 2).Render(title + "\n" + tableBody)

	body := lipgloss.JoinVertical(lipgloss.Left, "", intro, "", boxed)
	if m.detail {
		body = lipgloss.JoinVertical(lipgloss.Left, body, m.renderDetail())
	}
	if m.err != nil {
		body = GlowRed.Render("Error: "+m.err.Error()) + "\n" + body
	}
	return body
}

func (m issuersModel) renderDetail() string {
	row := m.selectedRow()
	if row == nil {
		return Dim.Render("  no selection")
	}

	var statusPill string
	switch {
	case row.Active:
		statusPill = PillActive.Render("ACTIVE")
	case row.RetiredAt != nil:
		statusPill = PillOff.Render("RETIRED")
	default:
		statusPill = PillOff.Render("INACTIVE")
	}

	// Left column: metadata KVs matching issuers.jsx expandedRowRender.
	colW := detailColW(m.width)
	meta := lipgloss.JoinVertical(lipgloss.Left,
		GlowCyan.Render("Métadonnées"),
		kvRow("keyid", GlowCyan.Render(row.KeyID), 10),
		kvRow("name", row.Name, 10),
		kvRow("status", statusPill, 10),
		kvRow("created", row.CreatedAt.Format("2006-01-02"), 10),
		kvRow("db-id", row.ID.String(), 10),
	)

	// Right column: actions matching issuers.jsx Actions section.
	activeLabel := "désigner active"
	if row.Active {
		activeLabel = "déjà active (aucune action)"
	}
	actions := lipgloss.JoinVertical(lipgloss.Left,
		GlowCyan.Render("Actions"),
		HintKey.Render("[a]")+" "+Dim.Render(activeLabel),
		HintKey.Render("[E]")+" "+Dim.Render("exporter clé publique (.pub)"),
		HintKey.Render("[K]")+" "+Dim.Render("exporter clé privée (.key) — confirmation"),
		GlowRed.Render("[x]")+" "+Dim.Render("retirer (la clé reste vérifiable côté binaire)"),
	)

	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(colW).Render(meta),
		"  ",
		lipgloss.NewStyle().Width(colW).Render(actions),
	)
	return BoxStyle.Width(m.width - 2).Render(cols)
}

// handleIssuerInputResult processes overlay results for the issuers screen.
func (m issuersModel) handleIssuerInputResult(res InputResultMsg) (issuersModel, tea.Cmd) {
	switch res.ID {
	case "issuer-name":
		if m.svc == nil {
			return m, nil
		}
		name := res.Value
		return m, func() tea.Msg {
			keyID := fmt.Sprintf("key-%d", time.Now().Unix())
			_, err := m.svc.Issuer.Generate(context.Background(), name, keyID, "operator")
			if err != nil {
				return IssuersLoadedMsg{Err: err}
			}
			rows, err := m.svc.Issuer.List(context.Background())
			return IssuersLoadedMsg{Rows: rows, Err: err}
		}

	case "issuer-export-pub":
		row := m.selectedRow()
		if row == nil || m.svc == nil {
			return m, nil
		}
		id := row.ID
		path := res.Value
		return m, func() tea.Msg {
			pem, err := m.svc.Issuer.ExportPublic(context.Background(), id)
			if err != nil {
				return pushOverlayMsg{newErrorOverlay("Export Error", err.Error())}
			}
			if err := os.WriteFile(path, pem, 0o600); err != nil {
				return pushOverlayMsg{newErrorOverlay("Write Error", err.Error())}
			}
			rows, err := m.svc.Issuer.List(context.Background())
			return IssuersLoadedMsg{Rows: rows, Err: err}
		}
	}
	return m, nil
}
