package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
)

// dashboardModel holds the data and layout state for the Dashboard view.
type dashboardModel struct {
	services *service.Services
	bundle   *httpsrv.Bundle

	counters struct {
		active, revoked, expired, expiringSoon int
	}
	activeKey struct {
		id, name, fingerprint string
	}
	servers [3]cmds.ServerStatus
	recent  []cmds.AuditEntry

	width, height int
	err           error
	loading       bool
}

func newDashboardModel(s *service.Services, b *httpsrv.Bundle) dashboardModel {
	return dashboardModel{
		services: s,
		bundle:   b,
		loading:  true,
		servers: [3]cmds.ServerStatus{
			{Name: "Revocation"},
			{Name: "Heartbeat"},
			{Name: "Probe"},
		},
	}
}

// refresh returns a Cmd that fetches a fresh DashboardSnapshot.
func (m dashboardModel) refresh() tea.Cmd {
	return cmds.DashboardSnapshotCmd(m.services)
}

func (m dashboardModel) Update(msg tea.Msg) (dashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case cmds.DashboardSnapshotMsg:
		m.loading = false
		m.err = msg.Err
		if msg.Err == nil {
			m.counters.active = msg.Active
			m.counters.revoked = msg.Revoked
			m.counters.expired = msg.Expired
			m.counters.expiringSoon = msg.ExpiringSoon
			m.activeKey.id = msg.ActiveKeyID
			m.activeKey.name = msg.ActiveKeyName
			m.activeKey.fingerprint = msg.ActiveKeyFingerprint
			for i, s := range msg.Servers {
				if i < 3 {
					m.servers[i] = s
				}
			}
			m.recent = msg.RecentAudit
		}
	}
	return m, nil
}

func (m dashboardModel) View() string {
	w := m.width
	h := m.height
	if w == 0 {
		w = 120
	}
	if h == 0 {
		h = 40
	}

	if m.loading {
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center,
			Dim.Render("Loading dashboard…"))
	}
	if m.err != nil {
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center,
			GlowRed.Render("Error: "+m.err.Error()))
	}

	// ── Counter tiles ────────────────────────────────────────────────
	tiles := m.renderCounterTiles(w)

	// ── Left column: active key + servers ───────────────────────────
	leftW := w / 2
	keyCard := m.renderKeyCard(leftW - 2)
	serversCard := m.renderServersCard(leftW - 2)
	leftCol := lipgloss.JoinVertical(lipgloss.Left, keyCard, "", serversCard)

	// ── Right column: recent audit + shortcuts ───────────────────────
	rightW := w - leftW - 2
	auditCard := m.renderAuditCard(rightW - 2)
	shortcutsCard := m.renderShortcutsCard(rightW - 2)
	rightCol := lipgloss.JoinVertical(lipgloss.Left, auditCard, "", shortcutsCard)

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(leftW).Render(leftCol),
		"  ",
		lipgloss.NewStyle().Width(rightW).Render(rightCol),
	)

	return lipgloss.JoinVertical(lipgloss.Left, tiles, "", body)
}

func (m dashboardModel) renderCounterTiles(w int) string {
	tileW := w/4 - 2

	tile := func(label string, val int, style lipgloss.Style) string {
		inner := lipgloss.JoinVertical(lipgloss.Center,
			style.Render(fmt.Sprintf("%d", val)),
			Dim.Render(label),
		)
		return BoxStyle.Width(tileW).Align(lipgloss.Center).Render(inner)
	}

	t1 := tile("Active", m.counters.active, GlowGreen)
	t2 := tile("Expiring Soon", m.counters.expiringSoon, GlowYellow)
	t3 := tile("Expired", m.counters.expired, GlowRed)
	t4 := tile("Revoked", m.counters.revoked, GlowRed)

	return lipgloss.JoinHorizontal(lipgloss.Top, t1, t2, t3, t4)
}

func (m dashboardModel) renderKeyCard(w int) string {
	var lines []string
	lines = append(lines, GlowCyan.Render("Active Issuer Key"))
	lines = append(lines, "")
	if m.activeKey.id == "" {
		lines = append(lines, Mute.Render("no active key"))
	} else {
		lines = append(lines, Base.Render("Name:  ")+Dim.Render(m.activeKey.name))
		lines = append(lines, Base.Render("KeyID: ")+Dim.Render(m.activeKey.id))
		lines = append(lines, Base.Render("FP:    ")+Mute.Render(m.activeKey.fingerprint))
	}
	return BoxStyle.Width(w).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m dashboardModel) renderServersCard(w int) string {
	var lines []string
	lines = append(lines, GlowCyan.Render("Servers"))
	lines = append(lines, "")

	if m.bundle == nil {
		lines = append(lines, Mute.Render("Phase 4 — to be wired"))
	} else {
		for _, s := range m.servers {
			var pill string
			if s.On {
				pill = PillOn.Render("ON")
			} else {
				pill = PillOff.Render("OFF")
			}
			line := fmt.Sprintf("%-12s %s  %s", s.Name, pill, Mute.Render(s.URL))
			lines = append(lines, Base.Render(line))
		}
	}
	return BoxStyle.Width(w).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m dashboardModel) renderAuditCard(w int) string {
	var lines []string
	lines = append(lines, GlowCyan.Render("Recent Events"))
	lines = append(lines, "")

	if len(m.recent) == 0 {
		lines = append(lines, Mute.Render("no events yet"))
	} else {
		for _, e := range m.recent {
			ts := e.At.Format(time.RFC3339)[:19]
			row := fmt.Sprintf("%-19s  %-20s  %s",
				Mute.Render(ts),
				GlowMagent.Render(e.Kind),
				Dim.Render(e.Actor),
			)
			lines = append(lines, row)
		}
	}
	return BoxStyle.Width(w).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m dashboardModel) renderShortcutsCard(w int) string {
	hints := [][2]string{
		{"1-9", "switch view"},
		{"q", "quit"},
		{"?", "help"},
		{"r", "refresh"},
	}
	var lines []string
	lines = append(lines, GlowCyan.Render("Shortcuts"))
	lines = append(lines, "")
	for _, h := range hints {
		lines = append(lines, HintKey.Render(h[0])+" "+HintText.Render(h[1]))
	}
	return BoxStyle.Width(w).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
