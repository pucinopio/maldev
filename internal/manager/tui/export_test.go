// export_test.go exposes internal types and constructors for black-box tests
// in package tui_test. This file is only compiled during `go test`.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
)

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
	return newServersModel(ctrl)
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
