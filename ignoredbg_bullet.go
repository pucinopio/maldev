//go:build ignore

package main

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func main() {
	fmt.Println("● rune width:", runewidth.RuneWidth('●'))
	fmt.Println("○ rune width:", runewidth.RuneWidth('○'))
	
	mute := lipgloss.NewStyle().Foreground(lipgloss.Color("#4a4a78"))
	bullet := mute.Render("●")
	fmt.Println("lipgloss.Width(●):", lipgloss.Width(bullet))
	
	// Check if BoxStyle padding interacts with Width
	st := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#2a2a52")).
		Padding(0, 1)
	
	line := "hello world this is a test line that is exactly 43 chars long!"
	fmt.Println("test line len:", lipgloss.Width(line))
	rendered := st.Width(43).Render(line)
	lines := len(splitLines(rendered))
	fmt.Println("rendered lines:", lines)
	fmt.Println("rendered:", rendered)
}

func splitLines(s string) []string {
	var lines []string
	cur := ""
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, cur)
			cur = ""
		} else {
			cur += string(c)
		}
	}
	lines = append(lines, cur)
	return lines
}
