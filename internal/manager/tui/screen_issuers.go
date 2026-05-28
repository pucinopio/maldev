package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// ensureExtension appends ext to path when the path does not already end with
// that suffix (case-insensitive). It is the single helper shared by all
// file-export paths that auto-append a file extension.
//
//	ensureExtension("/tmp/key",          ".pub") → "/tmp/key.pub"
//	ensureExtension("/tmp/key.pub",      ".pub") → "/tmp/key.pub"   (no-op)
//	ensureExtension("/tmp/key.pub.pem",  ".pub") → "/tmp/key.pub.pem.pub"
func ensureExtension(path, ext string) string {
	if strings.HasSuffix(strings.ToLower(path), strings.ToLower(ext)) {
		return path
	}
	return path + ext
}

// issuerStatusInline renders a flat, one-line coloured status label for the
// issuers detail panel. The bordered Pill* styles render on 3 rows which
// breaks kvRow's single-line baseline — this helper matches the licStatusPill
// pattern used by the Licenses screen.
func issuerStatusInline(row *ent.Issuer) string {
	switch {
	case row.Active:
		return GlowGreen.Render("● ACTIVE")
	case row.RetiredAt != nil:
		return GlowRed.Render("● RETIRED")
	default:
		return Mute.Render("● INACTIVE")
	}
}

// IssuersLoadedMsg carries the result of fetching all issuers.
type IssuersLoadedMsg struct {
	Rows []*ent.Issuer
	Err  error
}

// issuerImportPickedMsg carries the path returned by the file picker when
// the operator selected an Ed25519 private-key PEM to import.
type issuerImportPickedMsg struct{ path string }

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
		// First column is the active-issuer marker. Header is "ACT" so the
		// operator can read what the column means even when scanning the
		// table for the first time. The marker uses pure ASCII (">>") so
		// it renders in every terminal/font combination — earlier Unicode
		// glyphs (●, ▶) rendered correctly in our golden tests but the
		// operator's terminal failed to display them, hiding the active
		// row signal entirely. Width=5 fits ">>" plus padding.
		{Title: "ACT", Width: 5},
		{Title: "KEYID", Width: 20},
		{Title: "NAME", Width: 24},
		{Title: "STATUS", Width: 10},
		{Title: "CREATED", Width: 12},
		{Title: "#SIGNED", Width: 8},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(15),
		table.WithStyles(licTableStyles()),
	)
	return issuersModel{svc: svc, table: t, titleHints: &titleHintRow{}, detail: true}
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

	case issuersDeletedMsg:
		m.err = msg.err
		if msg.err == nil {
			m.rows = msg.rows
			m.rebuildTable()
			name := msg.name
			return m, func() tea.Msg {
				return pushOverlayMsg{NewOKOverlay("Suppression OK",
					fmt.Sprintf("Issuer %q supprimé.", name))}
			}
		}
		return m, nil

	case issuerImportPickedMsg:
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
			base := filepath.Base(path)
			name := strings.TrimSuffix(base, filepath.Ext(base))
			keyID := fmt.Sprintf("imported-%d", time.Now().Unix())
			if _, err := svc.Issuer.Import(context.Background(), name, keyID, pem, "operator"); err != nil {
				return pushOverlayMsg{newErrorOverlay("Import issuer", err.Error())}
			}
			rows, err := svc.Issuer.List(context.Background())
			return IssuersLoadedMsg{Rows: rows, Err: err}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "d":
			m.detail = !m.detail
			m.rebuildTable()
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

		case "i":
			// Import an Ed25519 private-key PEM from disk. Name defaults to the
			// filename stem; keyID gets a synthetic `imported-<unix>` value so
			// it never collides with a generated key. The operator can rename
			// via [e] afterwards, and re-issue with the correct keyID if the
			// imported key needs to match licences already in the store.
			return m, func() tea.Msg {
				return pushOverlayMsg{newFilePickerOverlay(func(path string) tea.Cmd {
					return func() tea.Msg { return issuerImportPickedMsg{path: path} }
				})}
			}

		case "E":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay(OverlayIDIssuerExportPub, "Export Public Key", "/path/to/issuer.pub.pem", 256)}
			}

		case "e":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			return m, pushRenameOverlayCmd(OverlayIDIssuerRename, "Rename Issuer", row.Name, 64)

		case "K":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			sub := fmt.Sprintf("Export private key for %q?\nThis reveals the signing key — store securely.", row.Name)
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay(OverlayIDIssuerExportPriv, "Export Private Key", sub, "export", "cancel", true)}
			}

		case "x":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			sub := fmt.Sprintf("Retire issuer %q?\nIt will no longer be usable for new licences.", row.Name)
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay(OverlayIDIssuerRetire, "Retire Issuer", sub, "retire", "cancel", true)}
			}

		case "D":
			// Hard-delete an issuer row. IssuerService.Delete refuses if any
			// licence still references it, so the worst case from the UI side
			// is a clean error overlay — the operator gets the count.
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			sub := fmt.Sprintf(
				"Supprimer définitivement l'Issuer %q (key %s) ?\n"+
					"Échoue si des licences le référencent encore — supprime-les\n"+
					"d'abord, ou utilise Retire (x) pour le marquer inactif sans\n"+
					"casser les licences déjà émises.",
				row.Name, row.KeyID)
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay(OverlayIDIssuerDelete, "Supprimer l'Issuer", sub, "supprimer", "annuler", true)}
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
	raw := make([][]string, 0, len(m.rows))
	for _, r := range m.rows {
		// Active-issuer marker. Constraints stacked up across iterations:
		//  - bubbles/table Selected style overrides cell foreground, so
		//    colour alone is unreliable on the cursor row (DS-I03);
		//  - Unicode glyphs (●, ▶) rendered in lipgloss/golden tests but
		//    the operator's terminal couldn't display them at all (DS-I03
		//    follow-up #2). Drop colour AND non-ASCII: ">>" is 2 cells
		//    of pure ASCII that every terminal renders identically and
		//    survives any Selected style override.
		dot := "     "
		if r.Active {
			dot = " >>  "
		}
		status := "inactive"
		if r.Active {
			status = "active"
		} else if r.RetiredAt != nil {
			status = "retired"
		}
		raw = append(raw, []string{
			dot, r.KeyID, r.Name, status, r.CreatedAt.Format("2006-01-02"), "—",
		})
	}
	// Weights ●=fixed dot, KEYID/NAME grow most, STATUS/CREATED/#SIGNED fixed.
	setAutoFitRows(&m.table, BoxedInner(m.width), []int{0, 2, 3, 0, 0, 0}, raw, 60)
	tableH := clampTableHeight(listTableHeight(m.hgt, m.width,
		"Les issuer keys sont les clés Ed25519 qui signent tes licences. Une seule clé est active à la fois ; les autres sont retraitées (retired)."),
		m.detail, len(raw) == 0)
	m.table.SetHeight(tableH)
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
		{Key: "↑↓", Label: " nav ", Cmd: func() tea.Cmd { return nil }},
		{Key: "d", Label: " détail ", Cmd: keyCmd("d")},
		{Key: "n", Label: " générer ", Cmd: keyCmd("n")},
		{Key: "i", Label: " importer ", Cmd: keyCmd("i")},
		{Key: "a", Label: " activer ", Cmd: keyCmd("a")},
		{Key: "E", Label: " export .pub ", Cmd: keyCmd("E")},
		{Key: "x", Label: " retirer ", Cmd: keyCmd("x")},
		{Key: "D", Label: " supprimer", Cmd: keyCmd("D")},
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

func (m issuersModel) renderDetail() string {
	row := m.selectedRow()
	if row == nil {
		hint := Dim.Render("aucune sélection — ") + HintKey.Render("[n]") + Dim.Render(" pour créer une clé")
		return BoxStyle.Width(BoxedWidth(m.width)).Render(
			lipgloss.JoinVertical(lipgloss.Left, Dim.Render("Détail issuer"), "", hint),
		)
	}

	const labelW = 10

	// Left column: canonical kvRow layout matching the Licenses identity tab.
	colW := detailColW(m.width)
	meta := lipgloss.JoinVertical(lipgloss.Left,
		GlowCyan.Render("Métadonnées"),
		kvRow("keyid", GlowCyan.Render(truncate(row.KeyID, colW-labelW)), labelW),
		kvRow("name", truncate(row.Name, colW-labelW), labelW),
		kvRow("status", issuerStatusInline(row), labelW),
		kvRow("created", row.CreatedAt.Format("2006-01-02"), labelW),
		kvRow("db-id", truncate(row.ID.String(), colW-labelW), labelW),
	)

	// Right column: action hints.
	activeLabel := "désigner active"
	if row.Active {
		activeLabel = "déjà active (aucune action)"
	}
	// Use HintKey for every action chip so the [a]/[e]/[E]/[K]/[x] column
	// shares the same leading 1-cell padding. Pre-fix [x] rendered via GlowRed
	// (no padding) and shifted left by 1, breaking the column visually — the
	// destructive nature is now conveyed by foreground colour while keeping
	// the geometry identical.
	hintDanger := HintKey.Foreground(Palette.Red)
	actions := lipgloss.JoinVertical(lipgloss.Left,
		GlowCyan.Render("Actions"),
		HintKey.Render("[a]")+" "+Dim.Render(activeLabel),
		HintKey.Render("[e]")+" "+Dim.Render("renommer"),
		HintKey.Render("[E]")+" "+Dim.Render("exporter clé publique (.pub)"),
		HintKey.Render("[K]")+" "+Dim.Render("exporter clé privée (.priv) — confirmation"),
		hintDanger.Render("[x]")+" "+Dim.Render("retirer (clé reste vérifiable côté binaire)"),
	)

	colStyle := lipgloss.NewStyle().Width(colW)
	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		colStyle.Render(meta),
		"  ",
		colStyle.Render(actions),
	)
	return BoxStyle.Width(BoxedWidth(m.width)).Render(cols)
}

// handleIssuerDeleteConfirm processes the OverlayIDIssuerDelete confirm reply.
// On Confirm=true it calls svc.Issuer.Delete and reloads the list. The service
// refuses if any licence still references the issuer — the resulting error is
// surfaced verbatim via an error overlay so the operator sees the count.
func (m issuersModel) handleIssuerDeleteConfirm(res ConfirmResultMsg) (issuersModel, tea.Cmd) {
	if res.ID != OverlayIDIssuerDelete || !res.Confirm {
		return m, nil
	}
	row := m.selectedRow()
	if row == nil || m.svc == nil {
		return m, nil
	}
	id := row.ID
	name := row.Name
	svc := m.svc
	return m, func() tea.Msg {
		if err := svc.Issuer.Delete(context.Background(), id, "operator"); err != nil {
			return pushOverlayMsg{newErrorOverlay("Suppression refusée", err.Error())}
		}
		rows, err := svc.Issuer.List(context.Background())
		return issuersDeletedMsg{rows: rows, err: err, name: name}
	}
}

// issuersDeletedMsg carries the post-delete list reload + the name of the
// removed issuer for the confirmation toast.
type issuersDeletedMsg struct {
	rows []*ent.Issuer
	err  error
	name string
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

	case OverlayIDIssuerExportPub:
		row := m.selectedRow()
		if row == nil || m.svc == nil {
			return m, nil
		}
		id := row.ID
		path := ensureExtension(res.Value, ".pub")
		return m, func() tea.Msg {
			pem, err := m.svc.Issuer.ExportPublic(context.Background(), id)
			if err != nil {
				return pushOverlayMsg{newErrorOverlay("Export Error", err.Error())}
			}
			if err := os.WriteFile(path, pem, 0o600); err != nil {
				return pushOverlayMsg{newErrorOverlay("Write Error", err.Error())}
			}
			return pushOverlayMsg{NewOKOverlay("Export OK", "Wrote "+path)}
		}

	case OverlayIDIssuerRename:
		if m.selectedRow() == nil || m.svc == nil {
			return m, nil
		}
		return m, stubRenameResultCmd(res.Value)

	case OverlayIDIssuerExportPrivPath:
		row := m.selectedRow()
		if row == nil || m.svc == nil {
			return m, nil
		}
		id := row.ID
		path := ensureExtension(res.Value, ".priv")
		return m, func() tea.Msg {
			pem, err := m.svc.Issuer.ExportPrivate(context.Background(), id)
			if err != nil {
				return pushOverlayMsg{newErrorOverlay("Export Error", err.Error())}
			}
			// 0o600: never group/world-readable. This file is the bytes Import()
			// will accept to register a foreign signing key — anyone holding it
			// can mint licences as this issuer.
			if err := os.WriteFile(path, pem, 0o600); err != nil {
				return pushOverlayMsg{newErrorOverlay("Write Error", err.Error())}
			}
			return pushOverlayMsg{NewOKOverlay("Export OK", "Wrote "+path)}
		}
	}
	return m, nil
}
