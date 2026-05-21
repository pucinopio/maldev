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
	case tea.KeyMsg:
		if msg.String() == "r" {
			return m, loadSettingsCmd(m.svc)
		}
	}
	return m, nil
}

func (m settingsModel) View() string {
	w := m.width
	if w == 0 {
		w = 120 // safe default when no WindowSizeMsg yet
	}
	hints := []string{"r", "refresh", "P", "changer passphrase", "V", "vacuum", "B", "backup"}
	if m.err != nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			GlowRed.Render("Error: "+m.err.Error()),
			renderStatusBar(hints, w))
	}
	if m.row == nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			Dim.Render("  loading…"),
			renderStatusBar(hints, w))
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.renderGrid(w), renderStatusBar(hints, w))
}

// renderGrid builds the 2-column grid matching settings.jsx layout.
func (m settingsModel) renderGrid(w int) string {
	r := m.row
	colW := w/2 - 1
	if colW < 20 {
		colW = 20
	}

	left := lipgloss.JoinVertical(lipgloss.Left,
		m.boxDefaultsLicence(colW, r),
		m.boxIdentiteOperateur(colW, r),
		m.boxCycleVieServeurs(colW, r),
		m.boxCascadePassphrase(colW, r),
	)
	right := lipgloss.JoinVertical(lipgloss.Left,
		m.boxArgonPreset(colW, r),
		m.boxBaseDeDonnees(colW, r),
		m.boxApparence(colW),
	)

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
	var chips []string
	for _, p := range presets {
		label := HintKey.Render("["+p.key+"]") + " " + p.label
		if r.DefaultArgonPreset == p.v {
			chips = append(chips, PillActive.Render(label))
		} else {
			chips = append(chips, PillOff.Render(label))
		}
	}
	body := strings.Join(chips, "\n") + "\n" +
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
	themes := []string{
		PillActive.Render(HintKey.Render("[1]") + " neon"),
		PillOff.Render(HintKey.Render("[2]") + " mono"),
		PillOff.Render(HintKey.Render("[3]") + " nord-soft"),
	}
	body := Dim.Render("thème : ") + strings.Join(themes, " ") + "\n" +
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

// settingsBox wraps body in a titled bordered box, matching settings.jsx Box usage.
func settingsBox(w int, title, body string) string {
	return BoxStyle.Width(w).Render(GlowCyan.Render(title) + "\n\n" + body)
}

// settingsKV renders a dim-key / fg-value row matching the KV component in settings.jsx.
func settingsKV(key, value string) string {
	return Dim.Render(fmt.Sprintf("%-22s", key)) + Base.Render(value)
}

// settingsToggle renders a toggle row: [✓]/[ ] label  on/off.
func settingsToggle(label string, on bool) string {
	indicator, state := Mute.Render("[ ]"), Mute.Render("off")
	if on {
		indicator, state = GlowGreen.Render("[✓]"), GlowGreen.Render("on")
	}
	return indicator + " " + Base.Render(label) + "  " + state
}
