//go:build ignore

package main

import (
	"fmt"
	"strings"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	colW := 43
	glowCyan := lipgloss.NewStyle().Foreground(lipgloss.Color("#00f0ff")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#7a7ab8"))
	mute := lipgloss.NewStyle().Foreground(lipgloss.Color("#4a4a78"))
	serverPillOn := lipgloss.NewStyle().Foreground(lipgloss.Color("#39ff88")).Bold(true)

	bullet := mute.Render("●")
	tag := serverPillOn.Render("ON")
	nameAddr := glowCyan.Render("revocation") + "  " + dim.Render(":8443")
	prefixW := lipgloss.Width(bullet) + 1 + lipgloss.Width(nameAddr)
	tagW := lipgloss.Width(tag)
	gap := colW - prefixW - tagW
	line1 := bullet + " " + nameAddr + strings.Repeat(" ", gap) + tag
	line2 := "  " + mute.Render("https://manager.local:8443 · 142 req")
	content := line1 + "\n" + line2

	fmt.Printf("line1 width: %d\n", lipgloss.Width(line1))
	fmt.Printf("line2 width: %d\n", lipgloss.Width(line2))

	st := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#2a2a52")).
		Padding(0, 1)
	
	rendered := st.Width(colW).Render(content)
	for i, l := range strings.Split(rendered, "\n") {
		fmt.Printf("line %d (visual %d): %q\n", i, lipgloss.Width(l), l)
	}
}
