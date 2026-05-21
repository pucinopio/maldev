//go:build ignore

package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func main() {
	colW := (144-1)/3 - 4
	fmt.Println("colW:", colW)

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
	fmt.Println("prefixW:", prefixW, "tagW:", tagW, "gap:", gap)

	line1 := bullet + " " + nameAddr + strings.Repeat(" ", gap) + tag
	fmt.Println("line1 Width:", lipgloss.Width(line1))

	line2 := "  " + mute.Render("https://manager.local:8443 · 142 req")
	fmt.Println("line2 Width:", lipgloss.Width(line2))
	fmt.Println("URL raw len:", lipgloss.Width("https://manager.local:8443 · 142 req"))
}
