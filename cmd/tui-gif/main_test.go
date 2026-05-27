package main

import (
	"image/gif"
	"os"
	"path/filepath"
	"testing"
)

// TestTuiGif_EndToEnd writes a tiny inline tape, invokes the parser +
// driver, and decodes the resulting GIF. Guards the encoder against
// regressions on every CI run — needs neither ttyd, ffmpeg, nor a display.
func TestTuiGif_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	tapePath := filepath.Join(dir, "demo.tape")
	gifPath := filepath.Join(dir, "out.gif")

	tape := `Width 80
Height 24
Delay 50ms
Output ` + gifPath + `
Key 2
Sleep 100ms
Down
Down
Key P
Sleep 100ms
Key j
Key j
Key g
`
	if err := os.WriteFile(tapePath, []byte(tape), 0o600); err != nil {
		t.Fatal(err)
	}

	// Parse + run the same way main() does, sans CLI plumbing.
	parsed, err := parseTape(tapePath)
	if err != nil {
		t.Fatalf("parseTape: %v", err)
	}
	if parsed.output != gifPath {
		t.Errorf("output path=%q want %q", parsed.output, gifPath)
	}
	if parsed.w != 80 || parsed.h != 24 {
		t.Errorf("dims=(%d,%d) want (80,24)", parsed.w, parsed.h)
	}
	if len(parsed.frames) == 0 {
		t.Fatal("tape parsed zero steps")
	}

	// Drive via the public main() by passing the tape on argv.
	os.Args = []string{"tui-gif", tapePath}
	main()

	f, err := os.Open(gifPath)
	if err != nil {
		t.Fatalf("open gif: %v", err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatalf("decode gif: %v", err)
	}
	if len(g.Image) < 2 {
		t.Errorf("gif has %d frames, want at least 2", len(g.Image))
	}
	if g.LoopCount != 0 {
		t.Errorf("loop count = %d, want 0 (infinite)", g.LoopCount)
	}
}

// TestFrameToCells_StripsControlSequences ensures non-SGR ANSI sequences
// (cursor moves, screen clears) are dropped while SGR colour codes are
// honored. Without this guard the rendered GIF would contain garbage when
// the TUI emits cursor positioning between styled segments.
func TestFrameToCells_StripsControlSequences(t *testing.T) {
	input := "\x1b[2J\x1b[H\x1b[31mred\x1b[0m\x1b[0;0H\x1b[32mgreen\x1b[0m"
	grid := frameToCells(input, 10, 1)
	got := ""
	for _, c := range grid[0] {
		got += string(c.r)
	}
	want := "redgreen  "
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	// Verify that the 'r' is rendered with the red palette entry and 'g'
	// with the green one.
	if grid[0][0].fg != xtermBasePalette[1] {
		t.Errorf("'r' fg = %v, want xterm red %v", grid[0][0].fg, xtermBasePalette[1])
	}
	if grid[0][3].fg != xtermBasePalette[2] {
		t.Errorf("'g' fg = %v, want xterm green %v", grid[0][3].fg, xtermBasePalette[2])
	}
}
