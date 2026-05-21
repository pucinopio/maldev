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

// dashBodyColW returns (leftW, rightW) for the dashboard body given terminal
// width w. The body uses a 5:6 flex split (gap=1) matching the widget tree in
// buildWidgetTree. Both serversCardContent and shortcutsCardContent use this
// so the ratio is encoded exactly once.
func dashBodyColW(w int) (left, right int) {
	avail := w - 1 // subtract the 1-char flex gap
	left = avail * 5 / 11
	right = avail - left
	return left, right
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
	// tileW is the minimum per tile; flex distributes remaining space equally.
	tileW := w / 5
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

	// Left column (~5/11 ≈ 45%): issuer key box + servers box.
	// keyTextW = leftColW - 6 (box border+padding overhead) for ACTIVE pill alignment.
	leftColW, _ := dashBodyColW(w)
	keyTextW := leftColW - 6
	if keyTextW < 1 {
		keyTextW = 1
	}
	keyContent := widgets.NewText(m.keyCardContent(keyTextW), lipgloss.NewStyle())
	keyBox := NewBoxWithHint(keyContent, "Clé d'émission active", "[k] gérer", false)
	serversContent := widgets.NewText(m.serversCardContent(), lipgloss.NewStyle())
	serversBox := NewBoxWithHint(serversContent, "Serveurs HTTP", "[7] détail · [s] start/stop", false)
	leftCol := NewFlex(Vertical, 1,
		FlexChild{W: keyBox, Min: 6, Flex: 1},
		FlexChild{W: serversBox, Min: 6, Flex: 1},
	)

	// Right column (~6/11 ≈ 55%): audit log + shortcuts.
	auditContent := widgets.NewText(m.auditCardContent(), lipgloss.NewStyle())
	auditBox := NewBoxWithHint(auditContent, "5 dernières actions", "[8] tout l'audit", false)
	shortcutsContent := widgets.NewText(m.shortcutsCardContent(), lipgloss.NewStyle())
	shortcutsBox := NewBoxWithHint(shortcutsContent, "Raccourcis", "touche → écran", false)
	rightCol := NewFlex(Vertical, 1,
		FlexChild{W: auditBox, Min: 6, Flex: 2},
		FlexChild{W: shortcutsBox, Min: 5, Flex: 1},
	)

	// 5/11 left / 6/11 right split — closer to reference 45/55 ratio.
	body := NewFlex(Horizontal, 1,
		FlexChild{W: leftCol, Flex: 5},
		FlexChild{W: rightCol, Flex: 6},
	)

	root := NewFlex(Vertical, 1,
		FlexChild{W: tilesRow, Min: 5, Max: 7},
		FlexChild{W: body, Flex: 1},
	)
	// Reserve 3 rows for chrome (title + tabs + breadcrumb) + 1 row for status bar.
	root.Layout(Rect{X: 0, Y: 3, W: w, H: h - 4})
	return root
}

func (m dashboardModel) View() string {
	if m.width == 0 {
		return ""
	}
	// Content height = terminal height minus chrome (title+tabs+breadcrumb=3) and status bar (1).
	contentH := m.height - 4
	if contentH < 1 {
		contentH = 1
	}
	if m.loading {
		return lipgloss.Place(m.width, contentH, lipgloss.Center, lipgloss.Center,
			Dim.Render("Loading dashboard…"))
	}
	if m.err != nil {
		return lipgloss.Place(m.width, contentH, lipgloss.Center, lipgloss.Center,
			GlowRed.Render("Error: "+m.err.Error()))
	}
	return m.buildWidgetTree().View()
}

// truncateFingerprint shortens a raw hex fingerprint like
// "4a:2f:88:d1:09:cc:fe:b3:72:1e:aa:5d:03:89:c0:f7"
// to "ed25519:4a2f…c0f7" (algo prefix + first 4 hex chars + ellipsis + last 4).
// When the input already looks like "algo:…" it is returned unchanged.
// The function never returns a string longer than ~24 chars, preventing wraps.
func truncateFingerprint(fpr string) string {
	if fpr == "" {
		return ""
	}
	// Already in algo:digest form — trust it.
	if strings.Contains(fpr, ":") {
		// Strip colons to get raw hex bytes, then format.
		raw := strings.ReplaceAll(fpr, ":", "")
		if len(raw) < 8 {
			return fpr
		}
		return "ed25519:" + raw[:4] + "…" + raw[len(raw)-4:]
	}
	if len(fpr) < 8 {
		return fpr
	}
	return "ed25519:" + fpr[:4] + "…" + fpr[len(fpr)-4:]
}

// keyCardContent builds the text content for the issuer key card.
// The KeyID is shown large; name and fingerprint follow on subsequent lines.
// An ACTIVE pill appears on the right of the first data line when a key is set.
// keyActivePill is the inline ACTIVE badge — flat (no border) so it stays
// on one line. The bordered PillActive is 3 lines tall and cannot be used inline.
var keyActivePill = GlowGreen

// keyCardContent builds the issuer key card text for the given inner text width.
// The ACTIVE pill is right-aligned on the KeyID line so it mirrors the reference.
func (m dashboardModel) keyCardContent(textW int) string {
	if m.activeKey.id == "" {
		return Mute.Render("aucune clé active")
	}
	pill := keyActivePill.Render("ACTIVE")
	keyID := GlowCyan.Render(m.activeKey.id)
	keyIDW := lipgloss.Width(keyID)
	pillW := lipgloss.Width(pill)
	gap := textW - keyIDW - pillW
	if gap < 1 {
		gap = 1
	}
	idLine := keyID + strings.Repeat(" ", gap) + pill
	fpr := truncateFingerprint(m.activeKey.fingerprint)
	return lipgloss.JoinVertical(lipgloss.Left,
		idLine,
		Base.Render("nom ")+Dim.Render(m.activeKey.name),
		Base.Render("fpr ")+Mute.Render(fpr),
	)
}

// serverPillStyle returns a single-line colored tag for running/stopped state.
// Unlike the bordered PillOn/PillOff styles used in tables, the inline variant
// is flat (no border) so it can be embedded mid-string without injecting \n.
var (
	serverPillOn  = GlowGreen // reuses theme constant — green bold flat tag for ON state
	serverPillOff = lipgloss.NewStyle().Foreground(Palette.FgMute).Bold(true)
)

// serverRow builds two lines for one server entry matching the reference layout:
//
//	● name  :port                                                      ON
//	  https://host:port · N req · up Xh Ym
//
// The ON/OFF tag is right-aligned to colW on the first line.
func serverRow(name, addr, url string, on bool, reqs uint64, uptime string, colW int) string {
	bullet := Mute.Render("●")
	if !on {
		bullet = Mute.Render("○")
	}

	var tag string
	if on {
		tag = serverPillOn.Render("ON")
	} else {
		tag = serverPillOff.Render("OFF")
	}

	nameAddr := GlowCyan.Render(name) + "  " + Dim.Render(addr)
	prefixW := lipgloss.Width(bullet) + 1 + lipgloss.Width(nameAddr)
	tagW := lipgloss.Width(tag)
	gap := colW - prefixW - tagW
	if gap < 1 {
		gap = 1
	}
	line1 := bullet + " " + nameAddr + strings.Repeat(" ", gap) + tag

	// Line 2: url · req count · uptime (or stopped hint).
	var detail string
	if on && url != "" && url != "—" {
		detail = Mute.Render(url)
		if reqs > 0 {
			detail += Mute.Render(fmt.Sprintf(" · %d req", reqs))
		}
		if uptime != "" {
			detail += Mute.Render(" · up " + uptime)
		}
	} else if !on {
		detail = Mute.Render("arrêté · démarrer via onglet [7]")
	}

	return line1 + "\n  " + detail
}

// serversCardContent builds the text content for the servers card.
// Each server is rendered as two compact lines with the ON/OFF tag right-aligned.
// When the bundle is wired (Phase 4+) it reads live Status snapshots directly.
func (m dashboardModel) serversCardContent() string {
	w := m.width
	if w == 0 {
		w = 120
	}
	// Box chrome: border(2) + padding(0,1) left+right(2) = 4; lipgloss Width()
	// absorbs the padding inside its argument, so text area = leftColW - 6.
	leftColW, _ := dashBodyColW(w)
	colW := leftColW - 6
	if colW < 20 {
		colW = 20
	}

	// sep is reused by both bundle-nil and live paths.
	sep := Mute.Render(strings.Repeat("╌", colW))

	buildRows := func(name, addr, url string, on bool, reqs uint64, uptime string) string {
		return serverRow(name, addr, url, on, reqs, uptime, colW)
	}

	if m.bundle == nil {
		var rows []string
		for _, s := range m.servers {
			// s.URL holds the listen address (":8443"). Build a full URL for display.
			url := ""
			if s.URL != "" {
				url = "https://manager.local" + s.URL
			}
			rows = append(rows, buildRows(s.Name, s.URL, url, s.On, s.Requests, s.Uptime))
		}
		if len(rows) == 0 {
			return Mute.Render("no servers configured")
		}
		return strings.Join(rows, "\n"+sep+"\n")
	}

	// Live path: read directly from Bundle.Statuses().
	statuses := m.bundle.Statuses()
	names := []string{"revocation", "heartbeat", "probe"}
	var rows []string
	for _, name := range names {
		s, ok := statuses[name]
		addr, url := "—", ""
		var reqs uint64
		if ok && s.ListenAddr != "" {
			addr = s.ListenAddr
			url = "https://manager.local" + s.ListenAddr
		}
		rows = append(rows, buildRows(name, addr, url, ok && s.Running, reqs, ""))
	}
	return strings.Join(rows, "\n"+sep+"\n")
}

// auditCardContent builds the text content for the recent events card.
// Format: HH:MM:SS kind target (actor) — note
func (m dashboardModel) auditCardContent() string {
	if len(m.recent) == 0 {
		return Mute.Render("aucun événement")
	}
	var lines []string
	for _, e := range m.recent {
		ts := e.At.Format("15:04:05")
		line := Mute.Render(ts) +
			" " + GlowMagent.Render(e.Kind) +
			" " + Dim.Render(e.TargetID) +
			" " + Mute.Render("("+e.Actor+")")
		if e.Note != "" {
			line += Mute.Render(" — " + e.Note)
		}
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

	// Right column width from the shared 5:6 split helper.
	// Box text area = rightColW - 6; 3 cells share that minus 2 separator chars.
	w := m.width
	if w == 0 {
		w = 120
	}
	_, rightColW := dashBodyColW(w)
	textW := rightColW - 6
	if textW < 1 {
		textW = 1
	}
	cellW := (textW - 2) / 3
	if cellW < 18 {
		cellW = 18
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
