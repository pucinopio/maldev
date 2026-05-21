//go:build ignore

package main

import (
	"fmt"
	"strings"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	// Test: Height() behavior when content has MORE lines than H
	content := strings.Repeat("line\n", 10) // 10 lines
	rendered := lipgloss.NewStyle().Width(20).Height(5).Render(content)
	lines := strings.Split(rendered, "\n")
	fmt.Printf("Height(5) with 10-line content: %d output lines\n", len(lines))
	for i, l := range lines {
		fmt.Printf("  %d: %q\n", i, l)
	}
}
