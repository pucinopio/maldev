package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
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
	banner := GlowYellow.Render("Settings editing — Phase 4") + "\n" +
		Dim.Render("All fields are read-only until Phase 4 wires the editor form.")

	var body string
	if m.err != nil {
		body = GlowRed.Render("Error: "+m.err.Error()) + "\n" + banner
	} else if m.row == nil {
		body = lipgloss.JoinVertical(lipgloss.Left, banner, "", Dim.Render("  loading…"))
	} else {
		body = lipgloss.JoinVertical(lipgloss.Left, banner, "", m.renderFields())
	}

	hints := []string{"r", "refresh"}
	return lipgloss.JoinVertical(lipgloss.Left, body, renderStatusBar(hints, m.width))
}

func (m settingsModel) renderFields() string {
	r := m.row
	audience := strings.Join(r.DefaultAudience, ", ")
	lines := []string{
		fmt.Sprintf("  %-28s %s", "default_issuer_name", r.DefaultIssuerName),
		fmt.Sprintf("  %-28s %s", "default_audience", audience),
		fmt.Sprintf("  %-28s %d", "default_ttl_seconds", r.DefaultTTLSeconds),
		fmt.Sprintf("  %-28s %s", "default_argon_preset", string(r.DefaultArgonPreset)),
		fmt.Sprintf("  %-28s %s", "operator_name", r.OperatorName),
		fmt.Sprintf("  %-28s %v", "auto_start_servers", r.AutoStartServers),
		fmt.Sprintf("  %-28s %v", "confirm_quit_with_servers", r.ConfirmQuitWithServers),
	}
	return BoxStyle.Width(m.width - 4).Render(strings.Join(lines, "\n"))
}
