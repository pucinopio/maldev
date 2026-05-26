// export_test.go exposes internal types and constructors for black-box tests
// in package tui_test. This file is only compiled during `go test`.
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
)

// SetTitleBarClock replaces the clock used by renderTitleBar with fn and
// returns a restore func. Call it in TestMain or t.Cleanup to get deterministic
// golden output. Example:
//
//	restore := tui.SetTitleBarClock(func() time.Time { return fixedTime })
//	t.Cleanup(restore)
func SetTitleBarClock(fn func() time.Time) (restore func()) {
	prev := titleBarClock
	titleBarClock = fn
	return func() { titleBarClock = prev }
}

// ServerEventMsg wraps an httpsrv.Event as the internal serverEventMsg so
// tests can inject events without importing the unexported type.
func ServerEventMsg(ev httpsrv.Event) tea.Msg {
	return serverEventMsg{ev: ev}
}

// ServerStartMsg wraps name as the internal serverStartMsg.
func ServerStartMsg(name string) tea.Msg { return serverStartMsg{name: name} }

// ServerStopMsg wraps name as the internal serverStopMsg.
func ServerStopMsg(name string) tea.Msg { return serverStopMsg{name: name} }

// NewServersModelForTest constructs a serversModel backed by the given
// Controller so tests can exercise start/stop routing without a real Bundle.
func NewServersModelForTest(ctrl httpsrv.Controller) serversModel {
	return newServersModel(nil, ctrl)
}

// InitServersModel applies a WindowSizeMsg to m and returns the updated model.
func InitServersModel(m serversModel, width, height int) serversModel {
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return updated
}

// UpdateServersModel drives m through a single message, executes the returned
// Cmd synchronously (one level deep), and returns the final model.
// This mirrors how bubbletea's runtime works: Update returns a Cmd, the
// runtime executes it and feeds the resulting Msg back.
func UpdateServersModel(m serversModel, msg tea.Msg) (serversModel, tea.Cmd) {
	updated, cmd := m.Update(msg)
	if cmd != nil {
		// Execute the Cmd to trigger any controller calls (e.g. Start/Stop).
		resultMsg := cmd()
		if resultMsg != nil {
			updated, cmd = updated.Update(resultMsg)
		}
	}
	return updated, cmd
}

// PushOverlayMsgForTest is the exported shape of pushOverlayMsg so black-box
// tests can type-assert the message returned by screen Cmds.
type PushOverlayMsgForTest struct{ Overlay Overlay }

// AsPushOverlay converts a raw tea.Msg to PushOverlayMsgForTest if it is a
// pushOverlayMsg, returning (msg, true) on success.
func AsPushOverlay(msg tea.Msg) (PushOverlayMsgForTest, bool) {
	if p, ok := msg.(pushOverlayMsg); ok {
		return PushOverlayMsgForTest{Overlay: p.overlay}, true
	}
	return PushOverlayMsgForTest{}, false
}

// NewSelectOverlayForTest exposes newSelectOverlay for black-box tests.
func NewSelectOverlayForTest(id, title string, options []SelectOption, initial string) Overlay {
	return newSelectOverlay(id, title, options, initial)
}

// SelectOptionForTest re-exports SelectOption so tests can construct values
// without importing the internal type via a type alias.
type SelectOptionForTest = SelectOption

// IPOptionsForTest exposes ipOptions for guard tests.
func IPOptionsForTest() []SelectOption { return ipOptions() }

// ServerTabWidthsForTest exposes the precomputed serverTabWidths array.
func ServerTabWidthsForTest() [3]int { return serverTabWidths }

// ServerRoleCacheForTest returns the serverRoleCache from a serversModel.
func ServerRoleCacheForTest(m serversModel) [3]string { return m.serverRoleCache }
