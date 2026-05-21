package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	"github.com/oioio-space/maldev/internal/manager/store/ent/setting"
)

// SettingsLoadedMsg carries the result of fetching settings.
type SettingsLoadedMsg struct {
	Row *ent.Setting
	Err error
}

type settingsModel struct {
	svc   *service.Services
	row   *ent.Setting
	err   error
	width int
	hgt   int
}

func newSettingsModel(svc *service.Services) settingsModel {
	return settingsModel{svc: svc}
}

func loadSettingsCmd(svc *service.Services) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return SettingsLoadedMsg{}
		}
		row, err := svc.Settings.Get(context.Background())
		return SettingsLoadedMsg{Row: row, Err: err}
	}
}

func (m settingsModel) Init() tea.Cmd { return loadSettingsCmd(m.svc) }

func (m settingsModel) Update(msg tea.Msg) (settingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.hgt = msg.Height
		return m, nil
	case SettingsLoadedMsg:
		m.err = msg.Err
		m.row = msg.Row
		return m, nil
	case settingsSetThemeMsg:
		// Persist would go through svc.Settings.SetTheme; for now we just
		// acknowledge so the UI feedback proves the click landed. Wire to the
		// real service in a follow-up once the schema lands.
		_ = msg
		return m, loadSettingsCmd(m.svc)

	case settingsSetArgonMsg:
		if m.svc != nil && m.row != nil {
			m.row.DefaultArgonPreset = msg.preset
			// Persistence hook: a follow-up will replace this with
			// svc.Settings.SetArgonPreset(ctx, preset). For now we mutate the
			// in-memory row so the user sees the click reflected immediately.
		}
		return m, nil

	case settingsActionMsg:
		// Action dispatch wires the toolbar + future click handlers to the
		// same code path so the user sees identical behaviour from either input.
		switch msg.kind {
		case "rekey":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("settings-rekey", "Changer la passphrase de la DB", "nouvelle passphrase…", 200)}
			}
		case "vacuum":
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay("settings-vacuum", "VACUUM + ANALYZE", "Lancer VACUUM puis ANALYZE sur la base ?", "lancer", "annuler", false)}
			}
		case "backup":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay("settings-backup", "Backup chiffré", "/path/to/backup.tar.gz.enc", 256)}
			}
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, loadSettingsCmd(m.svc)
		case "P":
			return m, func() tea.Msg { return settingsActionMsg{kind: "rekey"} }
		case "V":
			return m, func() tea.Msg { return settingsActionMsg{kind: "vacuum"} }
		case "B":
			return m, func() tea.Msg { return settingsActionMsg{kind: "backup"} }
		}
	}
	return m, nil
}

// settingsActionMsg is dispatched by both key handlers and OnClick so the
// keyboard and mouse paths converge in one place.
type settingsActionMsg struct{ kind string }

// settingsSetThemeMsg / settingsSetArgonMsg dispatch theme / argon-preset
// changes from clicks; the model handles them in Update.
type settingsSetThemeMsg struct{ idx int } // 1=neon, 2=mono, 3=nord-soft
type settingsSetArgonMsg struct{ preset setting.DefaultArgonPreset }

// OnClick implements ScreenMouseClick for the settings screen. It rebuilds
// the hit map on every call (cheap; settings render is small) and dispatches
// to the matching action. Y is absolute terminal row; chrome occupies rows
// 0..3 so the first settings content row is Y=4.
func (m settingsModel) OnClick(x, y, width int) tea.Cmd {
	h := m.buildHits(width)
	return h.dispatch(x, y)
}

// buildHits computes the click regions for every interactive element in the
// current settings render. Mirrors the layout of renderGrid + box helpers so
// that clicks land on the same cells the user sees.
func (m settingsModel) buildHits(width int) hits {
	if width <= 0 {
		return nil
	}
	colW := (width-1)/2 - 2
	if colW < 18 {
		colW = 18
	}
	// Right column starts at colW+3 (left col width = colW+2, then 1-cell gap).
	rightX := colW + 3
	rightInnerX := rightX + 2 // border(1) + padding(1)

	// Chrome rows: title(1) + tabs(2) + breadcrumb(1) = 4. Content Y=4.
	const chromeRows = 4

	var h hits

	// Right column box layout (constant heights per content):
	//   boxArgonPreset  → 8 rows (top, title, blank, p1, p2, p3, footer, bottom)
	//   boxBaseDeDonnees→ 10 rows (top, title, blank, chemin, passphrase, blank, [P], [V], [B], bottom)
	//   boxApparence    → 8 rows (top, title, blank, theme, t1, t2, t3, bottom)
	argonY := chromeRows                       // 4
	baseY := argonY + 8                        // 12
	apparenceY := baseY + 10                   // 22

	// ── Argon presets (3 clickable rows in argon box) ─────────────────────
	// Rows: 3 (preset 1), 4 (preset 2), 5 (preset 3) inside the box.
	presets := []setting.DefaultArgonPreset{
		setting.DefaultArgonPresetFast,
		setting.DefaultArgonPresetDefault,
		setting.DefaultArgonPresetParanoid,
	}
	for i, p := range presets {
		p := p
		h.add(rightInnerX, argonY+3+i, colW-2, 1, func() tea.Cmd {
			return func() tea.Msg { return settingsSetArgonMsg{preset: p} }
		})
	}

	// ── DB action chips [P] [V] [B] (rows 6, 7, 8 inside base box) ────────
	actions := []string{"rekey", "vacuum", "backup"}
	for i, a := range actions {
		a := a
		h.add(rightInnerX, baseY+6+i, colW-2, 1, func() tea.Cmd {
			return func() tea.Msg { return settingsActionMsg{kind: a} }
		})
	}

	// ── Theme markers in apparence box (row 3 inside the box) ─────────────
	// Layout: "thème : ●  [1]  neon     [2]  mono     [3]  nord-soft"
	// Each marker+label takes ~14 cells; we register 3 overlapping regions
	// roughly aligned with the rendered widths.
	const themeStartOffset = 9 // "thème : " is 8 chars + 1 margin
	themeCellWidth := (colW - 2 - themeStartOffset) / 3
	if themeCellWidth < 8 {
		themeCellWidth = 8
	}
	for i := 1; i <= 3; i++ {
		idx := i
		h.add(rightInnerX+themeStartOffset+(idx-1)*themeCellWidth, apparenceY+3, themeCellWidth, 1, func() tea.Cmd {
			return func() tea.Msg { return settingsSetThemeMsg{idx: idx} }
		})
	}

	return h
}

func (m settingsModel) View() string {
	w := m.width
	if w == 0 {
		w = 120 // safe default when no WindowSizeMsg yet
	}
	if m.err != nil {
		return GlowRed.Render("Error: " + m.err.Error())
	}
	// tui-snap sends one frame before loadSettingsCmd resolves; a zero-value
	// sentinel lets the grid render its chrome structure rather than a blank frame.
	if m.row == nil {
		m.row = &ent.Setting{}
	}
	// Status bar is added by root chrome via Hints().
	return m.renderGrid(w)
}

// renderGrid builds the 2-column grid matching settings.jsx layout.
// Both columns are rendered to the same total height so their bottoms align —
// this prevents the right column from appearing shorter when its boxes have
// different content heights than the left column boxes.
func (m settingsModel) renderGrid(w int) string {
	r := m.row
	// Each settingsBox renders as a box that is colW+2 cells wide (lipgloss
	// adds 1 border on each side outside the style Width). The two columns are
	// joined with " " so total = 2*(colW+2) + 1 ≤ w.
	colW := (w-1)/2 - 2
	if colW < 18 {
		colW = 18
	}

	leftBoxes := []string{
		m.boxDefaultsLicence(colW, r),
		m.boxIdentiteOperateur(colW, r),
		m.boxCycleVieServeurs(colW, r),
		m.boxCascadePassphrase(colW, r),
	}
	rightBoxes := []string{
		m.boxArgonPreset(colW, r),
		m.boxBaseDeDonnees(colW, r),
		m.boxApparence(colW),
	}

	left := lipgloss.JoinVertical(lipgloss.Left, leftBoxes...)
	right := lipgloss.JoinVertical(lipgloss.Left, rightBoxes...)

	// Pad the shorter column to match the taller one so JoinHorizontal
	// produces flush bottoms instead of a ragged staircase.  Width must be
	// set on the padded column so every appended blank line has the correct
	// width and lipgloss.JoinHorizontal sees uniform line lengths.
	lh := lipgloss.Height(left)
	rh := lipgloss.Height(right)
	switch {
	case lh > rh:
		right = lipgloss.NewStyle().Width(colW + 2).Height(lh).Render(right)
	case rh > lh:
		left = lipgloss.NewStyle().Width(colW + 2).Height(rh).Render(left)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

// ── box renderers ────────────────────────────────────────────────────────────

func (m settingsModel) boxDefaultsLicence(w int, r *ent.Setting) string {
	audience := strings.Join(r.DefaultAudience, ", ")
	ttlLabel := fmt.Sprintf("%d s", r.DefaultTTLSeconds)
	if r.DefaultTTLSeconds == 7776000 {
		ttlLabel = "7776000 (90 j)"
	}
	body := strings.Join([]string{
		settingsKV("default_issuer_name", r.DefaultIssuerName),
		settingsKV("default_audience", audience),
		settingsKV("default_ttl_seconds", ttlLabel),
		settingsKV("default_keyid", "active"),
	}, "\n")
	return settingsBox(w, "Defaults licence (wizard nouvelle licence)", body)
}

func (m settingsModel) boxArgonPreset(w int, r *ent.Setting) string {
	presets := []struct {
		key, label string
		v          setting.DefaultArgonPreset
	}{
		{"1", "fast (t=2 m=64MiB p=1)", setting.DefaultArgonPresetFast},
		{"2", "default (t=3 m=256MiB p=2)", setting.DefaultArgonPresetDefault},
		{"3", "paranoid (t=4 m=512MiB p=2)", setting.DefaultArgonPresetParanoid},
	}
	// Borderless preset rows. Embedding bordered chips here breaks the parent
	// box's row count (lipgloss line-wrap on inner borders) and produces the
	// well-known staircase against the left column.
	activeStyle := lipgloss.NewStyle().Foreground(Palette.Green).Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(Palette.FgMute)
	var rows []string
	for _, p := range presets {
		marker := inactiveStyle.Render(" ")
		labelStyle := inactiveStyle
		if r.DefaultArgonPreset == p.v {
			marker = activeStyle.Render("●")
			labelStyle = activeStyle
		}
		rows = append(rows, marker+" "+HintKey.Render("["+p.key+"]")+" "+labelStyle.Render(p.label))
	}
	body := strings.Join(rows, "\n") + "\n" +
		Dim.Render("Coût à la vérification côté binaire. paranoid ≈ 2.5s sur un i7.")
	return settingsBox(w, "default_argon_preset (binding password)", body)
}

func (m settingsModel) boxIdentiteOperateur(w int, r *ent.Setting) string {
	body := settingsKV("operator_name", r.OperatorName) + "\n" +
		Dim.Render("Toutes les entries Audit sont taguées avec cette valeur.")
	return settingsBox(w, "Identité opérateur (audit actor)", body)
}

func (m settingsModel) boxBaseDeDonnees(w int, _ *ent.Setting) string {
	body := strings.Join([]string{
		settingsKV("chemin", "~/.config/license-manager/db.sqlite"),
		settingsKV("passphrase", "résolue via "+GlowCyan.Render("MALDEV_MGR_PASSPHRASE_FILE / env")),
		"",
		HintKey.Render("[P]") + Dim.Render(" changer la passphrase (rekey complet en transaction)"),
		HintKey.Render("[V]") + Dim.Render(" vacuum + analyse"),
		HintKey.Render("[B]") + Dim.Render(" backup chiffré → fichier…"),
	}, "\n")
	return settingsBox(w, "Base de données", body)
}

func (m settingsModel) boxCycleVieServeurs(w int, r *ent.Setting) string {
	sep := max(0, w-6)
	body := GlowCyan.Render("À la fermeture") + "\n" +
		settingsToggle("confirm_quit_with_servers — modal si serveur(s) ON", r.ConfirmQuitWithServers) + "\n" +
		settingsToggle("arrêter tous les serveurs avant de sortir", true) + "\n" +
		Dim.Render(strings.Repeat("─", sep)) + "\n" +
		GlowCyan.Render("Au démarrage") + "\n" +
		settingsToggle("auto_start_servers — démarrer les serveurs au boot", r.AutoStartServers) + "\n" +
		settingsToggle("ouvrir directement Dashboard (défaut)", true)
	return settingsBox(w, "Cycle de vie serveurs HTTP", body)
}

func (m settingsModel) boxApparence(w int) string {
	// Borderless theme markers (same reason as boxArgonPreset).
	activeStyle := lipgloss.NewStyle().Foreground(Palette.Green).Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(Palette.FgMute)
	themes := []string{
		activeStyle.Render("●") + " " + HintKey.Render("[1]") + " " + activeStyle.Render("neon"),
		inactiveStyle.Render(" ") + " " + HintKey.Render("[2]") + " " + inactiveStyle.Render("mono"),
		inactiveStyle.Render(" ") + " " + HintKey.Render("[3]") + " " + inactiveStyle.Render("nord-soft"),
	}
	body := Dim.Render("thème : ") + strings.Join(themes, "  ") + "\n" +
		settingsToggle("bold + couleur saturée (équivalent glow en TUI)", true) + "\n" +
		settingsToggle("densité confort (+1 ligne de padding partout)", false) + "\n" +
		settingsToggle("show timestamps en local au lieu d'UTC", false)
	return settingsBox(w, "Apparence", body)
}

func (m settingsModel) boxCascadePassphrase(w int, _ *ent.Setting) string {
	steps := strings.Join([]string{
		"  1. " + GlowCyan.Render("--passphrase-file") + " <path>",
		"  2. env " + GlowCyan.Render("MALDEV_MGR_PASSPHRASE_FILE"),
		"  3. env " + GlowCyan.Render("MALDEV_MGR_PASSPHRASE"),
		"  4. fallback prompt TUI interactif",
	}, "\n")
	body := Dim.Render("La passphrase est résolue selon la cascade :") + "\n" +
		steps + "\n\n" +
		Dim.Render("Cette session a résolu via : ") + GlowMagent.Render("ENV_FILE") +
		Dim.Render(" → écran passphrase ") + GlowGreen.Render("sauté") + Dim.Render(".")
	return settingsBox(w, "Cascade passphrase au boot (read-only)", body)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func settingsBox(w int, title, body string) string {
	return BoxStyle.Width(w).Render(GlowCyan.Render(title) + "\n\n" + body)
}

// settingsKV renders a dim-key / fg-value row matching the KV component in settings.jsx.
func settingsKV(key, value string) string { return kvRow(key, value, 22) }

// kvRow renders a dim-key / fg-value row with an explicit key field width.
// Shared by settingsKV and detail-panel helpers across screen files.
func kvRow(key, value string, keyW int) string {
	return Dim.Render(fmt.Sprintf("%-*s", keyW, key)) + Base.Render(value)
}

// detailColW returns the width of each column in a 2-column detail panel
// given the total screen width. Ensures a minimum of 20 chars so labels
// are never truncated on narrow terminals.
func detailColW(totalW int) int {
	w := totalW/2 - 4
	if w < 20 {
		return 20
	}
	return w
}

// settingsToggle renders a toggle row: [✓]/[ ] label  on/off.
func settingsToggle(label string, on bool) string {
	indicator, state := Mute.Render("[ ]"), Mute.Render("off")
	if on {
		indicator, state = GlowGreen.Render("[✓]"), GlowGreen.Render("on")
	}
	return indicator + " " + Base.Render(label) + "  " + state
}
