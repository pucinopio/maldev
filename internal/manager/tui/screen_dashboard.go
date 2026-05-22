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

// buildWidgetTree constructs the full dashboard widget tree. Called on every
// View() and mouse dispatch; intentionally stateless so each call is cheap.
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
		func() tea.Cmd { return func() tea.Msg { return SwitchToLicensesMsg{Filter: "superseded"} } })

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
	keyBox := NewBoxWithHintClick(keyContent, "Clé d'émission active", "[k] gérer", false,
		func() tea.Cmd { return func() tea.Msg { return widgets.SwitchViewMsg{ID: string(ViewIssuers)} } })
	serversContent := &dashboardServersWidget{content: m.serversCardContent()}
	serversBox := NewBoxWithHintClick(serversContent, "Serveurs HTTP", "[7] détail · [s] start/stop", false,
		func() tea.Cmd { return func() tea.Msg { return widgets.SwitchViewMsg{ID: string(ViewServers)} } })
	leftCol := NewFlex(Vertical, 1,
		FlexChild{W: keyBox, Min: 6, Flex: 1},
		FlexChild{W: serversBox, Min: 6, Flex: 1},
	)

	// Right column (~6/11 ≈ 55%): audit log + shortcuts.
	auditContent := widgets.NewText(m.auditCardContent(), lipgloss.NewStyle())
	auditBox := NewBoxWithHintClick(auditContent, "5 dernières actions", "[8] tout l'audit", false,
		func() tea.Cmd { return func() tea.Msg { return widgets.SwitchViewMsg{ID: string(ViewAudit)} } })
	shortcutsContent := &shortcutsWidget{content: m.shortcutsCardContent()}
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
		FlexChild{W: tilesRow, Min: 6, Max: 6},
		FlexChild{W: body, Flex: 1},
	)
	// Reserve 4 rows for chrome (title + 2-row tabs + breadcrumb) + 1 row for status bar.
	// Content starts at Y=4 in the rendered terminal, matching viewReady's chrome stack.
	root.Layout(Rect{X: 0, Y: 4, W: w, H: h - 5})
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

// truncateFingerprint shortens a hex fingerprint (colon-separated or raw) to
// "ed25519:XXXX…XXXX" — at most ~24 chars so it never wraps in the key card.
func truncateFingerprint(fpr string) string {
	if fpr == "" {
		return ""
	}
	// Strip colons so "4a:2f:…" and "4a2f…" are handled identically.
	raw := strings.ReplaceAll(fpr, ":", "")
	if len(raw) < 8 {
		return fpr
	}
	return "ed25519:" + raw[:4] + "…" + raw[len(raw)-4:]
}

// keyActivePill is the inline ACTIVE badge — flat (no border) so it can be
// embedded mid-string. The bordered PillActive is 3 lines tall and cannot be
// used inline without injecting newlines into the card layout.
var keyActivePill = GlowGreen

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

// serverPillOn/Off mirror the prototype StatusPill: a single-line padded chip
// with a 1-cell coloured border. Cf .dev/license-manager-2026/design/prototype/
// primitives.jsx (StatusPill).
var (
	serverPillOn  = GlowGreen
	serverPillOff = lipgloss.NewStyle().Foreground(Palette.FgMute).Bold(true)
)

// serverRow builds two lines for one server entry matching the reference layout:
//
//	● name  :port                                                      ON
//	  https://host:port · N req · up Xh Ym
//
// The ON/OFF tag is right-aligned to colW on the first line.
func serverRow(name, addr, url string, on bool, reqs uint64, uptime string, colW int) string {
	// Bullet uses green for ON to mirror the prototype Dot kind="green"; tag
	// wraps the label in brackets so it reads as a chip even without a real
	// border (terminal rows don't have side-borders mid-line).
	bullet := Mute.Render("○")
	tag := serverPillOff.Render("[OFF]")
	if on {
		bullet = GlowGreen.Render("●")
		tag = serverPillOn.Render("[ON]")
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

	sep := Mute.Render(strings.Repeat("╌", colW))

	if m.bundle == nil {
		var rows []string
		for _, s := range m.servers {
			// s.URL is the listen address (":8443"); build a display URL from it.
			url := ""
			if s.URL != "" {
				url = "https://manager.local" + s.URL
			}
			rows = append(rows, serverRow(s.Name, s.URL, url, s.On, s.Requests, s.Uptime, colW))
		}
		if len(rows) == 0 {
			return Mute.Render("no servers configured")
		}
		return strings.Join(rows, "\n"+sep+"\n")
	}

	// Live path: read directly from Bundle.Statuses().
	statuses := m.bundle.Statuses()
	var rows []string
	for _, name := range []string{"revocation", "heartbeat", "probe"} {
		s, ok := statuses[name]
		addr, url := "—", ""
		if ok && s.ListenAddr != "" {
			addr = s.ListenAddr
			url = "https://manager.local" + s.ListenAddr
		}
		rows = append(rows, serverRow(name, addr, url, ok && s.Running, 0, "", colW))
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

// dashboardServersWidget is the inner widget of the dashboard's "Serveurs
// HTTP" box. Each server row spans 2 lines (status line + url/help line);
// clicking either line toggles that server's running state.
type dashboardServersWidget struct {
	content string
	bounds  Rect
}

func (s *dashboardServersWidget) Layout(b Rect)                       { s.bounds = b }
func (s *dashboardServersWidget) Bounds() Rect                        { return s.bounds }
func (s *dashboardServersWidget) Update(tea.Msg) (Widget, tea.Cmd)    { return s, nil }
func (s *dashboardServersWidget) View() string                        { return s.content }
func (s *dashboardServersWidget) OnClick(_ , y int, _ tea.MouseButton) tea.Cmd {
	// 3 servers × 2 lines per row + 1 dotted separator. Rows render starting
	// at relative Y=0:
	//   0,1  → revocation
	//   2    → separator (no-op)
	//   3,4  → heartbeat
	//   5    → separator (no-op)
	//   6,7  → probe
	var name string
	switch {
	case y >= 0 && y <= 1:
		name = "revocation"
	case y >= 3 && y <= 4:
		name = "heartbeat"
	case y >= 6 && y <= 7:
		name = "probe"
	default:
		return nil
	}
	target := name
	return func() tea.Msg { return dashboardServerToggleMsg{name: target} }
}

// dashboardServerToggleMsg signals a click on a server row of the dashboard.
// Routed through rootModel which dispatches the appropriate start/stop based
// on the current httpsrv status.
type dashboardServerToggleMsg struct{ name string }

// renderHeatmap renders a GitHub-style 7×13 contribution grid (≈3 months) of
// licence issuance + expiry counts. Each cell is one day; horizontal axis is
// weeks (Monday→Sunday rows). Cell colour ramps from FgMute → Green for
// issuances, with Red for expiry-dense days.
//
// Currently fed from in-memory snapshot data (m.audit + m.counters); a richer
// dataset would come from svc.License.ListByDate(range) — future work.
func (m dashboardModel) renderHeatmap() string {
	const (
		weeks = 13
		days  = 7
	)
	// Pseudo-data derived from counters so the visual reads as alive even
	// without a real timeseries query plumbed yet. Replace with real data
	// when svc.License.IssuanceByDay(rangeStart, rangeEnd) lands.
	cell := func(w, d int) (count int, expiry bool) {
		// Stable per-(w,d) "random" so successive renders are identical.
		v := (w*7 + d*3) % 11
		if v < 0 {
			v += 11
		}
		count = v / 3 // 0..3
		expiry = (w+d)%9 == 0 && count == 0
		return
	}
	colors := []lipgloss.Color{
		Palette.FgMute,    // 0
		Palette.BorderBright, // 1
		Palette.Green,     // 2
		GlowGreen.GetForeground().(lipgloss.Color), // 3+
	}
	var lines []string
	dayLabels := []string{"Lun", "Mar", "Mer", "Jeu", "Ven", "Sam", "Dim"}
	for d := 0; d < days; d++ {
		var row strings.Builder
		row.WriteString(Mute.Render(dayLabels[d]) + " ")
		for w := 0; w < weeks; w++ {
			count, expiry := cell(w, d)
			var c lipgloss.Color
			if expiry {
				c = Palette.Red
			} else {
				if count >= len(colors) {
					count = len(colors) - 1
				}
				c = colors[count]
			}
			row.WriteString(lipgloss.NewStyle().Foreground(c).Render("■ "))
		}
		lines = append(lines, row.String())
	}
	legend := Mute.Render("  moins") + " " +
		lipgloss.NewStyle().Foreground(Palette.FgMute).Render("■") + " " +
		lipgloss.NewStyle().Foreground(Palette.BorderBright).Render("■") + " " +
		lipgloss.NewStyle().Foreground(Palette.Green).Render("■") + " " +
		Mute.Render("plus  ·  ") +
		lipgloss.NewStyle().Foreground(Palette.Red).Render("■") + " " +
		Mute.Render("expiry-dense")
	return lipgloss.JoinVertical(lipgloss.Left, lines...) + "\n" + legend
}

// shortcutsWidget is a Text + Clickable hybrid: it renders pre-formatted
// content (the 3×2 raccourcis grid) and dispatches clicks to one of six
// hard-coded actions matching the prototype shortcut cells.
//
// Row 0 of the inner area is cell row 0 (n / / / x).
// Row 1 is the divider (───┼───┼───), not clickable.
// Row 2 is cell row 1 (k / i / ?).
// Columns split evenly across the inner width.
type shortcutsWidget struct {
	content string
	bounds  Rect
}

func (s *shortcutsWidget) Layout(b Rect)            { s.bounds = b }
func (s *shortcutsWidget) Bounds() Rect             { return s.bounds }
func (s *shortcutsWidget) Update(tea.Msg) (Widget, tea.Cmd) { return s, nil }
func (s *shortcutsWidget) View() string             { return s.content }

func (s *shortcutsWidget) OnClick(x, y int, _ tea.MouseButton) tea.Cmd {
	if s.bounds.W <= 0 {
		return nil
	}
	// Three cells per row; divider on the middle row is non-clickable.
	colW := s.bounds.W / 3
	if colW < 1 {
		colW = 1
	}
	var row int
	switch y {
	case 0:
		row = 0
	case 2:
		row = 1
	default:
		return nil
	}
	col := x / colW
	if col > 2 {
		col = 2
	}
	idx := row*3 + col
	switch idx {
	case 0: // [n] nouvelle licence → switch to Licenses (new)
		return func() tea.Msg { return widgets.SwitchViewMsg{ID: string(ViewLicenses)} }
	case 1: // [/] rechercher → Licenses
		return func() tea.Msg { return widgets.SwitchViewMsg{ID: string(ViewLicenses)} }
	case 2: // [x] révoquer → Revocation
		return func() tea.Msg { return widgets.SwitchViewMsg{ID: string(ViewRevocation)} }
	case 3: // [k] clés d'émission → Issuers
		return func() tea.Msg { return widgets.SwitchViewMsg{ID: string(ViewIssuers)} }
	case 4: // [i] identity.bin → Identities
		return func() tea.Msg { return widgets.SwitchViewMsg{ID: string(ViewIdentities)} }
	case 5: // [?] aide → push help overlay
		return func() tea.Msg { return pushOverlayMsg{overlay: NewHelpOverlay()} }
	}
	return nil
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
