package tui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestThemeSeparationRule scans the package for inline
// `lipgloss.NewStyle().Foreground(Palette.X)` patterns outside theme.go.
// Each match is a violation of the theme-separation rule documented at
// the top of theme.go — palette access must go through a named style var
// so ApplyTheme() can swap the palette without leaving stale colours.
func TestThemeSeparationRule(t *testing.T) {
	// Catches any chain that starts with `lipgloss.NewStyle()` and ends
	// with `.Foreground(Palette.X)` anywhere in between, e.g.:
	//   lipgloss.NewStyle().Width(8).Foreground(Palette.Yellow).Render(x)
	// Pass-2 used a tighter regex that only caught the FIRST method call;
	// the agent audit (pass-3 quality) found 3 sites that slipped through.
	pattern := regexp.MustCompile(`lipgloss\.NewStyle\(\)[^/\n]*\.Foreground\(Palette\.`)
	root := "."
	violations := []string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "theme.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if pattern.MatchString(line) {
				violations = append(violations, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("theme-separation rule violated — use a named style from theme.go instead of building one inline:\n%s",
			strings.Join(violations, "\n"))
	}
}

// TestApplyTheme_ReseedsAllGlowStyles asserts that every Glow* style
// vars picks up the new palette colour after ApplyTheme. Without this,
// adding a new Glow* var to theme.go and forgetting to wire it into
// reseedStyles() would go silently undetected (the var would keep
// emitting its boot-time palette colour).
func TestApplyTheme_ReseedsAllGlowStyles(t *testing.T) {
	defer ApplyTheme("neon")

	check := func(name string, want struct {
		Cyan, Magenta, Green, Red, Yellow, Violet lipgloss.Color
	}) {
		t.Helper()
		ApplyTheme(name)
		assert := func(field string, got lipgloss.TerminalColor, want lipgloss.Color) {
			if got != want {
				t.Errorf("ApplyTheme(%q): Glow%s foreground = %v, want %v (reseedStyles missing this var?)", name, field, got, want)
			}
		}
		assert("Cyan", GlowCyan.GetForeground(), want.Cyan)
		assert("Magent", GlowMagent.GetForeground(), want.Magenta)
		assert("Green", GlowGreen.GetForeground(), want.Green)
		assert("Red", GlowRed.GetForeground(), want.Red)
		assert("Yellow", GlowYellow.GetForeground(), want.Yellow)
		assert("Violet", GlowViolet.GetForeground(), want.Violet)
	}

	check("mono", struct {
		Cyan, Magenta, Green, Red, Yellow, Violet lipgloss.Color
	}{paletteMono.Cyan, paletteMono.Magenta, paletteMono.Green, paletteMono.Red, paletteMono.Yellow, paletteMono.Violet})
	check("nord-soft", struct {
		Cyan, Magenta, Green, Red, Yellow, Violet lipgloss.Color
	}{paletteNordSoft.Cyan, paletteNordSoft.Magenta, paletteNordSoft.Green, paletteNordSoft.Red, paletteNordSoft.Yellow, paletteNordSoft.Violet})
}

// TestApplyTheme_SwapsPaletteAndStyles checks that ApplyTheme mutates the
// active Palette and that the dependent style vars (Base/Dim) pick up the
// new foreground without a restart. Restores the default neon palette in
// defer so other tests see the standard colours.
func TestApplyTheme_SwapsPaletteAndStyles(t *testing.T) {
	defer ApplyTheme("neon")

	ApplyTheme("mono")
	if Palette.Fg != paletteMono.Fg {
		t.Errorf("Palette.Fg = %q after ApplyTheme(\"mono\"), want %q", Palette.Fg, paletteMono.Fg)
	}
	if got := Base.GetForeground(); got != paletteMono.Fg {
		t.Errorf("Base foreground = %q after mono theme, want %q (style reseed failed)", got, paletteMono.Fg)
	}

	ApplyTheme("nord-soft")
	if Palette.Magenta != paletteNordSoft.Magenta {
		t.Errorf("Palette.Magenta = %q after ApplyTheme(\"nord-soft\"), want %q", Palette.Magenta, paletteNordSoft.Magenta)
	}

	ApplyTheme("unknown-theme-name")
	if Palette.Fg != paletteNeon.Fg {
		t.Errorf("unknown theme should fall back to neon, got Fg = %q", Palette.Fg)
	}
}
