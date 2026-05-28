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
		// Apply the persisted theme + apparence so a restart shows the
		// same palette/weight/density/time-zone the operator picked last
		// session. Pre-fix the toggles persisted correctly but the
		// runtime helpers ignored them until restart anyway.
		if m.row != nil {
			ApplyTheme(string(m.row.Theme))
			ApplyApparence(m.row.BoldSaturated, m.row.ComfortDensity, m.row.TimestampsLocal)
		}
		return m, nil
	case settingsSetThemeMsg:
		themeMap := []setting.Theme{
			setting.ThemeNeon,
			setting.ThemeMono,
			setting.ThemeNordSoft,
		}
		idx := msg.idx - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(themeMap) {
			idx = len(themeMap) - 1
		}
		t := themeMap[idx]
		if m.row != nil {
			m.row.Theme = t
		}
		// Swap the runtime palette so the new colours appear on the very next
		// render — without this the change only became visible after a restart.
		ApplyTheme(string(t))
		return m, settingsPersistCmd(m.svc, func(q *ent.SettingUpdateOne) {
			q.SetTheme(t)
		})

	case settingsToggleMsg:
		// Toggle in-memory first so the UI updates immediately, then persist
		// via the async svc.Settings.Update path.
		if m.row != nil {
			switch msg.key {
			case "confirm_quit_with_servers":
				m.row.ConfirmQuitWithServers = !m.row.ConfirmQuitWithServers
				v := m.row.ConfirmQuitWithServers
				return m, settingsPersistCmd(m.svc, func(q *ent.SettingUpdateOne) {
					q.SetConfirmQuitWithServers(v)
				})
			case "auto_start_servers":
				m.row.AutoStartServers = !m.row.AutoStartServers
				v := m.row.AutoStartServers
				return m, settingsPersistCmd(m.svc, func(q *ent.SettingUpdateOne) {
					q.SetAutoStartServers(v)
				})
			case "stop_servers_on_exit":
				m.row.StopServersOnExit = !m.row.StopServersOnExit
				v := m.row.StopServersOnExit
				return m, settingsPersistCmd(m.svc, func(q *ent.SettingUpdateOne) {
					q.SetStopServersOnExit(v)
				})
			case "bold_saturated":
				m.row.BoldSaturated = !m.row.BoldSaturated
				v := m.row.BoldSaturated
				ApplyApparence(v, m.row.ComfortDensity, m.row.TimestampsLocal)
				return m, settingsPersistCmd(m.svc, func(q *ent.SettingUpdateOne) {
					q.SetBoldSaturated(v)
				})
			case "comfort_density":
				m.row.ComfortDensity = !m.row.ComfortDensity
				v := m.row.ComfortDensity
				ApplyApparence(m.row.BoldSaturated, v, m.row.TimestampsLocal)
				return m, settingsPersistCmd(m.svc, func(q *ent.SettingUpdateOne) {
					q.SetComfortDensity(v)
				})
			case "timestamps_local":
				m.row.TimestampsLocal = !m.row.TimestampsLocal
				v := m.row.TimestampsLocal
				ApplyApparence(m.row.BoldSaturated, m.row.ComfortDensity, v)
				return m, settingsPersistCmd(m.svc, func(q *ent.SettingUpdateOne) {
					q.SetTimestampsLocal(v)
				})
			}
		}
		return m, nil

	case settingsSetArgonMsg:
		// Mutate in-memory + persist.
		if m.row != nil {
			m.row.DefaultArgonPreset = msg.preset
			p := msg.preset
			return m, settingsPersistCmd(m.svc, func(q *ent.SettingUpdateOne) {
				q.SetDefaultArgonPreset(p)
			})
		}
		return m, nil

	case settingsPersistedMsg:
		// Persistence completed. On error, surface via OK overlay; otherwise
		// the in-memory row already reflects the new value.
		if msg.err != nil {
			return m, func() tea.Msg {
				return pushOverlayMsg{newErrorOverlay("Persist failed", msg.err.Error())}
			}
		}
		return m, nil

	case settingsActionMsg:
		// Action dispatch wires the toolbar + future click handlers to the
		// same code path so the user sees identical behaviour from either input.
		switch msg.kind {
		case "rekey":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay(OverlayIDSettingsRekey, "Changer la passphrase de la DB", "nouvelle passphrase…", 200)}
			}
		case "vacuum":
			return m, func() tea.Msg {
				return pushOverlayMsg{newConfirmOverlay(OverlayIDSettingsVacuum, "VACUUM + ANALYZE", "Lancer VACUUM puis ANALYZE sur la base ?", "lancer", "annuler", false)}
			}
		case "backup":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay(OverlayIDSettingsBackup, "Backup chiffré", "/path/to/backup.tar.gz.enc", 256)}
			}
		case "restore":
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay(OverlayIDSettingsRestore, "Restaurer un backup", "/path/to/backup.tar.gz.enc", 256)}
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
		// Argon preset hotkeys
		case "1":
			return m, func() tea.Msg { return settingsSetArgonMsg{preset: setting.DefaultArgonPresetFast} }
		case "2":
			return m, func() tea.Msg { return settingsSetArgonMsg{preset: setting.DefaultArgonPresetDefault} }
		case "3":
			return m, func() tea.Msg { return settingsSetArgonMsg{preset: setting.DefaultArgonPresetParanoid} }
		// Theme hotkeys — capital N/M/O to avoid colliding with lower-case
		// shortcuts already claimed by other screens.
		case "N":
			return m, func() tea.Msg { return settingsSetThemeMsg{idx: 1} }
		case "M":
			return m, func() tea.Msg { return settingsSetThemeMsg{idx: 2} }
		case "O":
			return m, func() tea.Msg { return settingsSetThemeMsg{idx: 3} }
		// Toggle hotkeys
		case "Q":
			return m, func() tea.Msg { return settingsToggleMsg{key: "confirm_quit_with_servers"} }
		case "U":
			return m, func() tea.Msg { return settingsToggleMsg{key: "auto_start_servers"} }
		case "S":
			return m, func() tea.Msg { return settingsToggleMsg{key: "stop_servers_on_exit"} }
		case "G":
			return m, func() tea.Msg { return settingsToggleMsg{key: "bold_saturated"} }
		case "D":
			return m, func() tea.Msg { return settingsToggleMsg{key: "comfort_density"} }
		case "T":
			return m, func() tea.Msg { return settingsToggleMsg{key: "timestamps_local"} }
		case "I":
			return m, func() tea.Msg { return settingsActionMsg{kind: "restore"} }
		}
	}
	return m, nil
}

// settingsActionMsg is dispatched by both key handlers and OnClick so the
// keyboard and mouse paths converge in one place.
type settingsActionMsg struct{ kind string }

// settingsPersistedMsg carries the result of an async Settings update.
type settingsPersistedMsg struct{ err error }

// settingsPersistCmd builds a Cmd that applies mut() to the singleton Setting
// row and reports the result via settingsPersistedMsg.
func settingsPersistCmd(svc *service.Services, mut func(*ent.SettingUpdateOne)) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return settingsPersistedMsg{}
		}
		_, err := svc.Settings.Update(context.Background(), mut)
		return settingsPersistedMsg{err: err}
	}
}

// settingsSetThemeMsg / settingsSetArgonMsg dispatch theme / argon-preset
// changes from clicks; the model handles them in Update.
type settingsSetThemeMsg struct{ idx int } // 1=neon, 2=mono, 3=nord-soft
type settingsSetArgonMsg struct{ preset setting.DefaultArgonPreset }
type settingsToggleMsg struct{ key string }

// OnClick implements ScreenMouseClick for the settings screen. Hit boxes
// are computed by SCANNING the rendered View() rather than hardcoding row
// offsets — too many things drift the offsets (toggle-label wrapping on
// narrow widths, new action lines like [I] import, extra apparence
// toggles), so any hardcoded mapping eventually goes stale.
func (m settingsModel) OnClick(x, y, width int) tea.Cmd {
	if width <= 0 {
		return nil
	}
	// Root passes absolute screen coords; m.View() returns the settings
	// grid without the chrome rows (tab bar + status). Subtract so the
	// line index matches what the operator clicked.
	yLocal := y - ChromeRows
	lines := strings.Split(m.View(), "\n")
	if yLocal < 0 || yLocal >= len(lines) {
		return nil
	}
	line := lines[yLocal]

	// Half-width: anything past the gap column belongs to the right pane;
	// otherwise the left pane.
	half := width / 2

	// Each entry maps a label substring to the cmd it dispatches. The
	// `right` bool says which pane the label is expected to live in
	// (prevents a "rekey" hit on the left pane from a hypothetical box
	// that mentions the word "rekey").
	type spec struct {
		label string
		right bool
		make  func() tea.Cmd
	}
	specs := []spec{
		// Argon presets (right column).
		{"[1]", true, func() tea.Cmd {
			return func() tea.Msg { return settingsSetArgonMsg{preset: setting.DefaultArgonPresetFast} }
		}},
		{"[2]", true, func() tea.Cmd {
			return func() tea.Msg { return settingsSetArgonMsg{preset: setting.DefaultArgonPresetDefault} }
		}},
		{"[3]", true, func() tea.Cmd {
			return func() tea.Msg { return settingsSetArgonMsg{preset: setting.DefaultArgonPresetParanoid} }
		}},
		// DB actions.
		{"[P]", true, func() tea.Cmd { return func() tea.Msg { return settingsActionMsg{kind: "rekey"} } }},
		{"[V]", true, func() tea.Cmd { return func() tea.Msg { return settingsActionMsg{kind: "vacuum"} } }},
		{"[B]", true, func() tea.Cmd { return func() tea.Msg { return settingsActionMsg{kind: "backup"} } }},
		{"[I]", true, func() tea.Cmd { return func() tea.Msg { return settingsActionMsg{kind: "restore"} } }},
		// Apparence (right column). Theme markers live on the same line
		// so we hit-test by X further down for those. Toggles take a
		// whole row, so the label-substring match is enough.
		{"bold_saturated", true, func() tea.Cmd {
			return func() tea.Msg { return settingsToggleMsg{key: "bold_saturated"} }
		}},
		{"comfort_density", true, func() tea.Cmd {
			return func() tea.Msg { return settingsToggleMsg{key: "comfort_density"} }
		}},
		{"timestamps_local", true, func() tea.Cmd {
			return func() tea.Msg { return settingsToggleMsg{key: "timestamps_local"} }
		}},
		// Cycle de vie (left column).
		{"confirm_quit_with_servers", false, func() tea.Cmd {
			return func() tea.Msg { return settingsToggleMsg{key: "confirm_quit_with_servers"} }
		}},
		{"stop_servers_on_exit", false, func() tea.Cmd {
			return func() tea.Msg { return settingsToggleMsg{key: "stop_servers_on_exit"} }
		}},
		{"auto_start_servers", false, func() tea.Cmd {
			return func() tea.Msg { return settingsToggleMsg{key: "auto_start_servers"} }
		}},
	}

	// Drop ANSI escape sequences from the line so substring matches see
	// the same bytes the operator sees on screen — without this a styled
	// "[P]" surrounded by colour escapes wouldn't match the literal "[P]".
	plain := stripANSI(line)
	for _, s := range specs {
		if !strings.Contains(plain, s.label) {
			continue
		}
		// Pane filter: the right pane starts at `half`; anything clicked
		// strictly before that is on the left.
		if s.right && x < half {
			continue
		}
		if !s.right && x >= half {
			continue
		}
		return s.make()
	}

	// Theme markers: the line that contains "thème :" carries [N]/[M]/[O]
	// at different X positions. Locate each in the plain text and hit-test
	// against its rendered span.
	if strings.Contains(plain, "thème :") {
		themes := []struct {
			marker string
			idx    int
		}{{"[N]", 1}, {"[M]", 2}, {"[O]", 3}}
		for _, th := range themes {
			start := strings.Index(plain, th.marker)
			if start < 0 {
				continue
			}
			// Allow a generous 12-cell hit region after the marker so the
			// "[N]  neon" label is clickable as one chip.
			if x >= start && x < start+12 {
				idx := th.idx
				return func() tea.Msg { return settingsSetThemeMsg{idx: idx} }
			}
		}
	}
	return nil
}

// stripANSI removes SGR escape sequences from s so substring searches see
// the plain rendered text. lipgloss only emits the `\x1b[...m` form which
// is what we strip here.
func stripANSI(s string) string {
	out := make([]byte, 0, len(s))
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if c == 'm' {
				inEsc = false
			}
			continue
		}
		out = append(out, c)
	}
	return string(out)
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
		m.boxApparence(colW, r),
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
	var rows []string
	for _, p := range presets {
		marker := Mute.Render(" ")
		labelStyle := Mute
		if r.DefaultArgonPreset == p.v {
			marker = GlowGreen.Render("●")
			labelStyle = GlowGreen
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
		HintKey.Render("[I]") + Dim.Render(" importer un backup ← fichier…"),
	}, "\n")
	return settingsBox(w, "Base de données", body)
}

func (m settingsModel) boxCycleVieServeurs(w int, r *ent.Setting) string {
	sep := max(0, w-6)
	body := GlowCyan.Render("À la fermeture") + "\n" +
		settingsToggle("confirm_quit_with_servers — modal si serveur(s) ON ["+Mute.Render("Q")+"]", r.ConfirmQuitWithServers) + "\n" +
		settingsToggle("stop_servers_on_exit — arrêter les serveurs avant tea.Quit ["+Mute.Render("S")+"]", r.StopServersOnExit) + "\n" +
		Dim.Render(strings.Repeat("─", sep)) + "\n" +
		GlowCyan.Render("Au démarrage") + "\n" +
		settingsToggle("auto_start_servers — démarrer les serveurs au boot ["+Mute.Render("U")+"]", r.AutoStartServers)
	return settingsBox(w, "Cycle de vie serveurs HTTP", body)
}

func (m settingsModel) boxApparence(w int, r *ent.Setting) string {
	// theme keys are N/M/O (not 1/2/3 which set argon preset).
	// Apparence toggles use G/D/T uppercase shortcuts.
	themes := []string{
		themeMarker(string(r.Theme), "neon", "[N]", "neon"),
		themeMarker(string(r.Theme), "mono", "[M]", "mono"),
		themeMarker(string(r.Theme), "nord-soft", "[O]", "nord-soft"),
	}
	body := Dim.Render("thème : ") + strings.Join(themes, "  ") + "\n" +
		settingsToggle("bold_saturated — couleurs vives, glow en TUI ["+Mute.Render("G")+"]", r.BoldSaturated) + "\n" +
		settingsToggle("comfort_density — +1 ligne de padding partout ["+Mute.Render("D")+"]", r.ComfortDensity) + "\n" +
		settingsToggle("timestamps_local — horodatages en local au lieu d'UTC ["+Mute.Render("T")+"]", r.TimestampsLocal)
	return settingsBox(w, "Apparence", body)
}

// themeMarker renders a "● [key] label" entry, highlighting the active theme.
func themeMarker(current, key, hotkey, label string) string {
	if string(current) == key {
		return GlowGreen.Render("●") + " " + HintKey.Render(hotkey) + " " + GlowGreen.Render(label)
	}
	return Mute.Render(" ") + " " + HintKey.Render(hotkey) + " " + Mute.Render(label)
}

// PassphraseSource is set by cmd/license-manager at boot to describe which
// cascade step actually resolved the operator's passphrase. Read by the
// settings screen so it doesn't lie about how the session unlocked.
var PassphraseSource = "TUI prompt"

func (m settingsModel) boxCascadePassphrase(w int, _ *ent.Setting) string {
	steps := strings.Join([]string{
		"  1. " + GlowCyan.Render("--passphrase-file") + " <path>",
		"  2. env " + GlowCyan.Render("MALDEV_MGR_PASSPHRASE_FILE"),
		"  3. env " + GlowCyan.Render("MALDEV_MGR_PASSPHRASE"),
		"  4. fallback prompt TUI interactif",
	}, "\n")
	body := Dim.Render("La passphrase est résolue selon la cascade :") + "\n" +
		steps + "\n\n" +
		Dim.Render("Cette session a résolu via : ") + GlowMagent.Render(PassphraseSource)
	return settingsBox(w, "Cascade passphrase au boot (read-only)", body)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func settingsBox(w int, title, body string) string {
	return BoxStyle.Width(w).Render(GlowCyan.Render(title) + "\n\n" + body)
}

// settingsKV renders a dim-key / fg-value row matching the KV component in settings.jsx.
func settingsKV(key, value string) string { return kvRow(key, value, 22) }

// kvRow / detailColW moved to layout.go (shared list-screen helpers).

// settingsToggle renders a toggle row: [✓]/[ ] label  on/off.
func settingsToggle(label string, on bool) string {
	indicator, state := Mute.Render("[ ]"), Mute.Render("off")
	if on {
		indicator, state = GlowGreen.Render("[✓]"), GlowGreen.Render("on")
	}
	return indicator + " " + Base.Render(label) + "  " + state
}
