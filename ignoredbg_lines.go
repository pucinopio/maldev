//go:build ignore

package main

import (
	"fmt"
	"strings"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/oioio-space/maldev/internal/manager/tui"
)

func main() {
	for _, sz := range []struct{w,h int}{{120,40},{144,44},{200,60}} {
		var m tea.Model = tui.New(nil, nil, tui.SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
		view := m.View()
		lines := strings.Split(view, "\n")
		fmt.Printf("%dx%d: %d lines (want %d)\n", sz.w, sz.h, len(lines), sz.h)
		if len(lines) != sz.h {
			for i, l := range lines {
				if i < 5 || i >= len(lines)-5 {
					fmt.Printf("  line %d: %q\n", i, l[:min(len(l),40)])
				}
			}
		}
	}
}

func min(a, b int) int {
	if a < b { return a }
	return b
}
