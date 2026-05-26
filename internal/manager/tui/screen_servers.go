package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"context"

	"github.com/NimbleMarkets/ntcharts/sparkline"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// serverSubTab identifies which sub-tab (server type) is active.
type serverSubTab int

const (
	serverTabRevocation serverSubTab = iota
	serverTabHeartbeat
	serverTabProbe
)

// probeInnerView identifies which inner panel is active inside the Probe
// sub-tab. T = active tokens, H = history (consumed/expired), L = live log.
type probeInnerView int

const (
	probeViewTokens probeInnerView = iota
	probeViewHistory
	probeViewLive
)

// serverDescriptionText returns the one-line French role description for the
// named server. Shown at the top of the Status box (D-S34).
func serverDescriptionText(name string) string {
	switch name {
	case "revocation":
		return "Sert la CRL signée aux clients qui vérifient leurs licences."
	case "heartbeat":
		return "Reçoit les pings périodiques des licences actives (anti-replay)."
	default:
		return "Émet des tokens de probe + reçoit les rapports d'identification machine."
	}
}

// serverAPIExamples returns example curl commands for each server (D-S37).
func serverAPIExamples(name, addr string) string {
	if addr == "" || addr == "—" {
		addr = "localhost:8443"
	}
	base := "https://" + addr
	switch name {
	case "revocation":
		return strings.Join([]string{
			GlowCyan.Render("GET  ") + Mute.Render(base+"/crl"),
			Dim.Render("  → Télécharge la CRL signée (DER ou PEM selon Accept:)"),
			"",
			GlowCyan.Render("POST ") + Mute.Render(base+"/revoke"),
			Dim.Render("  Content-Type: application/json"),
			Dim.Render(`  {"license_uuid":"…","reason":"…","actor":"operator"}`),
			Dim.Render("  Authorization: Bearer <admin_token>"),
		}, "\n")
	case "heartbeat":
		return strings.Join([]string{
			GlowCyan.Render("POST ") + Mute.Render(base+"/heartbeat/<license_uuid>"),
			Dim.Render("  Content-Type: application/json"),
			Dim.Render(`  {"totp":"<6-digit>"}   // omit when TOTP not required`),
			"",
			GlowCyan.Render("GET  ") + Mute.Render(base+"/metrics"),
			Dim.Render("  → Prometheus-compatible heartbeat counters"),
		}, "\n")
	default:
		return strings.Join([]string{
			GlowCyan.Render("GET  ") + Mute.Render(base+"/probe/<token>"),
			Dim.Render("  → Échange one-shot; rapporte le fingerprint machine"),
			"",
			GlowCyan.Render("GET  ") + Mute.Render(base+"/probe/<token>/agent"),
			Dim.Render("  → Agent long-poll (streaming fingerprint updates)"),
		}, "\n")
	}
}

// serversModel is the root model for the Servers screen (ViewServers).
// It renders prototype sub-tabs (R/H/P) with a 2-column status+log layout.
type serversModel struct {
	svc  *service.Services  // for probe token queries (nil when not wired)
	ctrl httpsrv.Controller // nil when bundle not wired

	cards [3]*ServerCard
	log   *serverLog

	activeTab    serverSubTab   // R / H / P sub-tab
	probeView    probeInnerView // Probe inner T / H / L view
	probeTokens  []*ent.ProbeToken // populated when T view is active
	probeHistory []*ent.ProbeToken // populated when H view is active

	// adminTokens caches admin tokens by server name so the operator can
	// retrieve them on demand with [t] after they were generated (D-S36).
	adminTokens map[string]string

	// Per-server request-rate ring buffer (60 samples = 1 minute window).
	// Each tick records the delta of Requests vs the previous tick.
	reqHistory [3][60]float64
	reqHead    [3]int
	prevReqs   [3]uint64

	width, height int
}

var serverNames = [3]string{"revocation", "heartbeat", "probe"}

func newServersModel(svc *service.Services, ctrl httpsrv.Controller) serversModel {
	m := serversModel{
		svc:         svc,
		ctrl:        ctrl,
		log:         newServerLog(),
		adminTokens: make(map[string]string),
	}
	for i, name := range serverNames {
		m.cards[i] = newServerCard(name)
	}
	return m
}

func (m serversModel) Init() tea.Cmd { return serverStatusTick() }

// serverStatusTickMsg is emitted every second so the Status box (uptime,
// req/s) re-renders without waiting for a user event or an httpsrv event.
type serverStatusTickMsg struct{}

// serverStatusTick schedules the next refresh tick.
func serverStatusTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return serverStatusTickMsg{} })
}

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

	case serverStartedMsg, serverStoppedMsg:
		// Refresh card statuses immediately so the UI flips ON/OFF without
		// waiting for the first event to arrive on MergedEvents.
		m.refreshStatuses()
		// Also push a synthetic event into the log so the operator sees the
		// transition timestamped even before the server's own events fire.
		var ev httpsrv.Event
		switch v := msg.(type) {
		case serverStartedMsg:
			ev = httpsrv.Event{Server: v.name, Kind: "lifecycle", Note: "started"}
		case serverStoppedMsg:
			ev = httpsrv.Event{Server: v.name, Kind: "lifecycle", Note: "stopped"}
		}
		w, cmd := m.log.Update(serverEventMsg{ev: ev})
		m.log, _ = w.(*serverLog)
		return m, cmd

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

	case serverStatusTickMsg:
		// Re-arm the tick so we get refreshed every second. refreshStatuses
		// pulls a fresh Statuses snapshot so uptime, req tot, req/s update
		// without any user input.
		m.refreshStatuses()
		return m, serverStatusTick()

	case probeTokensLoadedMsg:
		m.probeTokens = msg.rows
		return m, nil

	case probeHistoryLoadedMsg:
		m.probeHistory = msg.rows
		return m, nil

	case probeViewSwitchMsg:
		m.probeView = msg.view
		switch msg.view {
		case probeViewTokens:
			return m, loadProbeTokensCmd(m.svc)
		case probeViewHistory:
			return m, loadProbeHistoryCmd(m.svc)
		}
		return m, nil

	case serverSubTabClickMsg:
		m.activeTab = msg.tab
		w, cmd := m.log.Update(serverLogFilterMsg{server: msg.srv})
		m.log, _ = w.(*serverLog)
		return m, cmd

	case serverLogClearMsg, serverLogFilterMsg, serverLogAutoScrollMsg:
		w, cmd := m.log.Update(msg)
		m.log, _ = w.(*serverLog)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		// Start/stop the active-tab server — keyboard mirror of the card button.
		// The card's Button widget only fires via mouse (no focus management);
		// this case gives operators a keyboard shortcut that always works.
		case "s":
			name := serverNames[m.activeTab]
			if m.ctrl == nil {
				return m, nil
			}
			if m.cards[m.activeTab].status.Running {
				return m, stopServerCmd(m.ctrl, name)
			}
			return m, startServerCmd(m.ctrl, name)
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
		case "g":
			// Regenerate admin token for the active server.
			name := serverNames[m.activeTab]
			return m, func() tea.Msg {
				body := fmt.Sprintf("Régénérer l'admin token de %s ?\n"+
					"L'ancien token sera invalidé immédiatement. Tous les clients\n"+
					"qui s'authentifient avec doivent être mis à jour.", name)
				return pushOverlayMsg{newConfirmOverlay(OverlayIDServerRegenTok,
					"Regenerate admin token", body, "regen", "annuler", true)}
			}

		case "T":
			// D-S36: show cached admin token on demand ([T] = capital to avoid
			// conflict with [t] probe inner-view tokens key).
			name := serverNames[m.activeTab]
			tok, ok := m.adminTokens[name]
			if !ok || tok == "" {
				return m, func() tea.Msg {
					return pushOverlayMsg{newErrorOverlay("Token inconnu",
						"Aucun token enregistré pour "+name+".\n"+
							"Génère-le avec [g] pour le stocker dans cette session.")}
				}
			}
			return m, func() tea.Msg {
				return pushOverlayMsg{NewOKOverlay("Admin token · "+name,
					GlowMagent.Render(tok))}
			}
		case "e":
			// Edit config: route to an input overlay that lets the operator
			// change the bind address. Persisted via UpdateServerConfig once
			// the result lands (handled in app.go).
			name := serverNames[m.activeTab]
			return m, func() tea.Msg {
				return pushOverlayMsg{newInputOverlay(OverlayIDServerEditBind,
					"Edit "+name+" bind address",
					"127.0.0.1:8443", 64)}
			}
		case "a":
			// Auto-scroll toggle — flip the log's stick-to-bottom flag so the
			// operator can pause / resume tail-follow without leaving the screen.
			w, cmd := m.log.Update(serverLogAutoScrollMsg{})
			m.log, _ = w.(*serverLog)
			return m, cmd

		case "i":
			// D-S37: [i] API info — push a help overlay with curl examples for
			// the active server. 'i' = info; doesn't conflict with any existing
			// Servers key (a=auto-scroll, A=global startAll, s=start/stop, g=regen).
			name := serverNames[m.activeTab]
			addr := m.cards[m.activeTab].status.ListenAddr
			examples := serverAPIExamples(name, addr)
			return m, func() tea.Msg {
				return pushOverlayMsg{newErrorOverlay("API · "+name, examples)}
			}
		// Probe inner-view keys — only meaningful when the Probe sub-tab is
		// active; on the other sub-tabs they fall through to the default
		// dispatcher (the tea.Update loop below).
		case "t":
			if m.activeTab == serverTabProbe {
				m.probeView = probeViewTokens
				return m, loadProbeTokensCmd(m.svc)
			}
		case "h":
			if m.activeTab == serverTabProbe {
				m.probeView = probeViewHistory
				return m, loadProbeHistoryCmd(m.svc)
			}
		case "l":
			if m.activeTab == serverTabProbe {
				m.probeView = probeViewLive
				return m, nil
			}
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
	for i, card := range m.cards {
		s, ok := statuses[card.name]
		if !ok {
			continue
		}
		card.SetStatus(s)
		// Record delta requests for the rate sparkline (per-second resolution
		// since the tick fires every 1s). Reset on stop so the spark line
		// doesn't carry a stale tail across restarts.
		if !s.Running {
			m.prevReqs[i] = 0
			m.reqHistory[i][m.reqHead[i]] = 0
		} else {
			delta := float64(s.Requests - m.prevReqs[i])
			if delta < 0 {
				delta = 0
			}
			m.reqHistory[i][m.reqHead[i]] = delta
			m.prevReqs[i] = s.Requests
		}
		m.reqHead[i] = (m.reqHead[i] + 1) % len(m.reqHistory[i])
	}
}

// sparklineRequests renders the request-rate ring buffer for the given server
// index as an ntcharts sparkline. The line shows the last 60 seconds.
func (m serversModel) sparklineRequests(idx, width int) string {
	if width < 8 {
		return ""
	}
	sp := sparkline.New(width, 1)
	// Replay the ring in chronological order (oldest first).
	head := m.reqHead[idx]
	for i := 0; i < len(m.reqHistory[idx]); i++ {
		pos := (head + i) % len(m.reqHistory[idx])
		sp.Push(m.reqHistory[idx][pos])
	}
	sp.Draw()
	return sp.View()
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
			tab = serverSubTabActive.Render(
				GlowCyan.Render("["+td.key+"]") + " " + Base.Render(td.label) + " " + dot,
			)
		} else {
			tab = serverSubTabInactive.Render(
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
	// Flat brackets to match the dashboard pattern (3-line bordered pills
	// don't fit single-line layouts like the Status box header).
	statusDot := Mute.Render("●")
	statusPill := serverPillOff.Render("[OFF]")
	if s.Running {
		statusDot = GlowGreen.Render("●")
		statusPill = serverPillOn.Render("[ON]")
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
		ratePerSec := "—"
		if !s.StartedAt.IsZero() {
			elapsed := time.Since(s.StartedAt)
			uptime = formatDuration(elapsed)
			if secs := elapsed.Seconds(); secs > 0 {
				ratePerSec = fmt.Sprintf("%.2f", float64(s.Requests)/secs)
			}
		}
		// Sparkline of the last 60s request deltas — width fits the Status box.
		spark := m.sparklineRequests(activeCardIdx, m.width/2-12)
		statusLines = append(statusLines,
			Dim.Render("url    ")+" "+GlowCyan.Render(addrStr),
			Dim.Render("uptime ")+" "+Base.Render(uptime),
			Dim.Render("req tot")+" "+Base.Render(fmt.Sprintf("%d", s.Requests)),
			Dim.Render("req/s  ")+" "+Base.Render(ratePerSec+Dim.Render(" (depuis le start)")),
			Dim.Render("rate ▶ ")+" "+spark,
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
	// D-S34: one-line French role description shown at top of status box.
	serverRole := Mute.Render(serverDescriptionText(serverNames[m.activeTab]))
	statusBox := BoxFocused.Width(m.width/2 - 3).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			GlowCyan.Render("Status")+"  "+hintStop,
			serverRole,
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

	// ── Right column: live log (or Probe-specific inner view) ────────────
	// Cap to the budget left by the leftCol (m.width/2 - 1) + 2-cell gap so
	// long lines (e.g. live-log title with action hints, no-event message)
	// don't leak past the right edge of the screen at narrow widths.
	rightColW := m.width - (m.width/2 - 1) - 2
	if rightColW < 20 {
		rightColW = 20
	}
	rightCol := lipgloss.NewStyle().Width(rightColW).Render(m.renderRightColumn())

	// ── 2-column body ─────────────────────────────────────────────────────
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "  ", rightCol)

	// ── Action bar (clickable) ───────────────────────────────────────────
	// Pinned to a known Y so OnClick can hit-test each chip without having
	// to know the variable-height status/config boxes above.
	actionBar := m.renderActionBar()

	// Status bar is rendered by the root chrome (viewReady picks up Hints()
	// via the ScreenWithHints interface) — don't duplicate it here.
	return lipgloss.JoinVertical(lipgloss.Left, subTabBar, body, "", actionBar)
}

// serverActionChips is the ordered list of action chips rendered in the
// action bar. Each chip is hit-tested by X-range in OnClick.
var serverActionChips = []struct {
	key, label string
}{
	{"s", "start/stop"},
	{"e", "edit bind"},
	{"g", "regen token"},
	{"c", "clear log"},
	{"a", "auto-scroll"},
}

// renderActionBar emits a horizontal strip of [key] label chips. Mouse clicks
// on these chips are routed by m.OnClick using the same chip layout.
func (m serversModel) renderActionBar() string {
	var parts []string
	for _, c := range serverActionChips {
		parts = append(parts, HintKey.Render("["+c.key+"]")+" "+Dim.Render(c.label))
	}
	return " " + lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// actionBarHit returns the action key for a click at (x,y), or "" if no
// chip was hit. Mirrors renderActionBar's layout exactly.
func (m serversModel) actionBarHit(x int) string {
	cursor := 1 // matches the leading " " in renderActionBar
	for _, c := range serverActionChips {
		chip := HintKey.Render("["+c.key+"]") + " " + Dim.Render(c.label)
		w := lipgloss.Width(chip)
		if x >= cursor && x < cursor+w {
			return c.key
		}
		cursor += w
	}
	return ""
}

// renderRightColumn renders the right-side panel of the Servers screen.
// For the Revocation + Heartbeat sub-tabs it shows the live log only.
// For Probe it additionally shows an inner-tab strip with three views:
// [T] Tokens actifs · [H] History · [L] Live log.
func (m serversModel) renderRightColumn() string {
	if m.activeTab != serverTabProbe {
		return m.renderLogPanel()
	}
	// Probe inner-tab strip — active view gets a cyan underline matching the
	// outer sub-tab styling. Counts come from the merged event log for now
	// (real svc.Probe.History would feed them; this is a stub layout).
	tabDefs := []struct {
		key   string
		view  probeInnerView
		label string
	}{
		{"t", probeViewTokens, fmt.Sprintf("Tokens actifs (%d)", 0)},
		{"h", probeViewHistory, fmt.Sprintf("History (%d)", 0)},
		{"l", probeViewLive, "Live log"},
	}
	var parts []string
	for _, td := range tabDefs {
		key := fmt.Sprintf("[%s]", strings.ToUpper(td.key))
		var s string
		if m.probeView == td.view {
			s = lipgloss.NewStyle().Padding(0, 1).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(Palette.Cyan).
				Render(GlowCyan.Render(key) + " " + Base.Render(td.label))
		} else {
			s = lipgloss.NewStyle().Padding(0, 1).
				Render(Mute.Render(key) + " " + Dim.Render(td.label))
		}
		parts = append(parts, s)
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	var content string
	switch m.probeView {
	case probeViewTokens:
		content = m.renderProbeTokens()
	case probeViewHistory:
		content = m.renderProbeHistory()
	case probeViewLive:
		content = m.renderLogPanel()
	}
	return lipgloss.JoinVertical(lipgloss.Left, tabBar, content)
}

// renderLogPanel produces the Live log title + viewport.
func (m serversModel) renderLogPanel() string {
	logFilter := serverNames[m.activeTab]
	title := GlowCyan.Render(fmt.Sprintf("Live log · filter %s", logFilter)) +
		"  " + Mute.Render("[c] clear · [a] auto-scroll")
	return lipgloss.JoinVertical(lipgloss.Left, title, m.log.View())
}

// renderProbeTokens lists outstanding (not-yet-consumed, not-expired) probe
// tokens. Columns mirror the prototype: TOKEN / LABEL / ISSUED / TTL / STATE.
// Rows come from svc.Probe.History filtered to those still in waiting state.
func (m serversModel) renderProbeTokens() string {
	// D-S38: [q] for QR renamed to [Q] to avoid collision with global quit key.
	hint := HintKey.Render("[n]") + Dim.Render(" générer · ") +
		HintKey.Render("[Q]") + Dim.Render(" QR · ") +
		HintKey.Render("[x]") + Dim.Render(" révoquer")
	title := GlowCyan.Render(fmt.Sprintf("Tokens actifs (%d)", len(m.probeTokens)))
	header := title + "  " + hint

	var body string
	if len(m.probeTokens) == 0 {
		body = Dim.Render("\n  aucun token actif — [n] pour en générer un\n")
	} else {
		col := func(label string, w int) string {
			return lipgloss.NewStyle().Width(w).Render(GlowCyan.Render(label))
		}
		head := col("TOKEN", 22) + col("LABEL", 16) + col("ISSUED", 20) + col("TTL", 8) + col("STATE", 10)
		lines := []string{head}
		for _, t := range m.probeTokens {
			tok := t.ID
			if len(tok) > 20 {
				tok = tok[:20] + "…"
			}
			ttl := "—"
			if !t.ExpiresAt.IsZero() {
				ttl = time.Until(t.ExpiresAt).Round(time.Second).String()
			}
			state := "waiting"
			if !t.UsedAt.IsZero() {
				state = "consumed"
			}
			lines = append(lines,
				lipgloss.NewStyle().Width(22).Render(tok)+
					lipgloss.NewStyle().Width(16).Render(t.Label)+
					lipgloss.NewStyle().Width(20).Render(t.CreatedAt.Format("2006-01-02 15:04"))+
					FgYellow.Width(8).Render(ttl)+
					FgYellow.Width(10).Render(state),
			)
		}
		body = strings.Join(lines, "\n")
	}

	// Astuce callout matching the prototype Servers — Probe panel.
	astuceTitle := GlowCyan.Render("Astuce")
	astuceBody := Dim.Render(
		"Le ProbeServer s'utilise surtout depuis le wizard licence → bindings → machine →\n" +
			" « récupérer depuis une machine distante » (overlay). Mais tu peux le démarrer ici\n" +
			" pour générer un batch de tokens (cas d'usage §10 Probe batch).")
	astuce := BoxStyle.Render(astuceTitle + "\n\n" + astuceBody)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, "", astuce)
}

// renderProbeHistory shows received fingerprints (consumed tokens) with the
// machine identity each one delivered. Columns mirror the prototype:
// RECEIVED / LABEL / HOSTNAME / OS / LOCAL / USED IN.
func (m serversModel) renderProbeHistory() string {
	hint := HintKey.Render("[d]") + Dim.Render(" détail · ") +
		HintKey.Render("[↵]") + Dim.Render(" créer licence depuis…")
	title := GlowCyan.Render(fmt.Sprintf("Fingerprints history (%d)", len(m.probeHistory)))
	if len(m.probeHistory) == 0 {
		return title + "  " + hint + "\n" +
			Dim.Render("  aucun fingerprint reçu — démarre un agent pour voir la liste ici")
	}
	col := func(label string, w int) string {
		return lipgloss.NewStyle().Width(w).Render(GlowCyan.Render(label))
	}
	head := col("RECEIVED", 18) + col("LABEL", 14) + col("HOSTNAME", 16) + col("OS", 10) + col("LOCAL", 14)
	lines := []string{title + "  " + hint, head}
	for _, t := range m.probeHistory {
		recv := t.UsedAt.Format("2006-01-02 15:04")
		local := t.LocalHex
		if len(local) > 12 {
			local = local[:12] + "…"
		}
		lines = append(lines,
			lipgloss.NewStyle().Width(18).Render(recv)+
				lipgloss.NewStyle().Width(14).Render(t.Label)+
				lipgloss.NewStyle().Width(16).Render(t.Hostname)+
				lipgloss.NewStyle().Width(10).Render(t.Os)+
				FgCyan.Width(14).Render(local),
		)
	}
	return strings.Join(lines, "\n")
}

// probeTokensLoadedMsg / probeHistoryLoadedMsg carry the result of an async
// probe history fetch; populated when the operator opens the Tokens / History
// inner views on the Probe sub-tab.
type probeTokensLoadedMsg struct {
	rows []*ent.ProbeToken
	err  error
}
type probeHistoryLoadedMsg struct {
	rows []*ent.ProbeToken
	err  error
}

// loadProbeTokensCmd fetches recent probe-token rows. svc.Probe.History returns
// the most recent N regardless of state; we filter into "waiting" vs "used"
// here to populate the two views in one round-trip.
func loadProbeTokensCmd(svc *service.Services) tea.Cmd {
	if svc == nil {
		return nil
	}
	return func() tea.Msg {
		rows, err := svc.Probe.History(context.Background(), 20)
		if err != nil {
			return probeTokensLoadedMsg{err: err}
		}
		var active []*ent.ProbeToken
		for _, r := range rows {
			if r.UsedAt.IsZero() {
				active = append(active, r)
			}
		}
		return probeTokensLoadedMsg{rows: active}
	}
}

func loadProbeHistoryCmd(svc *service.Services) tea.Cmd {
	if svc == nil {
		return nil
	}
	return func() tea.Msg {
		rows, err := svc.Probe.History(context.Background(), 20)
		if err != nil {
			return probeHistoryLoadedMsg{err: err}
		}
		var used []*ent.ProbeToken
		for _, r := range rows {
			if !r.UsedAt.IsZero() {
				used = append(used, r)
			}
		}
		return probeHistoryLoadedMsg{rows: used}
	}
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
	// D-S39: outer R/H/P sub-tab bar sits at Y=ChromeRows (row 4).
	// Walk the rendered tab widths to find which tab was clicked.
	if y == ChromeRows {
		type tabDef struct {
			tab serverSubTab
			srv string
		}
		tabs := []tabDef{
			{serverTabRevocation, "revocation"},
			{serverTabHeartbeat, "heartbeat"},
			{serverTabProbe, "probe"},
		}
		// Reconstruct the visual widths of each tab cell by measuring the same
		// strings rendered in View(). Active tab gets a bold underline border
		// (NormalBorder bottom only) which changes the Padding; measure the
		// inactive variant so the widths are consistent regardless of current state.
		// The rendered tabs are joined horizontally; we walk them left-to-right.
		cursor := 0
		for _, td := range tabs {
			// Inactive tab: Padding(0,1) + "[X] label ●" (no border adds height but not width).
			label := "[" + func() string {
				switch td.tab {
				case serverTabRevocation:
					return "R"
				case serverTabHeartbeat:
					return "H"
				default:
					return "P"
				}
			}() + "] " + func() string {
				switch td.tab {
				case serverTabRevocation:
					return "Revocation"
				case serverTabHeartbeat:
					return "Heartbeat"
				default:
					return "Fingerprint probe"
				}
			}() + " ●"
			// Padding(0,1) adds 2 chars (1 left + 1 right).
			w := lipgloss.Width(label) + 2
			if x >= cursor && x < cursor+w {
				target := td.tab
				srv := td.srv
				return func() tea.Msg {
					return serverSubTabClickMsg{tab: target, srv: srv}
				}
			}
			cursor += w
		}
	}

	// Probe inner-tab strip on the active Probe sub-tab: lives at Y=5 (top
	// row of the right column, just below the outer sub-tab bar at Y=4).
	if m.activeTab == serverTabProbe && y == 5 {
		leftColW := m.width/2 - 1
		localX := x - leftColW
		// Fixed widths track renderRightColumn's Padding(0,1) layout.
		hits := []struct {
			view probeInnerView
			w    int
		}{
			{probeViewTokens, 21},
			{probeViewHistory, 17},
			{probeViewLive, 12},
		}
		cursor := 0
		for _, h := range hits {
			if localX >= cursor && localX < cursor+h.w {
				target := h.view
				return func() tea.Msg { return probeViewSwitchMsg{view: target} }
			}
			cursor += h.w
		}
	}
	// Action bar lives at m.height - 2 (one above the global status bar).
	if y == m.height-2 {
		switch m.actionBarHit(x) {
		case "s":
			name := serverNames[m.activeTab]
			if m.cards[m.activeTab].status.Running {
				return func() tea.Msg { return serverStopMsg{name: name} }
			}
			return func() tea.Msg { return serverStartMsg{name: name} }
		case "e":
			name := serverNames[m.activeTab]
			return func() tea.Msg {
				return pushOverlayMsg{newInputOverlay(OverlayIDServerEditBind,
					"Edit "+name+" bind address", "127.0.0.1:8443", 64)}
			}
		case "g":
			name := serverNames[m.activeTab]
			return func() tea.Msg {
				body := fmt.Sprintf("Régénérer l'admin token de %s ?\n"+
					"L'ancien token sera invalidé immédiatement.", name)
				return pushOverlayMsg{newConfirmOverlay(OverlayIDServerRegenTok,
					"Regenerate admin token", body, "regen", "annuler", true)}
			}
		case "c":
			return func() tea.Msg { return serverLogClearMsg{} }
		case "a":
			return func() tea.Msg { return serverLogAutoScrollMsg{} }
		}
		return nil
	}
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

// probeViewSwitchMsg signals a Probe inner-tab (T/H/L) click; handled in
// Update to mutate probeView + fire the matching load command.
type probeViewSwitchMsg struct{ view probeInnerView }

// serverSubTabActive / Inactive pre-built once; the sub-tab row is rendered
// every tick (~1 Hz). Variables not constants because lipgloss.Style is a
// runtime value, not a compile-time literal.
var (
	serverSubTabActive = lipgloss.NewStyle().
				Foreground(Palette.Fg).Bold(true).
				Padding(0, 1).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(Palette.Cyan)
	serverSubTabInactive = lipgloss.NewStyle().
				Foreground(Palette.FgDim).
				Padding(0, 1)
)
