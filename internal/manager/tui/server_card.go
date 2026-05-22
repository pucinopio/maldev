package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/tui/core"
	"github.com/oioio-space/maldev/internal/manager/tui/widgets"
)

// serverStartMsg is fired when the operator presses Start on a card.
type serverStartMsg struct{ name string }

// serverStopMsg is fired when the operator presses Stop on a card.
type serverStopMsg struct{ name string }

// serverStartedMsg / serverStoppedMsg signal that the action succeeded — they
// are distinct from serverStartMsg/serverStopMsg (the *triggers*) so the
// Update handler doesn't loop by re-issuing the same command.
type serverStartedMsg struct{ name string }
type serverStoppedMsg struct{ name string }

// serverActionErrMsg carries an error from a start/stop attempt.
type serverActionErrMsg struct {
	name string
	err  error
}

// ServerCard renders a single server status box (name, running indicator,
// listen address, uptime, request counter, last request, last error) plus
// Start / Stop buttons. It implements Widget and Clickable via its children.
type ServerCard struct {
	name   string
	status httpsrv.Status

	startBtn *widgets.Button
	stopBtn  *widgets.Button

	bounds core.Rect
}

// newServerCard constructs a card for the named server.
func newServerCard(name string) *ServerCard {
	sc := &ServerCard{name: name}
	sc.startBtn = widgets.NewButton("Start", "s", func() tea.Cmd {
		n := sc.name
		return func() tea.Msg { return serverStartMsg{name: n} }
	})
	sc.stopBtn = widgets.NewButton("Stop", "S", func() tea.Cmd {
		n := sc.name
		return func() tea.Msg { return serverStopMsg{name: n} }
	})
	return sc
}

func (sc *ServerCard) Layout(bounds core.Rect) {
	sc.bounds = bounds
	// Buttons sit at the bottom row inside the card border (2 cells padding each side).
	btnY := bounds.Y + bounds.H - 2
	btnH := 1
	half := (bounds.W - 4) / 2
	sc.startBtn.Layout(core.Rect{X: bounds.X + 2, Y: btnY, W: half, H: btnH})
	sc.stopBtn.Layout(core.Rect{X: bounds.X + 2 + half + 1, Y: btnY, W: half, H: btnH})
}

func (sc *ServerCard) Bounds() core.Rect { return sc.bounds }

// SetStatus refreshes the displayed status data without rebuilding the card.
func (sc *ServerCard) SetStatus(s httpsrv.Status) { sc.status = s }

func (sc *ServerCard) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	var cmds []tea.Cmd
	w, cmd := sc.startBtn.Update(msg)
	sc.startBtn, _ = w.(*widgets.Button)
	cmds = append(cmds, cmd)
	w, cmd = sc.stopBtn.Update(msg)
	sc.stopBtn, _ = w.(*widgets.Button)
	cmds = append(cmds, cmd)
	return sc, tea.Batch(cmds...)
}

func (sc *ServerCard) View() string {
	w := sc.bounds.W
	if w < 4 {
		return ""
	}

	var runPill string
	if sc.status.Running {
		runPill = PillOn.Render(" ON ")
	} else {
		runPill = PillOff.Render("OFF ")
	}

	addr := sc.status.ListenAddr
	if addr == "" {
		addr = Mute.Render("—")
	}

	uptime := "—"
	if sc.status.Running && !sc.status.StartedAt.IsZero() {
		uptime = formatDuration(time.Since(sc.status.StartedAt))
	}

	lastReq := "—"
	if !sc.status.LastReq.IsZero() {
		lastReq = sc.status.LastReq.Format("15:04:05")
	}

	lastErr := ""
	if sc.status.LastError != "" {
		lastErr = GlowRed.Render("⚠ " + sc.status.LastError)
	}

	lines := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			GlowCyan.Render(fmt.Sprintf("%-12s", sc.name)), " ", runPill),
		Dim.Render("Addr:    ") + Base.Render(addr),
		Dim.Render("Uptime:  ") + Base.Render(uptime),
		Dim.Render("Reqs:    ") + Base.Render(fmt.Sprintf("%d", sc.status.Requests)),
		Dim.Render("LastReq: ") + Base.Render(lastReq),
	}
	if lastErr != "" {
		lines = append(lines, lastErr)
	}

	// Pad to fill height minus 3 (border + button row).
	innerH := sc.bounds.H - 3
	for len(lines) < innerH {
		lines = append(lines, "")
	}

	btnRow := lipgloss.JoinHorizontal(lipgloss.Top,
		sc.startBtn.View(), " ", sc.stopBtn.View())

	body := lipgloss.JoinVertical(lipgloss.Left,
		append(lines, btnRow)...,
	)

	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(Palette.Border).
		Padding(0, 1).
		Width(w)
	if sc.status.Running {
		border = border.BorderForeground(Palette.Green)
	}
	return border.Render(body)
}

// OnClick implements core.Clickable — delegates to child buttons.
func (sc *ServerCard) OnClick(x, y int, btn tea.MouseButton) tea.Cmd {
	// Translate to absolute coords for child hit-test.
	abs := core.Rect{X: sc.bounds.X + x, Y: sc.bounds.Y + y, W: 1, H: 1}
	if sc.startBtn.Bounds().Contains(abs.X, abs.Y) {
		return sc.startBtn.OnClick(0, 0, btn)
	}
	if sc.stopBtn.Bounds().Contains(abs.X, abs.Y) {
		return sc.stopBtn.OnClick(0, 0, btn)
	}
	return nil
}

// Children exposes buttons for depth-first mouse dispatch in dispatchClick.
func (sc *ServerCard) Children() []Widget {
	return []Widget{sc.startBtn, sc.stopBtn}
}

// startServerCmd issues a Start call on the controller in a background
// goroutine and reports completion via serverStartedMsg (not the trigger
// serverStartMsg — using the same type would loop forever).
func startServerCmd(ctrl httpsrv.Controller, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := ctrl.Start(ctx, name); err != nil {
			return serverActionErrMsg{name: name, err: err}
		}
		return serverStartedMsg{name: name}
	}
}

// stopServerCmd issues a Stop call on the controller and reports completion
// via serverStoppedMsg.
func stopServerCmd(ctrl httpsrv.Controller, name string) tea.Cmd {
	return func() tea.Msg {
		if err := ctrl.Stop(name); err != nil {
			return serverActionErrMsg{name: name, err: err}
		}
		return serverStoppedMsg{name: name}
	}
}

// formatDuration formats d as "Xh Ym Zs" omitting leading zero segments.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
