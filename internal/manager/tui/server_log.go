package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/tui/core"
	"github.com/oioio-space/maldev/internal/manager/tui/widgets"
)

// serverEventMsg wraps an httpsrv.Event so bubbletea routes it through Update.
type serverEventMsg struct{ ev httpsrv.Event }

// serverLogClearMsg tells the log widget to discard its entries.
type serverLogClearMsg struct{}

// serverLogAutoScrollMsg toggles tail-follow (stick-to-bottom) on the log.
type serverLogAutoScrollMsg struct{}

// serverLogFilterMsg sets the active server name filter ("" = all).
type serverLogFilterMsg struct{ server string }

// serverLog is a scrollable live event log for the Servers screen.
// It maintains a capped ring slice (maxLogEntries) and renders via
// widgets.WrappedViewport. Auto-scroll is suspended while the operator
// has scrolled above the last line.
type serverLog struct {
	entries    []httpsrv.Event // ring — appended, capped at maxLogEntries
	filter     string          // "" means show all servers
	autoScroll bool

	vp     *widgets.WrappedViewport
	bounds core.Rect
}

const maxLogEntries = 500

func newServerLog() *serverLog {
	return &serverLog{
		autoScroll: true,
		vp:         widgets.NewWrappedViewport(""),
	}
}

func (sl *serverLog) Layout(bounds core.Rect) {
	sl.bounds = bounds
	sl.vp.Layout(bounds)
}

func (sl *serverLog) Bounds() core.Rect { return sl.bounds }

func (sl *serverLog) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	switch m := msg.(type) {
	case serverEventMsg:
		sl.append(m.ev)
		if sl.autoScroll {
			sl.vp.SetContent(sl.render())
		}
		return sl, nil

	case serverLogClearMsg:
		sl.entries = sl.entries[:0]
		sl.vp.SetContent("")
		return sl, nil

	case serverLogFilterMsg:
		sl.filter = m.server
		sl.vp.SetContent(sl.render())
		return sl, nil

	case serverLogAutoScrollMsg:
		sl.autoScroll = !sl.autoScroll
		if sl.autoScroll {
			// Re-rendering snaps the viewport to the latest content; the
			// WrappedViewport keeps its scroll at the bottom when content
			// grows, so this is enough to resume tail-follow.
			sl.vp.SetContent(sl.render())
		}
		return sl, nil

	case tea.MouseMsg:
		// Detect upward scroll to pause auto-scroll; wheel-down at bottom re-enables.
		if m.Action == tea.MouseActionPress {
			switch m.Button {
			case tea.MouseButtonWheelUp:
				sl.autoScroll = false
			case tea.MouseButtonWheelDown:
				// Re-enable once the user scrolls back to the bottom.
				// WrappedViewport does the actual scrolling; we rely on it being
				// at the last line as a signal.
				sl.autoScroll = true
			}
		}
	}

	updated, cmd := sl.vp.Update(msg)
	sl.vp, _ = updated.(*widgets.WrappedViewport)
	return sl, cmd
}

func (sl *serverLog) View() string {
	// Late-seed: the empty-state hint must show even before any event arrives.
	// SetContent is idempotent so re-emitting it here is safe.
	if len(sl.entries) == 0 {
		sl.vp.SetContent(sl.render())
	}
	return sl.vp.View()
}

// append adds ev to the ring slice, evicting the oldest entry when full.
func (sl *serverLog) append(ev httpsrv.Event) {
	if len(sl.entries) >= maxLogEntries {
		// Shift left by one — O(n) but n≤500 and the log is not in the hot path.
		copy(sl.entries, sl.entries[1:])
		sl.entries[len(sl.entries)-1] = ev
	} else {
		sl.entries = append(sl.entries, ev)
	}
}

// render builds the full viewport content string respecting the active filter.
func (sl *serverLog) render() string {
	var sb strings.Builder
	matched := 0
	for _, e := range sl.entries {
		if sl.filter != "" && e.Server != sl.filter {
			continue
		}
		sb.WriteString(renderLogLine(e))
		sb.WriteByte('\n')
		matched++
	}
	if matched == 0 {
		hint := "aucun évènement — démarre un serveur pour voir le trafic ici."
		if sl.filter != "" {
			hint = fmt.Sprintf("aucun évènement pour %q — démarre ce serveur ou change de sous-onglet.", sl.filter)
		}
		sb.WriteString(Mute.Render(hint))
	}
	return sb.String()
}

// renderLogLine formats a single event line with Kind-based colouring.
func renderLogLine(e httpsrv.Event) string {
	ts := e.At.Format("15:04:05")
	srv := fmt.Sprintf("%-12s", e.Server)

	var kindStyle lipgloss.Style
	switch e.Kind {
	case "started":
		kindStyle = GlowGreen
	case "stopped":
		kindStyle = GlowYellow
	case "error":
		kindStyle = GlowRed
	default: // "request"
		kindStyle = Dim
	}

	kind := kindStyle.Render(fmt.Sprintf("%-8s", e.Kind))

	detail := e.Note
	if e.Kind == "request" {
		status := ""
		if e.Status != 0 {
			st := lipgloss.NewStyle()
			switch {
			case e.Status >= 500:
				st = st.Foreground(Palette.Red)
			case e.Status >= 400:
				st = st.Foreground(Palette.Yellow)
			default:
				st = st.Foreground(Palette.Green)
			}
			status = st.Render(fmt.Sprintf("%d", e.Status))
		}
		detail = fmt.Sprintf("%s %s %s %s",
			Mute.Render(e.Method), Base.Render(e.Path), status, Mute.Render(e.Remote))
	}

	return fmt.Sprintf("%s  %s  %s  %s",
		Mute.Render(ts), Dim.Render(srv), kind, detail)
}

// filterChips builds a one-line chip row for the three server names.
// The active chip is highlighted; clicking fires serverLogFilterMsg.
// This returns a plain string (rendered directly in the screen's View).
func filterChips(active string) string {
	names := []string{"all", "revocation", "heartbeat", "probe"}
	parts := make([]string, len(names))
	for i, n := range names {
		key := n
		if key == "all" {
			key = ""
		}
		if key == active {
			parts[i] = PillActive.Render(" " + n + " ")
		} else {
			parts[i] = PillOff.Render(" " + n + " ")
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
