package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
)

// BenchmarkDashboardView_Cached measures the steady-state cost of rendering
// the dashboard after the widget tree cache has been primed. The win from
// caching (pass-2 commit 0a5a876) should make this dominated by string
// concat rather than widget construction.
func BenchmarkDashboardView_Cached(b *testing.B) {
	root := New(nil, nil, SessionReady)
	root.active = ViewDashboard
	var m tea.Model = root
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m, _ = m.Update(cmds.DashboardSnapshotMsg{Active: 12, Revoked: 3})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

// BenchmarkDashboardView_Invalidating mirrors the worst case: every frame
// invalidates the cache (e.g. a window-resize storm). This is the cost the
// cache saves us from on each tick of serverStatusTickMsg.
func BenchmarkDashboardView_Invalidating(b *testing.B) {
	root := New(nil, nil, SessionReady)
	root.active = ViewDashboard
	var m tea.Model = root
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
		_ = m.View()
	}
}

// BenchmarkTitleHintsHit microbenchmarks hit-testing on the title-bar
// helper, the hottest path inside any list screen's OnClick.
func BenchmarkTitleHintsHit(b *testing.B) {
	var row titleHintRow
	_ = titleBar(&row, "My Title", []titleHint{
		{Key: "n", Label: " new", Cmd: keyCmd("n")},
		{Key: "x", Label: " delete", Cmd: keyCmd("x")},
		{Key: "E", Label: " export", Cmd: keyCmd("E")},
		{Key: "r", Label: " refresh", Cmd: keyCmd("r")},
	}, 0, 120)
	row.SetY(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = row.hit(50, 10)
	}
}
