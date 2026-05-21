//go:build ignore

package main

import (
	"fmt"
	"strings"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/oioio-space/maldev/internal/manager/tui"
	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
)

func main() {
	for _, sz := range []struct{w,h int}{{120,40},{144,44}} {
		var m tea.Model = tui.New(nil, nil, tui.SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
		// inject seed data so dashboard exits loading state
		m, _ = m.Update(cmds.DashboardSnapshotMsg{Active: 5})
		view := m.View()
		lines := strings.Split(view, "\n")
		fmt.Printf("%dx%d with data: %d lines (want %d)\n", sz.w, sz.h, len(lines), sz.h)
	}
}
