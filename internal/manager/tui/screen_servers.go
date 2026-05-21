package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
)

// serverSubTab identifies which sub-tab (server type) is active.
type serverSubTab int

const (
	serverTabRevocation serverSubTab = iota
	serverTabHeartbeat
	serverTabProbe
)

// serversModel is the root model for the Servers screen (ViewServers).
// It renders prototype sub-tabs (R/H/P) with a 2-column status+log layout.
type serversModel struct {
	ctrl httpsrv.Controller // nil when bundle not wired

	cards [3]*ServerCard
	log   *serverLog

	activeTab serverSubTab // R / H / P sub-tab
	width, height int
}

var serverNames = [3]string{"revocation", "heartbeat", "probe"}

func newServersModel(ctrl httpsrv.Controller) serversModel {
	m := serversModel{ctrl: ctrl, log: newServerLog()}
	for i, name := range serverNames {
		m.cards[i] = newServerCard(name)
	}
	return m
}

func (m serversModel) Init() tea.Cmd { return nil }

func (m serversModel) Update(msg tea.Msg) (serversModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()

	case serverEventMsg:
		// Refresh card statuses on every event then forward to log.
		m.refreshStatuses()
		w, cmd := m.log.Update(msg)
		m.log, _ = w.(*serverLog)
		return m, cmd

	case serverStartMsg:
		if m.ctrl != nil {
			return m, startServerCmd(m.ctrl, msg.name)
		}

	case serverStopMsg:
		if m.ctrl != nil {
			return m, stopServerCmd(m.ctrl, msg.name)
		}

	case serverActionErrMsg:
		// Push error event into the log so the operator sees it.
		errEv := httpsrv.Event{
			Server: msg.name,
			Kind:   "error",
			Note:   msg.err.Error(),
		}
		w, cmd := m.log.Update(serverEventMsg{ev: errEv})
		m.log, _ = w.(*serverLog)
		return m, cmd

	case serverSubTabClickMsg:
		m.activeTab = msg.tab
		w, cmd := m.log.Update(serverLogFilterMsg{server: msg.srv})
		m.log, _ = w.(*serverLog)
		return m, cmd

	case serverLogClearMsg, serverLogFilterMsg:
		w, cmd := m.log.Update(msg)
		m.log, _ = w.(*serverLog)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		// Sub-tab selection — R/H/P matches prototype hotkeys.
		case "R":
			m.activeTab = serverTabRevocation
			w, cmd := m.log.Update(serverLogFilterMsg{server: "revocation"})
			m.log, _ = w.(*serverLog)
			return m, cmd
		case "H":
			m.activeTab = serverTabHeartbeat
			w, cmd := m.log.Update(serverLogFilterMsg{server: "heartbeat"})
			m.log, _ = w.(*serverLog)
			return m, cmd
		case "P":
			m.activeTab = serverTabProbe
			w, cmd := m.log.Update(serverLogFilterMsg{server: "probe"})
			m.log, _ = w.(*serverLog)
			return m, cmd
		// Legacy numeric filter keys kept for muscle-memory compatibility.
		case "1":
			w, cmd := m.log.Update(serverLogFilterMsg{server: ""})
			m.log, _ = w.(*serverLog)
			return m, cmd
		case "2":
			w, cmd := m.log.Update(serverLogFilterMsg{server: "revocation"})
			m.log, _ = w.(*serverLog)
			return m, cmd
		case "3":
			w, cmd := m.log.Update(serverLogFilterMsg{server: "heartbeat"})
			m.log, _ = w.(*serverLog)
			return m, cmd
		case "4":
			w, cmd := m.log.Update(serverLogFilterMsg{server: "probe"})
			m.log, _ = w.(*serverLog)
			return m, cmd
		case "c":
			w, cmd := m.log.Update(serverLogClearMsg{})
			m.log, _ = w.(*serverLog)
			return m, cmd
		}
	}

	var cmds []tea.Cmd
	for i := range m.cards {
		w, cmd := m.cards[i].Update(msg)
		m.cards[i], _ = w.(*ServerCard)
		cmds = append(cmds, cmd)
	}
	w, cmd := m.log.Update(msg)
	m.log, _ = w.(*serverLog)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// refreshStatuses pulls a fresh Status snapshot from the controller and
// pushes it into each card. Called after each incoming event so cards
// reflect the actual server state without polling.
func (m *serversModel) refreshStatuses() {
	if m.ctrl == nil {
		return
	}
	statuses := m.ctrl.Statuses()
	for _, card := range m.cards {
		if s, ok := statuses[card.name]; ok {
			card.SetStatus(s)
		}
	}
}

// layout assigns bounding boxes to all child widgets.
func (m *serversModel) layout() {
	w, h := m.width, m.height
	if w == 0 || h == 0 {
		return
	}

	// Reserve 2 chrome rows (already stripped by viewReady), 1 for top button
	// bar, 1 for filter chips, 1 for log header.
	cardH := h / 3
	if cardH < 8 {
		cardH = 8
	}
	logH := h - cardH - 3 // 3 = topbar + chips + log title
	if logH < 4 {
		logH = 4
	}

	cardW := w / 3
	for i, card := range m.cards {
		card.Layout(Rect{X: i * cardW, Y: 1, W: cardW, H: cardH})
	}
	m.log.Layout(Rect{X: 0, Y: cardH + 3, W: w, H: logH})
}

func (m serversModel) View() string {
	if m.width == 0 {
		return Mute.Render("loading…")
	}

	// ── Sub-tab bar (R / H / P) ───────────────────────────────────────────
	type tabDef struct {
		key   string
		id    serverSubTab
		label string
		cardI int
	}
	tabs := []tabDef{
		{"R", serverTabRevocation, "Revocation", 0},
		{"H", serverTabHeartbeat, "Heartbeat", 1},
		{"P", serverTabProbe, "Fingerprint probe", 2},
	}

	var tabParts []string
	for _, td := range tabs {
		card := m.cards[td.cardI]
		running := card.status.Running
		dot := Mute.Render("●")
		if running {
			dot = GlowGreen.Render("●")
		}
		active := m.activeTab == td.id
		var tab string
		if active {
			tab = lipgloss.NewStyle().
				Foreground(Palette.Fg).Bold(true).
				Padding(0, 1).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(Palette.Cyan).
				Render(
					GlowCyan.Render("["+td.key+"]") + " " + Base.Render(td.label) + " " + dot,
				)
		} else {
			tab = lipgloss.NewStyle().
				Foreground(Palette.FgDim).
				Padding(0, 1).
				Render(
					Mute.Render("["+td.key+"]") + " " + Dim.Render(td.label) + " " + dot,
				)
		}
		tabParts = append(tabParts, tab)
	}
	fanInNote := Mute.Render("  events fan-in via httpsrv.MergedEvents()")
	subTabBar := lipgloss.JoinHorizontal(lipgloss.Top,
		append(tabParts, fanInNote)...,
	)

	// ── Determine active card ─────────────────────────────────────────────
	activeCardIdx := int(m.activeTab)
	card := m.cards[activeCardIdx]
	s := card.status

	// ── Status box (left column top) ──────────────────────────────────────
	statusDot := Mute.Render("●")
	statusPill := PillOff.Render("OFF")
	if s.Running {
		statusDot = GlowGreen.Render("●")
		statusPill = PillOn.Render(" ON ")
	}
	addrStr := s.ListenAddr
	if addrStr == "" {
		addrStr = "—"
	}
	statusLines := []string{
		lipgloss.JoinHorizontal(lipgloss.Top, statusDot, " ", statusPill, "  ", Dim.Render("port"), " ", Base.Render(addrStr)),
	}
	if s.Running {
		uptime := "—"
		if !s.StartedAt.IsZero() {
			uptime = formatDuration(time.Since(s.StartedAt))
		}
		statusLines = append(statusLines,
			Dim.Render("url    ")+" "+GlowCyan.Render(addrStr),
			Dim.Render("uptime ")+" "+Base.Render(uptime),
			Dim.Render("reqs   ")+" "+Base.Render(fmt.Sprintf("%d", s.Requests)),
		)
	} else {
		statusLines = append(statusLines,
			Mute.Render("— server stopped — start with ")+HintKey.Render("s"),
		)
	}
	if s.LastError != "" {
		statusLines = append(statusLines, GlowRed.Render("⚠ "+s.LastError))
	}
	hintStop := Dim.Render("[s] " + func() string {
		if s.Running {
			return "stop"
		}
		return "start"
	}())
	statusBox := BoxFocused.Width(m.width/2 - 3).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			GlowCyan.Render("Status")+"  "+hintStop,
			"",
			lipgloss.JoinVertical(lipgloss.Left, statusLines...),
		),
	)

	// ── Config box (left column bottom) ──────────────────────────────────
	configLines := []string{
		Dim.Render("port       ") + Base.Render(addrStr),
		Dim.Render("TLS cert   ") + Mute.Render("/etc/license-manager/tls.crt"),
		Dim.Render("TLS key    ") + Mute.Render("/etc/license-manager/tls.key"),
		Dim.Render("admin tok  ") + func() string {
			if s.Running {
				return Mute.Render("tk_•••••••••••••• [g] regen")
			}
			return GlowMagent.Render("tk_aB3xZ9…") + Dim.Render(" shown once")
		}(),
	}
	if serverNames[m.activeTab] == "probe" {
		configLines = append(configLines,
			Dim.Render("token TTL  ")+Base.Render("60s"),
			Dim.Render("max tokens ")+Base.Render("8"),
		)
	}
	endpointNote := func() string {
		switch m.activeTab {
		case serverTabRevocation:
			return Dim.Render("endpoint: ") + GlowCyan.Render("/crl, /revoke (admin)")
		case serverTabHeartbeat:
			return Dim.Render("endpoint: ") + GlowCyan.Render("/heartbeat/<id>, /metrics")
		default:
			return Dim.Render("endpoints: ") + GlowCyan.Render("/probe/<token> (one-shot), /probe/<token>/agent")
		}
	}()
	configLines = append(configLines, "", endpointNote)
	configBox := BoxStyle.Width(m.width/2 - 3).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			append([]string{Dim.Render("Configuration") + "  " + Mute.Render("[e] edit · [g] regen token")},
				configLines...)...,
		),
	)
	leftCol := lipgloss.JoinVertical(lipgloss.Left, statusBox, configBox)

	// ── Right column: live log ────────────────────────────────────────────
	logFilter := serverNames[m.activeTab]
	logTitle := GlowCyan.Render(fmt.Sprintf("Live log · filter %s", logFilter)) +
		"  " + Mute.Render("[c] clear · [a] auto-scroll")
	logBody := m.log.View()
	rightCol := lipgloss.JoinVertical(lipgloss.Left, logTitle, logBody)

	// ── 2-column body ─────────────────────────────────────────────────────
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "  ", rightCol)

	// Status bar is rendered by the root chrome (viewReady picks up Hints()
	// via the ScreenWithHints interface) — don't duplicate it here.
	return lipgloss.JoinVertical(lipgloss.Left, subTabBar, body)
}

// startAllCmd issues Start for every server name via the controller.
func startAllCmd(ctrl httpsrv.Controller) tea.Cmd {
	var cmds []tea.Cmd
	for _, name := range serverNames {
		n := name
		cmds = append(cmds, startServerCmd(ctrl, n))
	}
	return tea.Batch(cmds...)
}

// stopAllCmd issues Stop for every server name via the controller.
func stopAllCmd(ctrl httpsrv.Controller) tea.Cmd {
	var cmds []tea.Cmd
	for _, name := range serverNames {
		n := name
		cmds = append(cmds, stopServerCmd(ctrl, n))
	}
	return tea.Batch(cmds...)
}

// activeServerCount returns the number of running servers from a Statuses map.
func activeServerCount(statuses map[string]httpsrv.Status) int {
	n := 0
	for _, s := range statuses {
		if s.Running {
			n++
		}
	}
	return n
}

// serverCountLabel formats "N/3 running" for the dashboard tile.
func serverCountLabel(statuses map[string]httpsrv.Status) string {
	return fmt.Sprintf("%d/3 running", activeServerCount(statuses))
}

// OnClick handles sub-tab bar clicks (Y=4) AND the status pill / start hint
// in the left-column Status box (~Y=7 for the [s] start text, Y=9 for the
// ON/OFF pill).
//
// Chrome rows 0..3, sub-tabs Y=4, status box starts at Y=5 (top border).
// Inside the status box: row 0=top border, row 1=title row, row 2=blank,
// row 3=●/port line, row 4=stopped hint OR url, etc.
func (m serversModel) OnClick(x, y, _ int) tea.Cmd {
	// Click on the OFF/ON pill area or the [s] start hint toggles the active
	// server. Pill renders inside the status box at row 3 (X≈3..10).
	if y == 7 || y == 9 {
		name := serverNames[m.activeTab]
		if m.cards[m.activeTab].status.Running {
			return func() tea.Msg { return serverStopMsg{name: name} }
		}
		return func() tea.Msg { return serverStartMsg{name: name} }
	}
	if y != 4 {
		return nil
	}
	labels := []struct {
		label string
		tab   serverSubTab
		srv   string
	}{
		{"[R] Revocation ●", serverTabRevocation, "revocation"},
		{"[H] Heartbeat ●", serverTabHeartbeat, "heartbeat"},
		{"[P] Fingerprint probe ●", serverTabProbe, "probe"},
	}
	cursor := 0
	for _, t := range labels {
		w := lipgloss.Width(t.label) + 2 // 1 left + 1 right padding
		if x >= cursor && x < cursor+w {
			target, srv := t.tab, t.srv
			return func() tea.Msg { return serverSubTabClickMsg{tab: target, srv: srv} }
		}
		cursor += w
	}
	return nil
}

// serverSubTabClickMsg signals a sub-tab click; handled in Update.
type serverSubTabClickMsg struct {
	tab serverSubTab
	srv string
}
