package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// RevocationLoadedMsg carries the result of fetching all revocation entries.
type RevocationLoadedMsg struct {
	Rows []service.RevocationView
	Err  error
}

type revocationModel struct {
	svc        *service.Services
	rows       []service.RevocationView
	err        error
	table      table.Model
	detail     bool
	width      int
	hgt        int
	titleHints *titleHintRow
	// detailLic is the lazy-loaded ent.License backing the currently-selected
	// row. We keep it alongside detailLicID so a re-select can detect a stale
	// cache and re-fetch. nil when not yet loaded.
	detailLic     *ent.License
	detailLicID   string
	detailLoading bool
	detailErr     error
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
		table.WithFocused(true),
		table.WithHeight(15),
		table.WithStyles(licTableStyles()),
	)
	return revocationModel{svc: svc, table: t, titleHints: &titleHintRow{}, detail: true}
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

	case tableSelectRowMsg:
		m.table.SetCursor(msg.row)
		// Cursor moved → invalidate detail cache + reload for the new row.
		m.detailLic = nil
		m.detailLicID = ""
		if m.detail {
			if cmd := m.loadDetailCmd(); cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case RevocationLoadedMsg:
		m.err = msg.Err
		m.rows = msg.Rows
		m.rebuildTable()
		if m.detail {
			if cmd := m.loadDetailCmd(); cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case revocationLicenseLoadedMsg:
		m.detailLoading = false
		m.detailErr = msg.err
		m.detailLic = msg.row
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "d":
			m.detail = !m.detail
			m.rebuildTable()
			if m.detail {
				if cmd := m.loadDetailCmd(); cmd != nil {
					return m, cmd
				}
			}
			return m, nil

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

		case "D":
			// Hard-delete the underlying licence row directly from the
			// revocation list. [x] only unrevokes (status flip); [D] removes
			// the licence entirely and drops it from the CRL.
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			short := row.LicenseUUID
			if len(short) > 12 {
				short = short[:8] + "…"
			}
			sub := fmt.Sprintf(
				"Supprimer définitivement la licence révoquée %q (uuid %s) ?\n"+
					"La ligne et son entrée de révocation seront effacées.\n"+
					"La CRL signée ne référencera plus cet UUID.",
				row.Subject, short)
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay(OverlayIDRevocationDelete, "Supprimer la licence", sub, "supprimer", "annuler", true)}
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
	raw := make([][]string, 0, len(m.rows))
	for _, r := range m.rows {
		raw = append(raw, []string{
			r.Subject,
			r.KeyID,
			r.RevokedAt.Format("2006-01-02"),
			r.Reason,
		})
	}
	// Weights: LICENSE (subject) prioritized, REASON growing, KEYID modest,
	// AT fixed-format.
	setAutoFitRows(&m.table, BoxedInner(m.width), []int{3, 1, 0, 2}, raw, 60)
	tableH := clampTableHeight(listTableHeight(m.hgt, m.width,
		" La CRL (Certificate Revocation List) liste les licences révoquées. Le serveur revocation l'expose en HTTPS pour que les clients vérifient la validité d'une licence.")-5,
		m.detail, len(raw) == 0) // -5 = 3 KPI tile rows (border+content+padding)
	m.table.SetHeight(tableH)
}

// OnClick handles row clicks on the revocation table. Chrome=4 rows; the 3
// OnClick handles title-bar hint chips + table-row selection. Table header
// sits one line below the title row recorded by View().
func (m revocationModel) OnClick(x, y, _ int) tea.Cmd {
	if cmd := m.titleHints.hit(x, y); cmd != nil {
		return cmd
	}
	return tableRowCmd(m.titleHints.y+1, len(m.rows), y)
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

	intro := Dim.Render(" La ") + GlowCyan.Render("CRL") +
		Dim.Render(" (Certificate Revocation List) liste les licences révoquées. Le serveur revocation l'expose en HTTPS pour que les clients vérifient la validité d'une licence.")

	titleLabel := fmt.Sprintf("Revocations (%d)", len(m.rows))
	// Note: [n] ajouter would imply a "create revocation" flow, but revocation
	// is initiated from the Licenses screen (x on a row). [d] détail would
	// imply a per-row detail panel; the row already shows subject/keyID/reason
	// in full so a detail panel adds nothing. Both hints removed to match
	// what's actually wired in Update().
	title := titleBar(m.titleHints, titleLabel, []titleHint{
		{Key: "↑↓", Label: " nav ", Cmd: func() tea.Cmd { return nil }},
		{Key: "d", Label: " détail ", Cmd: keyCmd("d")},
		{Key: "x", Label: " retirer ", Cmd: keyCmd("x")},
		{Key: "D", Label: " supprimer ", Cmd: keyCmd("D")},
		{Key: "E", Label: " export CRL ", Cmd: keyCmd("E")},
		{Key: "r", Label: " rafraîchir", Cmd: keyCmd("r")},
	}, 0, BoxedInner(m.width))
	// Title Y = TopChromeRows + tilesH + blank + introH + blank + box top border.
	tilesH := lipgloss.Height(tilesRow)
	introH := lipgloss.Height(lipgloss.NewStyle().Width(m.width).Render(intro))
	m.titleHints.SetY(TopChromeRows + tilesH + 1 + introH + 1 + 1)

	tableBody := m.table.View()
	if h := emptyTableHint(len(m.rows), m.width, "aucune révocation — la CRL est vide"); h != "" {
		tableBody = lipgloss.JoinVertical(lipgloss.Left, tableBody, "", h)
	}
	body := BoxStyle.Width(BoxedWidth(m.width)).Render(title + "\n" + tableBody)
	body = lipgloss.JoinVertical(lipgloss.Left, "", intro, "", body)
	if m.detail {
		// tilesRow (3 KPI boxes) is prepended below — subtract its height
		// here so the detail clip accounts for it. lipgloss.Height(body) only
		// measures the intro + table box at this point.
		remaining := m.hgt - ContentReservedRows - lipgloss.Height(body) - lipgloss.Height(tilesRow) - 4
		if remaining >= 6 {
			body = lipgloss.JoinVertical(lipgloss.Left, body, clipDetailBox(m.renderDetail(), remaining))
		}
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
	// Truncate footer so it never wraps onto a second line — wrapping
	// makes adjacent tiles uneven in height (one wraps, others don't),
	// which lipgloss.JoinHorizontal then renders as visible misalignment.
	// BoxStyle has Padding(0, 1) → text room = w - 2.
	innerW := w - 2
	if lipgloss.Width(footer) > innerW {
		runes := []rune(footer)
		for lipgloss.Width(string(runes)+"…") > innerW && len(runes) > 0 {
			runes = runes[:len(runes)-1]
		}
		footer = string(runes) + "…"
	}
	inner := lipgloss.JoinVertical(lipgloss.Left,
		Dim.Render(label),
		lipgloss.NewStyle().Foreground(color).Bold(true).Render(value),
		Mute.Render(footer),
	)
	return BoxStyle.Width(w).Render(inner)
}

// revocationLicenseLoadedMsg carries the full ent.License backing the
// currently-selected revocation row.
type revocationLicenseLoadedMsg struct {
	row *ent.License
	err error
}

// loadDetailCmd fetches the License row backing the selected revocation,
// short-circuiting when the cache already matches. Returns nil when there
// is no selection or no service.
func (m *revocationModel) loadDetailCmd() tea.Cmd {
	row := m.selectedRow()
	if row == nil || m.svc == nil {
		return nil
	}
	if m.detailLicID == row.LicenseUUID && m.detailLic != nil {
		return nil
	}
	m.detailLoading = true
	m.detailLicID = row.LicenseUUID
	id := row.LicenseID
	svc := m.svc
	return func() tea.Msg {
		lic, err := svc.License.Get(context.Background(), id)
		return revocationLicenseLoadedMsg{row: lic, err: err}
	}
}

// renderDetail shows the full revocation context: untruncated subject/UUID/
// keyID/reason/timestamp/actor + (lazy-loaded) underlying licence metadata
// like audience/features/bindings/identity pin. Matches the licences detail
// panel layout for consistency.
func (m revocationModel) renderDetail() string {
	row := m.selectedRow()
	if row == nil {
		hint := Dim.Render("aucune sélection — la CRL est vide ou aucune ligne n'est ciblée")
		return BoxStyle.Width(BoxedWidth(m.width)).Render(
			lipgloss.JoinVertical(lipgloss.Left, Dim.Render("Détail révocation"), "", hint),
		)
	}

	const labelW = 12
	valueW := BoxedInner(m.width) - labelW
	if valueW < 12 {
		valueW = 12
	}

	header := Dim.Render("Détail · ") +
		GlowMagent.Render("lic:"+truncate(row.LicenseUUID, 12)) + Dim.Render(" · ") +
		Base.Render(row.Subject)

	revoked := row.RevokedAt.Format("2006-01-02 15:04:05")
	actor := row.RevokedBy
	if actor == "" {
		actor = "—"
	}
	reason := row.Reason
	if reason == "" {
		reason = Mute.Render("(aucune raison enregistrée)")
	}

	left := lipgloss.JoinVertical(lipgloss.Left,
		GlowCyan.Render("Révocation"),
		kvRow("subject", truncate(row.Subject, valueW), labelW),
		kvRow("uuid", GlowCyan.Render(row.LicenseUUID), labelW),
		kvRow("issuer", truncate(row.KeyID, valueW), labelW),
		kvRow("revoked at", revoked, labelW),
		kvRow("revoked by", actor, labelW),
		kvRow("reason", reason, labelW),
	)

	// Right column: lazy-loaded license context (audience/features/bindings).
	right := m.renderDetailLicenseContext(labelW, valueW)

	colW := detailColW(m.width)
	colStyle := lipgloss.NewStyle().Width(colW)
	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		colStyle.Render(left), "  ", colStyle.Render(right),
	)
	return BoxStyle.Width(BoxedWidth(m.width)).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, "", cols),
	)
}

// renderDetailLicenseContext shows audience/features/bindings/identity-pin
// from the lazy-loaded License row. Three states: loading, error, ready.
func (m revocationModel) renderDetailLicenseContext(labelW, valueW int) string {
	title := GlowCyan.Render("Licence sous-jacente")
	switch {
	case m.detailLoading:
		return lipgloss.JoinVertical(lipgloss.Left, title, "", Dim.Render("  chargement…"))
	case m.detailErr != nil:
		return lipgloss.JoinVertical(lipgloss.Left, title, "", GlowRed.Render("  erreur : "+m.detailErr.Error()))
	case m.detailLic == nil:
		return lipgloss.JoinVertical(lipgloss.Left, title, "", Dim.Render("  (non chargée)"))
	}
	lic := m.detailLic
	identity := "—"
	if lic.IdentitySha256 != "" {
		identity = GlowCyan.Render(lic.IdentitySha256[:min(16, len(lic.IdentitySha256))] + "…")
	}
	binary := "—"
	if lic.BinarySha256 != "" {
		binary = GlowCyan.Render(lic.BinarySha256[:min(16, len(lic.BinarySha256))] + "…")
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		kvRow("audience", truncate(strings.Join(lic.Audience, ", "), valueW), labelW),
		kvRow("features", truncate(strings.Join(lic.Features, ", "), valueW), labelW),
		kvRow("not-before", lic.NotBefore.Format("2006-01-02"), labelW),
		kvRow("not-after", lic.NotAfter.Format("2006-01-02"), labelW),
		kvRow("payload", string(lic.PayloadKind), labelW),
		kvRow("identity", identity, labelW),
		kvRow("binary", binary, labelW),
	)
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
	if res.ID == OverlayIDRevocationDelete && res.Confirm {
		row := m.selectedRow()
		if row == nil || m.svc == nil {
			return m, nil
		}
		id := row.LicenseID
		svc := m.svc
		return m, func() tea.Msg {
			if err := svc.License.Delete(context.Background(), id, "operator"); err != nil {
				return pushOverlayMsg{newErrorOverlay("Suppression échouée", err.Error())}
			}
			rows, err := svc.Revoke.ListRevoked(context.Background())
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

