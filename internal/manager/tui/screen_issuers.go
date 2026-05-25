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

	// titleHints is shared by reference (pointer) so the layout state View()
	// stores survives to the next OnClick call — bubbletea passes the model
	// by value but the pointer points to the same heap struct.
	titleHints *titleHintRow
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
	return issuersModel{svc: svc, table: t, titleHints: &titleHintRow{}}
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
	tableH := listTableHeight(m.hgt, m.width,
		"Les issuer keys sont les clés Ed25519 qui signent tes licences. Une seule clé est active à la fois ; les autres sont retraitées (retired).")
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
	stretchLastColumn(&m.table, BoxedInner(m.width))
}

// OnClick dispatches title-bar hint clicks (synthesised as KeyMsg) and
// table-row selection. The title row Y is recorded by View() into
// m.titleHints; data rows sit two lines below it (border + title).
func (m issuersModel) OnClick(x, y, _ int) tea.Cmd {
	if cmd := m.titleHints.hit(x, y); cmd != nil {
		return cmd
	}
	return tableRowCmd(m.titleHints.y+1, len(m.rows), y)
}

// tableSelectRowMsg asks the active screen to move its table cursor to row.
type tableSelectRowMsg struct{ row int }

// listTableHeight returns the table height budget for a list screen whose body
// is "intro paragraph + boxed table". Accounts for the intro wrapping on narrow
// terminals so the box bottom border never overflows into the status bar.
//
// chrome=ChromeRows (title+tabs+breadcrumb+statusbar), intro+blank=introH+2,
// box border+title+padding=4, sub-tabs row=1 → total fixed overhead is the sum
// below; subtracting hits the table cell budget directly.
func listTableHeight(hgt, width int, intro string) int {
	introH := 1
	if width > 1 {
		introH = (len(intro) + width - 2) / (width - 1)
		if introH < 1 {
			introH = 1
		}
	}
	h := hgt - (ChromeRows + introH + 2 + 2 + 1 + 1)
	if h < 3 {
		h = 3
	}
	return h
}

func (m issuersModel) View() string {
	intro := Dim.Render(" Les ") + GlowCyan.Render("issuer keys") +
		Dim.Render(" sont les clés Ed25519 qui signent tes licences. Une seule clé est active à la fois ; les autres sont retraitées (retired).")

	titleLabel := fmt.Sprintf("Issuer keys Ed25519 (%d)", len(m.rows))
	title := titleBar(m.titleHints, titleLabel, []titleHint{
		{Key: "n", Label: " générer ", Cmd: keyCmd("n")},
		{Key: "a", Label: " activer ", Cmd: keyCmd("a")},
		{Key: "E", Label: " export .pub ", Cmd: keyCmd("E")},
		{Key: "x", Label: " retraiter", Cmd: keyCmd("x")},
	}, 0, BoxedInner(m.width))
	// Title row Y = TopChromeRows + leading blank + introH + trailing blank
	// + box top border. introH uses the rendered height so wrapped intros and
	// future multi-line additions stay accurate.
	introH := lipgloss.Height(lipgloss.NewStyle().Width(m.width).Render(intro))
	m.titleHints.SetY(TopChromeRows + 1 + introH + 1 + 1)

	tableBody := m.table.View()
	if h := emptyTableHint(len(m.rows), m.width, "aucune clé d'émission — n pour créer la première"); h != "" {
		tableBody = lipgloss.JoinVertical(lipgloss.Left, tableBody, "", h)
	}
	boxed := BoxStyle.Width(BoxedWidth(m.width)).Render(title + "\n" + tableBody)

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

	colStyle := lipgloss.NewStyle().Width(colW)
	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		colStyle.Render(meta),
		"  ",
		colStyle.Render(actions),
	)
	return BoxStyle.Width(BoxedWidth(m.width)).Render(cols)
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
