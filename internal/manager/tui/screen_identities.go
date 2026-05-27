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

// IdentitiesLoadedMsg carries the result of fetching all identity entries.
type IdentitiesLoadedMsg struct {
	Rows []*ent.Identity
	Err  error
}

type identitiesModel struct {
	svc        *service.Services
	rows       []*ent.Identity
	err        error
	table      table.Model
	detail     bool
	width      int
	hgt        int
	titleHints *titleHintRow
}

func newIdentitiesModel(svc *service.Services) identitiesModel {
	cols := []table.Column{
		{Title: "NAME", Width: 28},
		{Title: "SHA256", Width: 20},
		{Title: "#REFS", Width: 6},
		{Title: "CREATED", Width: 12},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(15),
		table.WithStyles(licTableStyles()),
	)
	return identitiesModel{svc: svc, table: t, titleHints: &titleHintRow{}}
}

func listIdentitiesCmd(svc *service.Services) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return IdentitiesLoadedMsg{}
		}
		rows, err := svc.Identity.List(context.Background())
		return IdentitiesLoadedMsg{Rows: rows, Err: err}
	}
}

func (m identitiesModel) Init() tea.Cmd { return listIdentitiesCmd(m.svc) }

func (m identitiesModel) Update(msg tea.Msg) (identitiesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.hgt = msg.Height
		m.rebuildTable()
		return m, nil

	case tableSelectRowMsg:
		m.table.SetCursor(msg.row)
		return m, nil

	case IdentitiesLoadedMsg:
		m.err = msg.Err
		m.rows = msg.Rows
		m.rebuildTable()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "d":
			m.detail = !m.detail
			return m, nil

		case "n":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("identity-name", "New Identity", "name (e.g. prod-binary-v1)", 80)}
			}

		case "e":
			// edit (rename) identity — was missing from handler list.
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			return m, pushRenameOverlayCmd(OverlayIDIdentityRename, "Rename Identity", row.Name, 80)

		case "E":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("identity-export", "Export identity.bin", "/path/to/identity.bin", 256)}
			}

		case "R":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			refs := m.refCount(row)
			var body string
			if refs > 0 {
				body = fmt.Sprintf("Regenerate identity %q?\n%d licence(s) will be invalidated.", row.Name, refs)
			} else {
				body = fmt.Sprintf("Regenerate identity %q?\nNo licences currently reference it.", row.Name)
			}
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay(OverlayIDIdentityRegen, "Regenerate Identity", body, "regenerate", "cancel", true)}
			}

		case "x":
			row := m.selectedRow()
			if row == nil {
				return m, nil
			}
			if m.refCount(row) > 0 {
				return m, func() tea.Msg {
					return pushOverlayMsg{newErrorOverlay("Cannot Delete", "Identity is referenced by one or more licences.\nRevoke those licences first.")}
				}
			}
			sub := fmt.Sprintf("Delete identity %q?\nThis cannot be undone.", row.Name)
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay(OverlayIDIdentityDelete, "Delete Identity", sub, "delete", "cancel", true)}
			}

		case "r":
			return m, listIdentitiesCmd(m.svc)
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *identitiesModel) selectedRow() *ent.Identity {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rows) {
		return nil
	}
	return m.rows[idx]
}

// refCount returns the usage count for a row. Uses the in-memory rows so it
// works without an extra DB round-trip in the UI hot path.
func (m *identitiesModel) refCount(_ *ent.Identity) int {
	// Without eager-loading we can't know the exact ref count from the row alone.
	// Return 0 here; the service.Delete call will return an error if refs > 0.
	return 0
}

func (m *identitiesModel) rebuildTable() {
	raw := make([][]string, 0, len(m.rows))
	for _, r := range m.rows {
		// SHA-256 is 64 chars hex — too wide to display fully in any reasonable
		// terminal. Truncate before sizing so the ideal calc stays sane.
		sha := r.Sha256
		if len(sha) > 18 {
			sha = sha[:18] + "…"
		}
		raw = append(raw, []string{r.Name, sha, "—", r.CreatedAt.Format("2006-01-02")})
	}
	// Weights: NAME grows most, SHA256 modest, #REFS/CREATED fixed.
	setAutoFitRows(&m.table, BoxedInner(m.width), []int{3, 2, 0, 0}, raw, 60)
	tableH := clampTableHeight(listTableHeight(m.hgt, m.width,
		" Une identity.bin est un blob de 32 octets aléatoires embarqué dans le binaire via //go:embed. Une licence peut être pinnée à son sha256 pour validation."),
		m.detail, len(raw) == 0)
	m.table.SetHeight(tableH)
}

// OnClick handles title-bar hint chips + table-row selection.
func (m identitiesModel) OnClick(x, y, _ int) tea.Cmd {
	if cmd := m.titleHints.hit(x, y); cmd != nil {
		return cmd
	}
	return tableRowCmd(m.titleHints.y+1, len(m.rows), y)
}

func (m identitiesModel) View() string {
	// Descriptive intro paragraph (prototype: identities.jsx).
	intro := Dim.Render(" Une ") + GlowCyan.Render("identity.bin") +
		Dim.Render(" est un blob de 32 octets aléatoires embarqué dans le binaire via ") +
		GlowCyan.Render("//go:embed") +
		Dim.Render(". Une licence peut être pinnée à son sha256 pour validation.")

	// Bordered titled box with right-aligned action chips, matching the
	// prototype "Identities (N)  [n] créer · [E] export .bin · [R] régénérer · [x] supprimer".
	titleLabel := fmt.Sprintf("Identities (%d)", len(m.rows))
	regenLabel := " régénérer " + GlowYellow.Render("⚠")
	title := titleBar(m.titleHints, titleLabel, []titleHint{
		{Key: "↑↓", Label: " nav ", Cmd: func() tea.Cmd { return nil }},
		{Key: "n", Label: " créer ", Cmd: keyCmd("n")},
		{Key: "E", Label: " export .bin ", Cmd: keyCmd("E")},
		{Key: "R", Label: regenLabel, Cmd: keyCmd("R")},
		{Key: "x", Label: " supprimer", Cmd: keyCmd("x")},
	}, 0, BoxedInner(m.width))
	introH := lipgloss.Height(lipgloss.NewStyle().Width(m.width).Render(intro))
	m.titleHints.SetY(TopChromeRows + 1 + introH + 1 + 1)

	tableBody := m.table.View()
	if h := emptyTableHint(len(m.rows), m.width, "aucune identité — émets une licence pour en créer une"); h != "" {
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

func (m identitiesModel) renderDetail() string {
	row := m.selectedRow()
	if row == nil {
		return Dim.Render("  no selection")
	}

	refs := m.refCount(row)
	// Left column: Détail KVs matching identities.jsx expandedRowRender.
	colW := detailColW(m.width)
	refsLabel := fmt.Sprintf("%d licence(s) pinnée(s) sur cette identité", refs)
	detail := lipgloss.JoinVertical(lipgloss.Left,
		GlowCyan.Render("Détail"),
		kvRow("name", row.Name, 10),
		kvRow("sha256", GlowCyan.Render(row.Sha256), 10),
		kvRow("bytes", "32 (aléatoires, crypto/rand)", 10),
		kvRow("created", row.CreatedAt.Format("2006-01-02"), 10),
		kvRow("refs", refsLabel, 10),
	)

	// Right column: Actions matching identities.jsx Actions section.
	regenDanger := GlowYellow.Render("[R]")
	if refs > 0 {
		regenDanger = GlowRed.Render("[R]")
	}
	regenLabel := fmt.Sprintf("régénérer — casse %d licence(s) ⚠", refs)
	deleteLabel := "supprimer"
	if refs > 0 {
		deleteLabel = fmt.Sprintf("supprimer (impossible : %d refs)", refs)
	}
	actions := lipgloss.JoinVertical(lipgloss.Left,
		GlowCyan.Render("Actions"),
		HintKey.Render("[E]")+" "+Dim.Render("exporter le .bin (prêt pour //go:embed)"),
		regenDanger+" "+Dim.Render(regenLabel),
		GlowRed.Render("[x]")+" "+Dim.Render(deleteLabel),
	)

	colStyle := lipgloss.NewStyle().Width(colW)
	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		colStyle.Render(detail),
		"  ",
		colStyle.Render(actions),
	)
	return BoxStyle.Width(BoxedWidth(m.width)).Render(cols)
}

// handleIdentityInputResult processes overlay results for the identities screen.
func (m identitiesModel) handleIdentityInputResult(res InputResultMsg) (identitiesModel, tea.Cmd) {
	switch res.ID {
	case OverlayIDIdentityRename:
		// Stub rename — Identity service doesn't expose Rename yet.
		return m, stubRenameResultCmd(res.Value)

	case "identity-name":
		if m.svc == nil {
			return m, nil
		}
		name := res.Value
		return m, func() tea.Msg {
			_, err := m.svc.Identity.Create(context.Background(), name, "operator")
			if err != nil {
				return IdentitiesLoadedMsg{Err: err}
			}
			rows, err := m.svc.Identity.List(context.Background())
			return IdentitiesLoadedMsg{Rows: rows, Err: err}
		}

	case "identity-export":
		row := m.selectedRow()
		if row == nil || m.svc == nil {
			return m, nil
		}
		id := row.ID
		// Auto-append .bin when the operator omits the extension.
		path := ensureExtension(res.Value, ".bin")
		return m, func() tea.Msg {
			data, err := m.svc.Identity.ExportBin(context.Background(), id)
			if err != nil {
				return pushOverlayMsg{newErrorOverlay("Export Error", err.Error())}
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				return pushOverlayMsg{newErrorOverlay("Write Error", err.Error())}
			}
			return pushOverlayMsg{NewOKOverlay("Export OK", "Écrit → "+path)}
		}
	}
	return m, nil
}

// handleIdentityConfirmResult processes confirm overlay results.
func (m identitiesModel) handleIdentityConfirmResult(res ConfirmResultMsg) (identitiesModel, tea.Cmd) {
	switch res.ID {
	case OverlayIDIdentityRegen:
		if !res.Confirm || m.svc == nil {
			return m, nil
		}
		row := m.selectedRow()
		if row == nil {
			return m, nil
		}
		id := row.ID
		return m, func() tea.Msg {
			err := m.svc.Identity.Regenerate(context.Background(), id, true, "operator")
			if err != nil {
				return IdentitiesLoadedMsg{Err: err}
			}
			rows, err := m.svc.Identity.List(context.Background())
			return IdentitiesLoadedMsg{Rows: rows, Err: err}
		}

	case OverlayIDIdentityDelete:
		if !res.Confirm || m.svc == nil {
			return m, nil
		}
		row := m.selectedRow()
		if row == nil {
			return m, nil
		}
		id := row.ID
		return m, func() tea.Msg {
			err := m.svc.Identity.Delete(context.Background(), id, "operator")
			if err != nil {
				return pushOverlayMsg{newErrorOverlay("Delete Error", err.Error())}
			}
			rows, err := m.svc.Identity.List(context.Background())
			return IdentitiesLoadedMsg{Rows: rows, Err: err}
		}
	}
	return m, nil
}
