// Command tui-click-sweep exhaustively clicks every cell of every view in
// license-manager's TUI and reports which clicks produced a state change
// (a non-zero diff vs the unclicked baseline). Use it to find dead zones,
// off-by-one hit boxes, and unintentional wiring.
//
// Usage:
//
//	go run ./cmd/tui-click-sweep [-width 160] [-height 50] [-view dashboard,licenses,...]
//
// Output: per-view summary table — rows containing at least one cell that
// triggered a SwitchViewMsg / filter change / overlay push.
package main

import (
	"flag"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/oioio-space/maldev/internal/manager/tui"
)

func init() {
	// Force true-color rendering so chip state changes (active=magenta vs
	// inactive=dim) produce distinct ANSI strings. Without this, lipgloss
	// auto-detects no TTY and strips colors, making the click sweep blind
	// to filter-chip selection changes.
	lipgloss.SetColorProfile(termenv.TrueColor)
}

var (
	widthFlag  = flag.Int("width", 160, "terminal width")
	heightFlag = flag.Int("height", 50, "terminal height")
	viewFlag   = flag.String("view", "", "comma-separated views to sweep (default: all)")
)

type cell struct {
	x, y int
	hit  bool
}

func main() {
	flag.Parse()

	allViews := []tui.ViewID{
		tui.ViewDashboard, tui.ViewLicenses, tui.ViewIssuers,
		tui.ViewRecipients, tui.ViewIdentities, tui.ViewRevocation,
		tui.ViewServers, tui.ViewAudit, tui.ViewSettings,
	}

	views := allViews
	if *viewFlag != "" {
		views = nil
		want := strings.Split(*viewFlag, ",")
		for _, w := range want {
			views = append(views, tui.ViewID(strings.TrimSpace(w)))
		}
	}

	for _, v := range views {
		runSweep(v, *widthFlag, *heightFlag)
	}
}

func runSweep(view tui.ViewID, w, h int) {
	fmt.Printf("\n=== %s (%dx%d) ===\n", view, w, h)

	// Build a fresh root model anchored at the target view.
	mk := func() tea.Model {
		var m tea.Model = tui.New(nil, nil, tui.SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
		// Switch via digit keypress so the model picks up Init() etc.
		digit := digitForView(view)
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{digit}})
		return m
	}

	// Compare content rows only (skip title bar — it carries a live clock).
	strip := func(s string) string {
		lines := strings.Split(s, "\n")
		if len(lines) > 1 {
			lines = lines[1:]
		}
		return strings.Join(lines, "\n")
	}
	baseline := strip(mk().View())
	hits := make([]cell, 0, w*h)
	for y := 0; y < h-1; y++ { // exclude status bar row
		for x := 0; x < w; x++ {
			m := mk()
			updated, cmd := m.Update(tea.MouseMsg{
				X: x, Y: y,
				Button: tea.MouseButtonLeft,
				Action: tea.MouseActionPress,
			})
			if cmd != nil {
				if msg := cmd(); msg != nil {
					updated, _ = updated.Update(msg)
				}
			}
			after := strip(updated.View())
			if after != baseline {
				hits = append(hits, cell{x: x, y: y, hit: true})
			}
		}
	}

	// Group by Y row → print one summary line per row with non-empty hits.
	type span struct{ y, xStart, xEnd int }
	var spans []span
	var cur *span
	for _, c := range hits {
		if cur != nil && cur.y == c.y && c.x == cur.xEnd+1 {
			cur.xEnd = c.x
			continue
		}
		if cur != nil {
			spans = append(spans, *cur)
		}
		cur = &span{y: c.y, xStart: c.x, xEnd: c.x}
	}
	if cur != nil {
		spans = append(spans, *cur)
	}

	if len(spans) == 0 {
		fmt.Printf("  (no cells produced a state change)\n")
		return
	}
	for _, s := range spans {
		fmt.Printf("  Y=%2d  X=%3d..%3d  (%d cells)\n", s.y, s.xStart, s.xEnd, s.xEnd-s.xStart+1)
	}
	fmt.Printf("  total %d cells trigger a change\n", len(hits))
}

func digitForView(v tui.ViewID) rune {
	switch v {
	case tui.ViewDashboard:
		return '1'
	case tui.ViewLicenses:
		return '2'
	case tui.ViewIssuers:
		return '3'
	case tui.ViewRecipients:
		return '4'
	case tui.ViewIdentities:
		return '5'
	case tui.ViewRevocation:
		return '6'
	case tui.ViewServers:
		return '7'
	case tui.ViewAudit:
		return '8'
	case tui.ViewSettings:
		return '9'
	}
	return '1'
}
