package tui

import (
	"fmt"
	"strings"

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
		active, revoked, expired, expiringSoon, superseded int
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
			m.counters.superseded = msg.Superseded
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

	// Five counter tiles with reference subtitles.
	tileW := w/5 - 1
	activeTile := widgets.NewTile("Actives [a]", m.counters.active,
		"signées par la clé active", Palette.Green,
		func() tea.Cmd { return func() tea.Msg { return SwitchToLicensesMsg{Filter: "active"} } })
	revokedTile := widgets.NewTile("Révoquées [r]", m.counters.revoked,
		"présentes dans la CRL", Palette.Red,
		func() tea.Cmd { return func() tea.Msg { return SwitchToLicensesMsg{Filter: "revoked"} } })
	expiredTile := widgets.NewTile("Expirées [e]", m.counters.expired,
		"NotAfter dépassé", Palette.Orange,
		func() tea.Cmd { return func() tea.Msg { return SwitchToLicensesMsg{Filter: "expired"} } })
	expiringTile := widgets.NewTile("Expirent < 7 j [w]", m.counters.expiringSoon,
		"à renouveler", Palette.Yellow,
		func() tea.Cmd { return func() tea.Msg { return SwitchToLicensesMsg{Filter: "expiring"} } })
	supersededTile := widgets.NewTile("Superseded [u]", m.counters.superseded,
		"re-émises plus tard", Palette.Cyan,
		func() tea.Cmd { return func() tea.Msg { return SwitchToLicensesMsg{Filter: "active"} } })

	tilesRow := NewFlex(Horizontal, 0,
		FlexChild{W: activeTile, Min: tileW, Flex: 1},
		FlexChild{W: revokedTile, Min: tileW, Flex: 1},
		FlexChild{W: expiredTile, Min: tileW, Flex: 1},
		FlexChild{W: expiringTile, Min: tileW, Flex: 1},
		FlexChild{W: supersededTile, Min: tileW, Flex: 1},
	)

	// Left column (~⅓): issuer key box + servers box.
	keyContent := widgets.NewText(m.keyCardContent(), lipgloss.NewStyle())
	keyBox := NewBox(keyContent, "Clé d'émission active [k] gérer", false)
	serversContent := widgets.NewText(m.serversCardContent(), lipgloss.NewStyle())
	serversBox := NewBox(serversContent, "Serveurs HTTP [7] détail · [s] start/stop", false)
	leftCol := NewFlex(Vertical, 1,
		FlexChild{W: keyBox, Min: 6, Flex: 1},
		FlexChild{W: serversBox, Min: 6, Flex: 1},
	)

	// Right column (~⅔): audit log + shortcuts.
	auditContent := widgets.NewText(m.auditCardContent(), lipgloss.NewStyle())
	auditBox := NewBox(auditContent, "5 dernières actions [8] tout l'audit", false)
	shortcutsContent := widgets.NewText(m.shortcutsCardContent(), lipgloss.NewStyle())
	shortcutsBox := NewBox(shortcutsContent, "Raccourcis touche → écran", false)
	rightCol := NewFlex(Vertical, 1,
		FlexChild{W: auditBox, Min: 6, Flex: 2},
		FlexChild{W: shortcutsBox, Min: 5, Flex: 1},
	)

	// ⅓ left / ⅔ right split.
	body := NewFlex(Horizontal, 1,
		FlexChild{W: leftCol, Flex: 1},
		FlexChild{W: rightCol, Flex: 2},
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

// keyCardContent builds the text content for the issuer key card.
// The KeyID is shown large; name and fingerprint follow on subsequent lines.
// An ACTIVE pill appears on the right of the first data line when a key is set.
func (m dashboardModel) keyCardContent() string {
	if m.activeKey.id == "" {
		return Mute.Render("aucune clé active")
	}
	pill := PillActive.Render("ACTIVE")
	idLine := GlowCyan.Render(m.activeKey.id) + "  " + pill
	return lipgloss.JoinVertical(lipgloss.Left,
		idLine,
		Base.Render("nom ")+Dim.Render(m.activeKey.name),
		Base.Render("fpr ")+Mute.Render(m.activeKey.fingerprint),
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
// Format: HH:MM:SS  kind  target  (actor)
func (m dashboardModel) auditCardContent() string {
	if len(m.recent) == 0 {
		return Mute.Render("aucun événement")
	}
	var lines []string
	for _, e := range m.recent {
		ts := e.At.Format("15:04:05")
		line := fmt.Sprintf("%s  %-22s  %-16s  %s",
			Mute.Render(ts),
			GlowMagent.Render(e.Kind),
			Dim.Render(e.TargetID),
			Mute.Render("("+e.Actor+")"),
		)
		lines = append(lines, line)
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// shortcutsCardContent builds the 6-hint shortcuts grid matching dashboard.jsx:
// 3 columns × 2 rows, dashed borders between cells (visual separator only).
func (m dashboardModel) shortcutsCardContent() string {
	type shortcut struct{ key, label string }
	hints := []shortcut{
		{"n", "nouvelle licence"},
		{"/", "rechercher"},
		{"x", "révoquer"},
		{"k", "clés d'émission"},
		{"i", "identity.bin"},
		{"?", "aide contextuelle"},
	}

	// The shortcuts box lives in the right column (~⅔ of total width).
	// Subtract box chrome (border 2 + padding 2 = 4) then divide by 3 cols.
	// Use a generous minimum so labels are never truncated.
	rightColW := m.width * 2 / 3
	cellW := (rightColW - 4) / 3
	if cellW < 20 {
		cellW = 20
	}

	sep := Mute.Render("│")
	divPart := strings.Repeat("─", cellW)
	divider := Mute.Render(divPart + "┼" + divPart + "┼" + divPart)

	var rows [2]string
	for row := 0; row < 2; row++ {
		var cells [3]string
		for col := 0; col < 3; col++ {
			h := hints[row*3+col]
			cell := HintKey.Render("["+h.key+"]") + " " + HintText.Render(h.label)
			cells[col] = lipgloss.NewStyle().Width(cellW).Render(cell)
		}
		rows[row] = cells[0] + sep + cells[1] + sep + cells[2]
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows[0], divider, rows[1])
}
