package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
	"github.com/oioio-space/maldev/internal/manager/tui/widgets"
)

// SwitchToLicensesMsg is fired when a dashboard tile is clicked.
// Filter is one of "active", "expiring", "expired", "revoked".
type SwitchToLicensesMsg struct{ Filter string }

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

// buildWidgetTree constructs the full dashboard widget tree and assigns layout.
// Called on View() and on mouse dispatch so it always reflects current data.
// The tree is intentionally stateless — cheap for small trees.
func (m dashboardModel) buildWidgetTree() Widget {
	w, h := m.width, m.height
	if w == 0 {
		w = 120
	}
	if h == 0 {
		h = 40
	}

	// Counter tiles row.
	tileW := w/4 - 2
	activeTile := widgets.NewTile("Active", m.counters.active, "", Palette.Green,
		func() tea.Cmd { return func() tea.Msg { return SwitchToLicensesMsg{Filter: "active"} } })
	expiringTile := widgets.NewTile("Expiring Soon", m.counters.expiringSoon, "", Palette.Yellow,
		func() tea.Cmd { return func() tea.Msg { return SwitchToLicensesMsg{Filter: "expiring"} } })
	expiredTile := widgets.NewTile("Expired", m.counters.expired, "", Palette.Red,
		func() tea.Cmd { return func() tea.Msg { return SwitchToLicensesMsg{Filter: "expired"} } })
	revokedTile := widgets.NewTile("Revoked", m.counters.revoked, "", Palette.Red,
		func() tea.Cmd { return func() tea.Msg { return SwitchToLicensesMsg{Filter: "revoked"} } })

	tilesRow := NewFlex(Horizontal, 0,
		FlexChild{W: activeTile, Min: tileW, Flex: 1},
		FlexChild{W: expiringTile, Min: tileW, Flex: 1},
		FlexChild{W: expiredTile, Min: tileW, Flex: 1},
		FlexChild{W: revokedTile, Min: tileW, Flex: 1},
	)

	// Left column content.
	keyContent := widgets.NewText(m.keyCardContent(), lipgloss.NewStyle())
	keyBox := NewBox(keyContent, "Active Issuer Key", false)
	serversContent := widgets.NewText(m.serversCardContent(), lipgloss.NewStyle())
	serversBox := NewBox(serversContent, "Servers", false)
	leftCol := NewFlex(Vertical, 1,
		FlexChild{W: keyBox, Min: 6, Flex: 1},
		FlexChild{W: serversBox, Min: 6, Flex: 1},
	)

	// Right column content.
	auditContent := widgets.NewText(m.auditCardContent(), lipgloss.NewStyle())
	auditBox := NewBox(auditContent, "Recent Events", false)
	shortcutsContent := widgets.NewText(m.shortcutsCardContent(), lipgloss.NewStyle())
	shortcutsBox := NewBox(shortcutsContent, "Shortcuts", false)
	rightCol := NewFlex(Vertical, 1,
		FlexChild{W: auditBox, Min: 6, Flex: 1},
		FlexChild{W: shortcutsBox, Min: 6, Flex: 1},
	)

	body := NewFlex(Horizontal, 2,
		FlexChild{W: leftCol, Flex: 1},
		FlexChild{W: rightCol, Flex: 1},
	)

	root := NewFlex(Vertical, 1,
		FlexChild{W: tilesRow, Min: 5, Max: 7},
		FlexChild{W: body, Flex: 1},
	)
	// Reserve 2 rows for chrome (title + tabs).
	root.Layout(Rect{X: 0, Y: 2, W: w, H: h - 2})
	return root
}

func (m dashboardModel) View() string {
	if m.width == 0 {
		return ""
	}
	if m.loading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			Dim.Render("Loading dashboard…"))
	}
	if m.err != nil {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			GlowRed.Render("Error: "+m.err.Error()))
	}
	return m.buildWidgetTree().View()
}

// keyCardContent builds the text content for the key card.
func (m dashboardModel) keyCardContent() string {
	if m.activeKey.id == "" {
		return Mute.Render("no active key")
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		Base.Render("Name:  ")+Dim.Render(m.activeKey.name),
		Base.Render("KeyID: ")+Dim.Render(m.activeKey.id),
		Base.Render("FP:    ")+Mute.Render(m.activeKey.fingerprint),
	)
}

// serversCardContent builds the text content for the servers card.
// When the bundle is wired (Phase 4+) it reads live Status snapshots directly
// so the dashboard always reflects the current running state.
func (m dashboardModel) serversCardContent() string {
	if m.bundle == nil {
		// Bundle not wired — fall back to the snapshot injected by DashboardSnapshotMsg.
		var lines []string
		for _, s := range m.servers {
			var pill string
			if s.On {
				pill = PillOn.Render("ON")
			} else {
				pill = PillOff.Render("OFF")
			}
			lines = append(lines, fmt.Sprintf("%-12s %s  %s", s.Name, pill, Mute.Render(s.URL)))
		}
		if len(lines) == 0 {
			return Mute.Render("no servers configured")
		}
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	// Live path: read directly from Bundle.Statuses().
	statuses := m.bundle.Statuses()
	names := []string{"revocation", "heartbeat", "probe"}
	lines := make([]string, 0, len(names))
	for _, name := range names {
		s, ok := statuses[name]
		var pill string
		if ok && s.Running {
			pill = PillOn.Render("ON")
		} else {
			pill = PillOff.Render("OFF")
		}
		addr := "—"
		if ok && s.ListenAddr != "" {
			addr = s.ListenAddr
		}
		lines = append(lines, fmt.Sprintf("%-12s %s  %s", name, pill, Mute.Render(addr)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// auditCardContent builds the text content for the recent events card.
func (m dashboardModel) auditCardContent() string {
	if len(m.recent) == 0 {
		return Mute.Render("no events yet")
	}
	var lines []string
	for _, e := range m.recent {
		ts := e.At.Format(time.RFC3339)[:19]
		lines = append(lines, fmt.Sprintf("%-19s  %-20s  %s",
			Mute.Render(ts), GlowMagent.Render(e.Kind), Dim.Render(e.Actor)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// shortcutsCardContent builds the text content for the shortcuts card.
func (m dashboardModel) shortcutsCardContent() string {
	hints := [][2]string{
		{"1-9", "switch view"}, {"q", "quit"}, {"?", "help"}, {"r", "refresh"},
	}
	var lines []string
	for _, h := range hints {
		lines = append(lines, HintKey.Render(h[0])+" "+HintText.Render(h[1]))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
