package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
)

// TestDashboardCacheReused asserts that widgetTree() returns the same Widget
// instance on back-to-back calls (cache hit) and a fresh one after an
// invalidating Update (resize / snapshot).
func TestDashboardCacheReused(t *testing.T) {
	m := newDashboardModel(nil, nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})

	first := m.widgetTree()
	second := m.widgetTree()
	if first != second {
		t.Errorf("widgetTree() should return cached instance on second call; got fresh widget")
	}

	// Snapshot Update must invalidate.
	m, _ = m.Update(cmds.DashboardSnapshotMsg{Active: 3})
	third := m.widgetTree()
	if third == second {
		t.Errorf("widgetTree() should rebuild after DashboardSnapshotMsg; got cached instance")
	}

	// Resize must invalidate too.
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	fourth := m.widgetTree()
	if fourth == third {
		t.Errorf("widgetTree() should rebuild after WindowSizeMsg; got cached instance")
	}
}
