package tui

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// RecipientsLoadedMsg carries the result of fetching all recipient keys.
type RecipientsLoadedMsg struct {
	Rows []*ent.RecipientKey
	Err  error
}

type recipientsModel struct {
	svc        *service.Services
	rows       []*ent.RecipientKey
	err        error
	table      table.Model
	detail     bool
	width      int
	hgt        int
	titleHints *titleHintRow
}

func newRecipientsModel(svc *service.Services) recipientsModel {
	cols := []table.Column{
		{Title: "KEYID", Width: 20},
		{Title: "NAME", Width: 28},
		{Title: "CREATED", Width: 12},
		{Title: "#SEALED", Width: 8},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(15),
		table.WithStyles(licTableStyles()),
	)
	return recipientsModel{svc: svc, table: t, titleHints: &titleHintRow{}}
}

func listRecipientsCmd(svc *service.Services) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return RecipientsLoadedMsg{}
		}
		rows, err := svc.Recipient.List(context.Background())
		return RecipientsLoadedMsg{Rows: rows, Err: err}
	}
}

func (m recipientsModel) Init() tea.Cmd { return listRecipientsCmd(m.svc) }

func (m recipientsModel) Update(msg tea.Msg) (recipientsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.hgt = msg.Height
		m.rebuildTable()
		return m, nil

	case RecipientsLoadedMsg:
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

		case "n":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("recipient-name", "New Recipient Key", "name (e.g. customer-acme)", 80)}
			}

		case "e":
			// edit (rename) recipient — was missing, 'e' never handled.
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			return m, pushRenameOverlayCmd(OverlayIDRecipientRename, "Rename Recipient", row.Name, 80)

		case "E":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("recipient-export-pub", "Export Public Key", "/path/to/recipient.pub", 256)}
			}

		case "x":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			sub := fmt.Sprintf("Delete recipient key %q?\nThis cannot be undone.", row.Name)
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay(OverlayIDRecipientDelete, "Delete Recipient", sub, "delete", "cancel", true)}
			}

		case "r":
			return m, listRecipientsCmd(m.svc)
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *recipientsModel) selectedRow() *ent.RecipientKey {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rows) {
		return nil
	}
	return m.rows[idx]
}

func (m *recipientsModel) rebuildTable() {
	rows := make([]table.Row, 0, len(m.rows))
	for _, r := range m.rows {
		keyID := fmt.Sprintf("%x", r.PublicKey)
		if len(keyID) > 18 {
			keyID = keyID[:18]
		}
		created := r.CreatedAt.Format("2006-01-02")
		rows = append(rows, table.Row{keyID, r.Name, created, "—"})
	}
	tableH := clampTableHeight(listTableHeight(m.hgt, m.width,
		" Les recipient keys servent à sceller un payload (NaCl box). Le destinataire d'une licence possède la clé privée X25519 et peut déchiffrer le sealed payload."),
		m.detail, len(rows) == 0)
	m.table.SetRows(rows)
	m.table.SetHeight(tableH)
	stretchLastColumn(&m.table, BoxedInner(m.width))
}

// OnClick handles title-bar hint chips + table-row selection.
func (m recipientsModel) OnClick(x, y, _ int) tea.Cmd {
	if cmd := m.titleHints.hit(x, y); cmd != nil {
		return cmd
	}
	return tableRowCmd(m.titleHints.y+1, len(m.rows), y)
}

func (m recipientsModel) View() string {
	intro := Dim.Render(" Les ") + GlowCyan.Render("recipient keys") +
		Dim.Render(" servent à sceller un payload (NaCl box). Le destinataire d'une licence possède la clé privée X25519 et peut déchiffrer le sealed payload.")

	titleLabel := fmt.Sprintf("Recipient keys X25519 (%d)", len(m.rows))
	title := titleBar(m.titleHints, titleLabel, []titleHint{
		{Key: "n", Label: " générer ", Cmd: keyCmd("n")},
		{Key: "i", Label: " importer ", Cmd: keyCmd("i")},
		{Key: "E", Label: " export .pub ", Cmd: keyCmd("E")},
		{Key: "x", Label: " retirer", Cmd: keyCmd("x")},
	}, 0, BoxedInner(m.width))
	introH := lipgloss.Height(lipgloss.NewStyle().Width(m.width).Render(intro))
	m.titleHints.SetY(TopChromeRows + 1 + introH + 1 + 1)

	tableBody := m.table.View()
	if h := emptyTableHint(len(m.rows), m.width, "aucun destinataire — n pour en ajouter un"); h != "" {
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

func (m recipientsModel) renderDetail() string {
	row := m.selectedRow()
	if row == nil {
		return Dim.Render("  no selection")
	}

	pubHex := fmt.Sprintf("%x", row.PublicKey)
	if len(pubHex) > 32 {
		pubHex = pubHex[:32] + "…"
	}

	// Left column: Détail KVs matching recipients.jsx expandedRowRender.
	colW := detailColW(m.width)
	detail := lipgloss.JoinVertical(lipgloss.Left,
		GlowCyan.Render("Détail"),
		kvRow("keyid", GlowCyan.Render(row.ID.String()[:8]+"…"), 10),
		kvRow("name", row.Name, 10),
		kvRow("created", row.CreatedAt.Format("2006-01-02"), 10),
		kvRow("fpr", GlowCyan.Render(pubHex), 10),
		kvRow("sealed", "—", 10),
	)

	// Right column: Actions matching recipients.jsx Actions section.
	actions := lipgloss.JoinVertical(lipgloss.Left,
		GlowCyan.Render("Actions"),
		HintKey.Render("[E]")+" "+Dim.Render("exporter clé publique (.pub) pour l'embarquer"),
		HintKey.Render("[K]")+" "+Dim.Render("exporter clé privée (.key) — réservé au destinataire"),
		GlowRed.Render("[x]")+" "+Dim.Render("retirer (sealed payloads existants inutilisables)"),
	)

	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(colW).Render(detail),
		"  ",
		lipgloss.NewStyle().Width(colW).Render(actions),
	)
	return BoxStyle.Width(BoxedWidth(m.width)).Render(cols)
}

// handleRecipientInputResult processes overlay results for the recipients screen.
func (m recipientsModel) handleRecipientInputResult(res InputResultMsg) (recipientsModel, tea.Cmd) {
	switch res.ID {
	case OverlayIDRecipientRename:
		// Stub rename — Recipient service doesn't expose Rename yet.
		return m, stubRenameResultCmd(res.Value)

	case "recipient-name":
		if m.svc == nil {
			return m, nil
		}
		name := res.Value
		return m, func() tea.Msg {
			_, err := m.svc.Recipient.Generate(context.Background(), name, "operator")
			if err != nil {
				return RecipientsLoadedMsg{Err: err}
			}
			rows, err := m.svc.Recipient.List(context.Background())
			return RecipientsLoadedMsg{Rows: rows, Err: err}
		}

	case "recipient-export-pub":
		row := m.selectedRow()
		if row == nil || m.svc == nil {
			return m, nil
		}
		id := row.ID
		path := res.Value
		return m, func() tea.Msg {
			pub, err := m.svc.Recipient.ExportPublic(context.Background(), id)
			if err != nil {
				return pushOverlayMsg{newErrorOverlay("Export Error", err.Error())}
			}
			if err := os.WriteFile(path, pub, 0o600); err != nil {
				return pushOverlayMsg{newErrorOverlay("Write Error", err.Error())}
			}
			rows, err := m.svc.Recipient.List(context.Background())
			return RecipientsLoadedMsg{Rows: rows, Err: err}
		}
	}
	return m, nil
}

// handleRecipientConfirmResult processes confirm overlay results.
func (m recipientsModel) handleRecipientConfirmResult(res ConfirmResultMsg) (recipientsModel, tea.Cmd) {
	if res.ID == OverlayIDRecipientDelete && res.Confirm {
		row := m.selectedRow()
		if row == nil || m.svc == nil {
			return m, nil
		}
		id := row.ID
		return m, func() tea.Msg {
			err := m.svc.Recipient.Delete(context.Background(), id, "operator")
			if err != nil {
				return RecipientsLoadedMsg{Err: err}
			}
			rows, err := m.svc.Recipient.List(context.Background())
			return RecipientsLoadedMsg{Rows: rows, Err: err}
		}
	}
	return m, nil
}
