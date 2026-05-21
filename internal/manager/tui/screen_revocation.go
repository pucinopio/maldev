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
)

// RevocationLoadedMsg carries the result of fetching all revocation entries.
type RevocationLoadedMsg struct {
	Rows []service.RevocationView
	Err  error
}

type revocationModel struct {
	svc   *service.Services
	rows  []service.RevocationView
	err   error
	table table.Model
	width int
	hgt   int
}

func newRevocationModel(svc *service.Services) revocationModel {
	cols := []table.Column{
		{Title: "LICENSE", Width: 22},
		{Title: "KEYID", Width: 18},
		{Title: "AT", Width: 12},
		{Title: "REASON", Width: 30},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(false),
		table.WithHeight(15),
		table.WithStyles(licTableStyles()),
	)
	return revocationModel{svc: svc, table: t}
}

func listRevocationCmd(svc *service.Services) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return RevocationLoadedMsg{}
		}
		rows, err := svc.Revoke.ListRevoked(context.Background())
		return RevocationLoadedMsg{Rows: rows, Err: err}
	}
}

func (m revocationModel) Init() tea.Cmd { return listRevocationCmd(m.svc) }

func (m revocationModel) Update(msg tea.Msg) (revocationModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.hgt = msg.Height
		m.rebuildTable()
		return m, nil

	case RevocationLoadedMsg:
		m.err = msg.Err
		m.rows = msg.Rows
		m.rebuildTable()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "x":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			sub := fmt.Sprintf("Remove revocation for %q?\nLicense will become active again.", row.Subject)
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay("revocation-remove", "Remove Revocation", sub, "remove", "cancel", true)}
			}

		case "E":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("revocation-export", "Export Signed CRL", "/path/to/crl.pem", 256)}
			}

		case "r":
			return m, listRevocationCmd(m.svc)
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *revocationModel) selectedRow() *service.RevocationView {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rows) {
		return nil
	}
	return &m.rows[idx]
}

func (m *revocationModel) rebuildTable() {
	rows := make([]table.Row, 0, len(m.rows))
	for _, r := range m.rows {
		at := r.RevokedAt.Format("2006-01-02")
		subject := r.Subject
		if len(subject) > 20 {
			subject = subject[:19] + "…"
		}
		reason := r.Reason
		if len(reason) > 28 {
			reason = reason[:27] + "…"
		}
		rows = append(rows, table.Row{subject, r.KeyID, at, reason})
	}
	tableH := m.hgt - 6
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

func (m revocationModel) View() string {
	// 3*(tileW+2)+2 ≤ m.width: lipgloss adds 1 border per side outside Width,
	// and 2 separator spaces sit between the 3 tiles.
	tileW := (m.width-2)/3 - 2
	if tileW < 14 {
		tileW = 14
	}
	entriesTile := revocInfoTile("Entries CRL",
		fmt.Sprintf("%d", len(m.rows)),
		"révocations signées", Palette.Red, tileW)
	pushedTile := revocInfoTile("Pushed via :8443", "oui",
		"serveur révocation en ligne", Palette.Green, tileW)
	exportTile := revocInfoTile("Dernier export", "—",
		"manager.crl.pem (offline copy)", Palette.FgDim, tileW)

	tilesRow := lipgloss.JoinHorizontal(lipgloss.Top,
		entriesTile, " ", pushedTile, " ", exportTile)

	body := m.table.View()
	if hint := emptyTableHint(len(m.rows), m.width, "aucune révocation — la CRL est vide"); hint != "" {
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", hint)
	}
	if m.err != nil {
		body = GlowRed.Render("Error: "+m.err.Error()) + "\n" + body
	}
	// Status bar comes from root chrome via Hints() — don't duplicate.
	return lipgloss.JoinVertical(lipgloss.Left, tilesRow, body)
}

// revocInfoTile renders a small bordered tile with a string value for
// the revocation screen header (used for non-numeric summary values).
func revocInfoTile(label, value, footer string, color lipgloss.Color, w int) string {
	inner := lipgloss.JoinVertical(lipgloss.Left,
		Dim.Render(label),
		lipgloss.NewStyle().Foreground(color).Bold(true).Render(value),
		Mute.Render(footer),
	)
	return BoxStyle.Width(w).Render(inner)
}

// handleRevocationConfirmResult processes confirm overlay results.
func (m revocationModel) handleRevocationConfirmResult(res ConfirmResultMsg) (revocationModel, tea.Cmd) {
	if res.ID == "revocation-remove" && res.Confirm {
		row := m.selectedRow()
		if row == nil || m.svc == nil {
			return m, nil
		}
		id := row.LicenseID
		return m, func() tea.Msg {
			err := m.svc.Revoke.Unrevoke(context.Background(), id, "operator")
			if err != nil {
				return RevocationLoadedMsg{Err: err}
			}
			rows, err := m.svc.Revoke.ListRevoked(context.Background())
			return RevocationLoadedMsg{Rows: rows, Err: err}
		}
	}
	return m, nil
}

// handleRevocationInputResult processes input overlay results.
func (m revocationModel) handleRevocationInputResult(res InputResultMsg) (revocationModel, tea.Cmd) {
	if res.ID == "revocation-export" {
		if m.svc == nil {
			return m, nil
		}
		path := res.Value
		return m, func() tea.Msg {
			pem, err := m.svc.Revoke.PublishSignedList(context.Background(), 7*24*time.Hour)
			if err != nil {
				return pushOverlayMsg{newErrorOverlay("Export Error", err.Error())}
			}
			if err := os.WriteFile(path, pem, 0o644); err != nil {
				return pushOverlayMsg{newErrorOverlay("Write Error", err.Error())}
			}
			rows, err := m.svc.Revoke.ListRevoked(context.Background())
			return RevocationLoadedMsg{Rows: rows, Err: err}
		}
	}
	return m, nil
}

