package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
)

// serversModel is the root model for the Servers screen (ViewServers).
// It composes three ServerCard widgets in a row above a serverLog pane.
type serversModel struct {
	ctrl httpsrv.Controller // nil when bundle not wired

	cards [3]*ServerCard
	log   *serverLog

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
		// Fired by the card button — issue the actual controller call.
		if m.ctrl != nil {
			return m, startServerCmd(m.ctrl, msg.name)
		}

	case serverStopMsg:
		// Fired by the card button — issue the actual controller call.
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

	case serverLogClearMsg, serverLogFilterMsg:
		w, cmd := m.log.Update(msg)
		m.log, _ = w.(*serverLog)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
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

	// Forward remaining messages to cards and log.
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

	// ── Top action bar ────────────────────────────────────────────────────
	var startAll, stopAll string
	startAll = lipgloss.NewStyle().Foreground(Palette.Green).Bold(true).Padding(0, 1).
		Border(lipgloss.NormalBorder()).BorderForeground(Palette.Green).
		Render("[A] Start all")
	stopAll = lipgloss.NewStyle().Foreground(Palette.Red).Bold(true).Padding(0, 1).
		Border(lipgloss.NormalBorder()).BorderForeground(Palette.Red).
		Render("[Z] Stop all")
	topBar := lipgloss.JoinHorizontal(lipgloss.Top, startAll, "  ", stopAll,
		"  ", Mute.Render("s=start  S=stop  c=clear log  1-4=filter"))

	// ── Cards row ─────────────────────────────────────────────────────────
	cardViews := make([]string, 3)
	for i, card := range m.cards {
		cardViews[i] = card.View()
	}
	cardsRow := lipgloss.JoinHorizontal(lipgloss.Top, cardViews...)

	// ── Log pane ──────────────────────────────────────────────────────────
	chips := filterChips(m.log.filter)
	logTitle := GlowCyan.Render("Event log") + "  " + chips + "  " +
		Mute.Render("c=clear")
	logBody := m.log.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		cardsRow,
		logTitle,
		logBody,
	)
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
