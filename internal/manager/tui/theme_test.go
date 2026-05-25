package tui

import "testing"

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
