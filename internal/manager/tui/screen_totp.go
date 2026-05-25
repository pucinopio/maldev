package tui

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// TOTPLoadedMsg carries the result of listing TOTP secrets.
type TOTPLoadedMsg struct {
	Rows []*ent.TOTPSecret
	Err  error
}

// totpDetailLoadedMsg carries the decrypted view of a single TOTP secret.
type totpDetailLoadedMsg struct {
	View *service.TOTPSecretView
	Err  error
}

// totpQRExportedMsg signals a PNG export completed (or failed).
type totpQRExportedMsg struct {
	Path string
	Err  error
}

type totpModel struct {
	svc        *service.Services
	rows       []*ent.TOTPSecret
	view       *service.TOTPSecretView // decrypted view of the cursor row
	err        error
	table      table.Model
	vp         viewport.Model
	width      int
	hgt        int
	titleHints *titleHintRow
}

func newTOTPModel(svc *service.Services) totpModel {
	cols := []table.Column{
		{Title: "LABEL", Width: 32},
		{Title: "ID", Width: 12},
		{Title: "CREATED", Width: 12},
		{Title: "BOUND TO", Width: 20},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(false),
		table.WithHeight(8),
		table.WithStyles(licTableStyles()),
	)
	vp := viewport.New(0, 8)
	return totpModel{svc: svc, table: t, vp: vp, titleHints: &titleHintRow{}}
}

func listTOTPCmd(svc *service.Services) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return TOTPLoadedMsg{}
		}
		rows, err := svc.TOTP.List(context.Background())
		return TOTPLoadedMsg{Rows: rows, Err: err}
	}
}

func loadTOTPDetailCmd(svc *service.Services, id uuid.UUID, issuerName string) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return totpDetailLoadedMsg{}
		}
		v, err := svc.TOTP.ByID(context.Background(), id, issuerName)
		return totpDetailLoadedMsg{View: v, Err: err}
	}
}

func (m totpModel) Init() tea.Cmd { return listTOTPCmd(m.svc) }

func (m totpModel) Update(msg tea.Msg) (totpModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.hgt = msg.Height
		m.rebuildTable()
		return m, nil

	case TOTPLoadedMsg:
		m.err = msg.Err
		m.rows = msg.Rows
		m.view = nil
		m.rebuildTable()
		return m, m.loadCursorDetail()

	case totpDetailLoadedMsg:
		if msg.Err == nil {
			m.view = msg.View
			m.vp.SetContent(m.renderQR())
		}
		return m, nil

	case totpQRExportedMsg:
		if msg.Err != nil {
			return m, func() tea.Msg { return pushOverlayMsg{newErrorOverlay("Export failed", msg.Err.Error())} }
		}
		return m, func() tea.Msg {
			return pushOverlayMsg{NewOKOverlay("QR exported", "Saved to "+msg.Path)}
		}

	case tableSelectRowMsg:
		m.table.SetCursor(msg.row)
		return m, m.loadCursorDetail()

	case tea.KeyMsg:
		switch msg.String() {
		case "n":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("totp-label", "New TOTP secret",
					"account label (e.g. user@app)", 80)}
			}
		case "x":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			label := row.AccountLabel
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay("totp-delete",
					"Delete TOTP secret",
					"Supprimer le secret TOTP \""+label+"\" ?\nLes licences déjà émises avec ce secret continueront à valider.",
					"supprimer", "annuler", true)}
			}
		case "E":
			if m.view == nil || m.view.QRImagePNG == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("totp-export-png", "Export QR PNG",
					"/path/to/qr.png", 256)}
			}
		case "r":
			return m, listTOTPCmd(m.svc)
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	// Cursor move → reload detail for the new selection.
	return m, tea.Batch(cmd, m.loadCursorDetail())
}

func (m totpModel) loadCursorDetail() tea.Cmd {
	row := m.selectedRow()
	if row == nil {
		return nil
	}
	return loadTOTPDetailCmd(m.svc, row.ID, "license-manager")
}

func (m *totpModel) selectedRow() *ent.TOTPSecret {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rows) {
		return nil
	}
	return m.rows[idx]
}

func (m *totpModel) rebuildTable() {
	rows := make([]table.Row, 0, len(m.rows))
	for _, r := range m.rows {
		shortID := r.ID.String()
		if len(shortID) > 12 {
			shortID = shortID[:8] + "…"
		}
		bound := "—"
		// The license_totps FK is populated when the secret was issued via the
		// wizard; standalone secrets show "—".
		// (Direct read avoids an extra eager-load query for the list view.)
		created := r.CreatedAt.Format("2006-01-02")
		rows = append(rows, table.Row{r.AccountLabel, shortID, created, bound})
	}
	tableH := listTableHeight(m.hgt, m.width,
		" Les secrets TOTP créés ici sont réutilisables — la liste apparaît dans le wizard step 7. Suppression libère le secret mais n'invalide pas les licences qui en dépendent.")
	if tableH < 3 {
		tableH = 3
	}
	tableH /= 2 // reserve half the budget for the detail/QR panel
	if tableH < 3 {
		tableH = 3
	}
	if len(rows) == 0 {
		tableH = 1
	}
	m.table.SetRows(rows)
	m.table.SetHeight(tableH)
	stretchLastColumn(&m.table, BoxedInner(m.width))
	m.vp.Width = BoxedInner(m.width)
	m.vp.Height = tableH
	if m.view != nil {
		m.vp.SetContent(m.renderQR())
	}
}

// OnClick handles title-bar hint chips + table-row selection. Header row
// sits one line below the title row recorded by View().
func (m totpModel) OnClick(x, y, _ int) tea.Cmd {
	if cmd := m.titleHints.hit(x, y); cmd != nil {
		return cmd
	}
	return tableRowCmd(m.titleHints.y+1, len(m.rows), y)
}

func (m totpModel) View() string {
	intro := Dim.Render(" Les ") + GlowCyan.Render("secrets TOTP") +
		Dim.Render(" créés ici sont réutilisables — la liste apparaît dans le wizard step 7. Suppression libère le secret mais n'invalide pas les licences qui en dépendent.")

	titleLabel := fmt.Sprintf("TOTP secrets (%d)", len(m.rows))
	// Compute the layout mode + list-box inner width FIRST so titleBar sizes
	// the hint strip to whatever the list-box can actually fit. Otherwise
	// the title wraps mid-render and the click hit-test for the table
	// header lands a row above the real header.
	totalW := m.width - 4
	const minDetailW = 36
	const minListW = 50
	twoCol := totalW >= minListW+2+minDetailW
	var listBoxOuterW int
	if twoCol {
		rightW := minDetailW
		if rightW < totalW*45/100 {
			rightW = totalW * 45 / 100
		}
		listBoxOuterW = totalW - rightW - 2
	} else {
		listBoxOuterW = BoxedWidth(m.width)
	}
	listInnerW := listBoxOuterW - BoxStyle.GetHorizontalFrameSize() + BoxStyle.GetHorizontalBorderSize()
	title := titleBar(m.titleHints, titleLabel, []titleHint{
		{Key: "n", Label: " générer ", Cmd: keyCmd("n")},
		{Key: "E", Label: " export QR PNG ", Cmd: keyCmd("E")},
		{Key: "x", Label: " supprimer ", Cmd: keyCmd("x")},
		{Key: "r", Label: " rafraîchir", Cmd: keyCmd("r")},
	}, 0, listInnerW)
	introH := lipgloss.Height(lipgloss.NewStyle().Width(m.width).Render(intro))
	// Title may still wrap if listInnerW is small — measure its actual height
	// so the headerY click target lands on the row AFTER the wrap.
	titleH := lipgloss.Height(lipgloss.NewStyle().Width(listInnerW).Render(title))
	m.titleHints.SetY(TopChromeRows + 1 + introH + 1 + titleH)

	tableBody := m.table.View()
	if h := emptyTableHint(len(m.rows), m.width, "aucun secret TOTP — n pour créer le premier"); h != "" {
		tableBody = lipgloss.JoinVertical(lipgloss.Left, tableBody, "", h)
	}
	var body string
	if twoCol {
		rightW := minDetailW
		if rightW < totalW*45/100 {
			rightW = totalW * 45 / 100
		}
		listBox := BoxStyle.Width(listBoxOuterW).Render(title + "\n" + tableBody)
		detailBox := m.renderDetailSide(rightW)
		twoColView := lipgloss.JoinHorizontal(lipgloss.Top, listBox, "  ", detailBox)
		body = lipgloss.JoinVertical(lipgloss.Left, "", intro, "", twoColView)
	} else {
		listBox := BoxStyle.Width(listBoxOuterW).Render(title + "\n" + tableBody)
		detailBox := m.renderDetailSide(BoxedWidth(m.width))
		body = lipgloss.JoinVertical(lipgloss.Left, "", intro, "", listBox, detailBox)
	}
	if m.err != nil {
		body = GlowRed.Render("Error: "+m.err.Error()) + "\n" + body
	}
	return body
}

func (m totpModel) renderDetailSide(w int) string {
	if m.view == nil {
		title := Dim.Render("QR code · ") + Dim.Render("aucune sélection")
		hint := Dim.Render("crée un secret avec ") + HintKey.Render("[n]") +
			Dim.Render(" ou sélectionne une ligne.")
		return BoxStyle.Width(w).Render(
			lipgloss.JoinVertical(lipgloss.Left, title, "", hint),
		)
	}
	title := Dim.Render("QR · ") + GlowCyan.Render(m.view.AccountLabel)
	// Inline the QR ASCII so the side panel doesn't depend on the viewport
	// width — keeps the half-block grid intact.
	qr := m.view.QRImageASCII
	hint := HintKey.Render("[E]") + Dim.Render(" PNG  ·  ") +
		Dim.Render("secret: ") + Mute.Render(m.view.Secret)
	return BoxFocused.Width(w).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, "", qr, "", hint),
	)
}

// Hints implements ScreenWithHints — drives the bottom status bar.
func (m totpModel) Hints() []string {
	return []string{
		"n nouveau TOTP",
		"E export QR PNG",
		"x supprimer",
		"r rafraîchir",
		"q quitter",
	}
}

// renderQR returns the ASCII QR + the otpauth URI, ready for the viewport.
func (m totpModel) renderQR() string {
	if m.view == nil {
		return ""
	}
	return m.view.QRImageASCII + "\n\n" + Dim.Render(m.view.OtpauthURI)
}

// handleTOTPInputResult routes overlay text input to the right action.
func (m totpModel) handleTOTPInputResult(res InputResultMsg) (totpModel, tea.Cmd) {
	switch res.ID {
	case "totp-label":
		if m.svc == nil || res.Value == "" {
			return m, nil
		}
		label := res.Value
		return m, func() tea.Msg {
			_, _, err := m.svc.TOTP.Generate(context.Background(), label)
			if err != nil {
				return TOTPLoadedMsg{Err: err}
			}
			rows, err := m.svc.TOTP.List(context.Background())
			return TOTPLoadedMsg{Rows: rows, Err: err}
		}
	case "totp-export-png":
		if m.view == nil || m.view.QRImagePNG == nil {
			return m, nil
		}
		path := res.Value
		png := m.view.QRImagePNG
		return m, func() tea.Msg {
			err := os.WriteFile(path, png, 0o600)
			return totpQRExportedMsg{Path: path, Err: err}
		}
	}
	return m, nil
}

// handleTOTPConfirmResult routes the confirm overlay back to the screen.
func (m totpModel) handleTOTPConfirmResult(res ConfirmResultMsg) (totpModel, tea.Cmd) {
	if res.ID != "totp-delete" || !res.Confirm {
		return m, nil
	}
	row := m.selectedRow()
	if row == nil || m.svc == nil {
		return m, nil
	}
	id := row.ID
	return m, func() tea.Msg {
		if err := m.svc.TOTP.Delete(context.Background(), id); err != nil {
			return TOTPLoadedMsg{Err: err}
		}
		rows, err := m.svc.TOTP.List(context.Background())
		return TOTPLoadedMsg{Rows: rows, Err: err}
	}
}
