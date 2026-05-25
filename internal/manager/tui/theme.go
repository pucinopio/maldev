package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// paletteData is the colour set a Theme resolves to. Three palettes ship
// today: neon (default), mono, nord-soft. ApplyTheme swaps the active one.
type paletteData struct {
	Bg, Bg1, Bg2, Bg3 lipgloss.Color
	Border, BorderBright,
	Fg, FgDim, FgMute,
	Cyan, Magenta, Green, Yellow, Orange, Red, Violet lipgloss.Color
}

var (
	paletteNeon = paletteData{
		Bg: "#05050d", Bg1: "#0a0a18", Bg2: "#10102a", Bg3: "#16163a",
		Border: "#2a2a52", BorderBright: "#4a4aa0",
		Fg: "#e6e6ff", FgDim: "#7a7ab8", FgMute: "#4a4a78",
		Cyan: "#00f0ff", Magenta: "#ff36d4", Green: "#39ff88",
		Yellow: "#ffce39", Orange: "#ff8a3c", Red: "#ff3c5f", Violet: "#a070ff",
	}
	paletteMono = paletteData{
		Bg: "#000000", Bg1: "#0d0d0d", Bg2: "#1a1a1a", Bg3: "#262626",
		Border: "#333333", BorderBright: "#666666",
		Fg: "#f5f5f5", FgDim: "#a0a0a0", FgMute: "#606060",
		Cyan: "#cccccc", Magenta: "#ffffff", Green: "#bbbbbb",
		Yellow: "#dddddd", Orange: "#aaaaaa", Red: "#ffffff", Violet: "#999999",
	}
	paletteNordSoft = paletteData{
		Bg: "#2e3440", Bg1: "#3b4252", Bg2: "#434c5e", Bg3: "#4c566a",
		Border: "#4c566a", BorderBright: "#81a1c1",
		Fg: "#eceff4", FgDim: "#d8dee9", FgMute: "#7a869a",
		Cyan: "#88c0d0", Magenta: "#b48ead", Green: "#a3be8c",
		Yellow: "#ebcb8b", Orange: "#d08770", Red: "#bf616a", Violet: "#b48ead",
	}
)

// Palette is the active palette. Read by every screen via Palette.Xxx. Mutated
// only by ApplyTheme; treat as read-only from caller code.
var Palette = paletteNeon

// Style vars below are recomputed from Palette by reseedStyles(). They are
// not const because ApplyTheme swaps them when the operator picks a theme
// in Settings. lipgloss.Style is a value type — reassignment is cheap and
// safe; new renders pick up the new values immediately.
var (
	Base       lipgloss.Style
	Dim        lipgloss.Style
	Mute       lipgloss.Style
	GlowCyan   lipgloss.Style
	GlowMagent lipgloss.Style
	GlowGreen  lipgloss.Style
	GlowRed    lipgloss.Style
	GlowYellow lipgloss.Style

	// BorderBright is used for box edges so the grid is legible on dark
	// terminals — Palette.Border is intentionally dim and only meant for
	// inner separators / inactive chips.
	BoxStyle   lipgloss.Style
	BoxFocused lipgloss.Style

	TabActive   lipgloss.Style
	TabInactive lipgloss.Style

	PillActive     lipgloss.Style
	PillExpiring   lipgloss.Style
	PillRevoked    lipgloss.Style
	PillSuperseded lipgloss.Style
	PillOn         lipgloss.Style
	PillOff        lipgloss.Style

	Modal       lipgloss.Style
	ModalDanger lipgloss.Style
	ModalOK     lipgloss.Style

	HintKey  lipgloss.Style
	HintText lipgloss.Style
)

func init() {
	reseedStyles()
	reseedCoreColors()
}

// ApplyTheme switches the active palette and rebuilds every style var so a
// subsequent Render call uses the new colours. Call from settingsSetThemeMsg
// and at boot once the persisted theme has been resolved. The known names
// are "neon" (default), "mono", "nord-soft". Unknown names fall back to neon.
//
// Limitation: styles cached by widgets/ (sync.OnceValue in statusbar.go,
// tabbar.go, tile.go) are NOT rebuilt because they snapshot core.Colors at
// first use. They will reflect the new theme only after a restart.
func ApplyTheme(name string) {
	switch name {
	case "mono":
		Palette = paletteMono
	case "nord-soft":
		Palette = paletteNordSoft
	default:
		Palette = paletteNeon
	}
	reseedStyles()
	reseedCoreColors()
}

func reseedStyles() {
	Base = lipgloss.NewStyle().Foreground(Palette.Fg)
	Dim = lipgloss.NewStyle().Foreground(Palette.FgDim)
	Mute = lipgloss.NewStyle().Foreground(Palette.FgMute)
	GlowCyan = lipgloss.NewStyle().Foreground(Palette.Cyan).Bold(true)
	GlowMagent = lipgloss.NewStyle().Foreground(Palette.Magenta).Bold(true)
	GlowGreen = lipgloss.NewStyle().Foreground(Palette.Green).Bold(true)
	GlowRed = lipgloss.NewStyle().Foreground(Palette.Red).Bold(true)
	GlowYellow = lipgloss.NewStyle().Foreground(Palette.Yellow).Bold(true)

	BoxStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(Palette.BorderBright).Padding(0, 1)
	BoxFocused = BoxStyle.BorderForeground(Palette.Magenta)

	TabActive = lipgloss.NewStyle().Foreground(Palette.Fg).Bold(true).Padding(0, 2).
		Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(Palette.Magenta)
	TabInactive = lipgloss.NewStyle().Foreground(Palette.FgDim).Padding(0, 2)

	PillActive = lipgloss.NewStyle().Foreground(Palette.Green).Bold(true).Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(Palette.Green)
	PillExpiring = lipgloss.NewStyle().Foreground(Palette.Yellow).Bold(true).Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(Palette.Yellow)
	PillRevoked = lipgloss.NewStyle().Foreground(Palette.Red).Bold(true).Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(Palette.Red)
	PillSuperseded = lipgloss.NewStyle().Foreground(Palette.Violet).Bold(true).Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(Palette.Violet)
	PillOn = PillActive
	PillOff = lipgloss.NewStyle().Foreground(Palette.FgMute).Bold(true).Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(Palette.FgMute)

	Modal = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(Palette.Magenta).Padding(1, 2)
	ModalDanger = Modal.BorderForeground(Palette.Red)
	ModalOK = Modal.BorderForeground(Palette.Green)

	HintKey = lipgloss.NewStyle().Foreground(Palette.Magenta).Bold(true).Padding(0, 1)
	HintText = lipgloss.NewStyle().Foreground(Palette.FgDim)
}

func reseedCoreColors() {
	core.Colors.Bg1 = Palette.Bg1
	core.Colors.Border = Palette.Border
	core.Colors.BorderBright = Palette.BorderBright
	core.Colors.Fg = Palette.Fg
	core.Colors.FgDim = Palette.FgDim
	core.Colors.FgMute = Palette.FgMute
	core.Colors.Magenta = Palette.Magenta
	core.Colors.Green = Palette.Green
	core.Colors.Yellow = Palette.Yellow
	core.Colors.Red = Palette.Red
}
