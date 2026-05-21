package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

func init() {
	// Populate core.Colors so that widgets/ can use palette values without
	// importing tui (which would create a cycle).
	core.Colors.Bg1 = "#0a0a18"
	core.Colors.Border = "#2a2a52"
	core.Colors.BorderBright = "#4a4aa0"
	core.Colors.Fg = "#e6e6ff"
	core.Colors.FgDim = "#7a7ab8"
	core.Colors.FgMute = "#4a4a78"
	core.Colors.Magenta = "#ff36d4"
	core.Colors.Green = "#39ff88"
	core.Colors.Yellow = "#ffce39"
	core.Colors.Red = "#ff3c5f"
}

var Palette = struct {
	Bg, Bg1, Bg2, Bg3                                                    lipgloss.Color
	Border, BorderBright                                                  lipgloss.Color
	Fg, FgDim, FgMute                                                     lipgloss.Color
	Cyan, Magenta, Green, Yellow, Orange, Red, Violet lipgloss.Color
}{
	Bg: "#05050d", Bg1: "#0a0a18", Bg2: "#10102a", Bg3: "#16163a",
	Border: "#2a2a52", BorderBright: "#4a4aa0",
	Fg: "#e6e6ff", FgDim: "#7a7ab8", FgMute: "#4a4a78",
	Cyan: "#00f0ff", Magenta: "#ff36d4", Green: "#39ff88",
	Yellow: "#ffce39", Orange: "#ff8a3c", Red: "#ff3c5f", Violet: "#a070ff",
}

var (
	Base       = lipgloss.NewStyle().Foreground(Palette.Fg)
	Dim        = lipgloss.NewStyle().Foreground(Palette.FgDim)
	Mute       = lipgloss.NewStyle().Foreground(Palette.FgMute)
	GlowCyan   = lipgloss.NewStyle().Foreground(Palette.Cyan).Bold(true)
	GlowMagent = lipgloss.NewStyle().Foreground(Palette.Magenta).Bold(true)
	GlowGreen  = lipgloss.NewStyle().Foreground(Palette.Green).Bold(true)
	GlowRed    = lipgloss.NewStyle().Foreground(Palette.Red).Bold(true)
	GlowYellow = lipgloss.NewStyle().Foreground(Palette.Yellow).Bold(true)

	// BorderBright is used for box edges so the grid is legible on dark
	// terminals — Palette.Border is intentionally dim and only meant for
	// inner separators / inactive chips.
	BoxStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(Palette.BorderBright).Padding(0, 1)
	BoxFocused = BoxStyle.Border(lipgloss.NormalBorder()).BorderForeground(Palette.Magenta)

	TabActive = lipgloss.NewStyle().Foreground(Palette.Fg).Bold(true).Padding(0, 2).
			Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(Palette.Magenta)
	TabInactive = lipgloss.NewStyle().Foreground(Palette.FgDim).Padding(0, 2)

	PillActive     = lipgloss.NewStyle().Foreground(Palette.Green).Bold(true).Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(Palette.Green)
	PillExpiring   = lipgloss.NewStyle().Foreground(Palette.Yellow).Bold(true).Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(Palette.Yellow)
	PillRevoked    = lipgloss.NewStyle().Foreground(Palette.Red).Bold(true).Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(Palette.Red)
	PillSuperseded = lipgloss.NewStyle().Foreground(Palette.Violet).Bold(true).Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(Palette.Violet)
	PillOn         = PillActive
	PillOff        = lipgloss.NewStyle().Foreground(Palette.FgMute).Bold(true).Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(Palette.FgMute)

	Modal       = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(Palette.Magenta).Padding(1, 2)
	ModalDanger = Modal.BorderForeground(Palette.Red)
	ModalOK     = Modal.BorderForeground(Palette.Green)

	HintKey  = lipgloss.NewStyle().Foreground(Palette.Magenta).Bold(true).Padding(0, 1)
	HintText = lipgloss.NewStyle().Foreground(Palette.FgDim)
)
