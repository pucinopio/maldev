//go:build ignore

package main

import (
	"fmt"
	"strings"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	// Verify: Width(43) with Padding(0,1) — what is the actual content area?
	st := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#2a2a52")).
		Padding(0, 1)
	
	// Fill with 'x' to find real content width
	for w := 38; w <= 45; w++ {
		content := strings.Repeat("x", w)
		rendered := st.Width(43).Render(content)
		lines := strings.Split(rendered, "\n")
		fmt.Printf("content len %d → %d render lines (content line: %q)\n", w, len(lines), lines[1])
	}
}
