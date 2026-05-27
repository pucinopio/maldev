package main

import (
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestDumpFrames is a manual helper — `go test -run TestDumpFrames -v` extracts
// each frame of a tape's GIF as a PNG so the operator can inspect the visual
// output of the PEM tab / detail panel etc. Skipped unless TUI_GIF_DUMP is set.
func TestDumpFrames(t *testing.T) {
	src := os.Getenv("TUI_GIF_DUMP")
	if src == "" {
		t.Skip("set TUI_GIF_DUMP=path/to/file.gif to enable")
	}
	f, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatal(err)
	}
	dir := src + ".frames"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i, frame := range g.Image {
		path := filepath.Join(dir, "frame-"+strconv.Itoa(i)+".png")
		out, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(out, frame); err != nil {
			out.Close()
			t.Fatal(err)
		}
		out.Close()
	}
	t.Logf("wrote %d frames → %s", len(g.Image), dir)
}
