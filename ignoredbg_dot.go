//go:build ignore

package main

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func main() {
	fmt.Println("· (U+00B7) width:", runewidth.RuneWidth('·'))
	fmt.Println("╌ (U+254C) width:", runewidth.RuneWidth('╌'))
	
	url := "https://manager.local:8443 · 142 req"
	fmt.Println("url lipgloss.Width:", lipgloss.Width(url))
	fmt.Println("url len:", len([]rune(url)))
}
