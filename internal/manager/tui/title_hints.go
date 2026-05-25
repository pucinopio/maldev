package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// titleHint is one clickable "[key] label" action shown in the hint strip of
// a list-screen box title. Cmd fires when the operator clicks the segment.
type titleHint struct {
	Key   string
	Label string
	Cmd   func() tea.Cmd
}

// titleHintRow records the absolute layout of a hint strip so OnClick can
// hit-test it without re-parsing the rendered string. Screens hold a
// *titleHintRow that titleBar mutates each frame and OnClick consults.
type titleHintRow struct {
	y       int    // absolute Y of the title row (recorded via SetY)
	startX  int    // absolute X of the first hint segment
	segWs   []int  // rendered widths of each hint segment, in order
	hints   []titleHint
}

// titleBar renders a list-screen box title row: cyan label on the left,
// right-aligned "[k] label" hint chips, and populates t with the absolute
// X/Y of each clickable segment.
//
// Arguments:
//   - t          nilable; when non-nil receives the layout for click dispatch
//   - label      the title text rendered with GlowCyan
//   - hints      the clickable action chips, in display order
//   - boxLeftX   absolute terminal X of the box's outer left edge (0 for
//                full-width screens; positive when the box is indented)
//   - boxInnerW  inner width of the box (== BoxedInner(screenWidth))
//
// The caller still has to write `t.SetY(absoluteRowY)` once it knows where
// the title row lands on the screen — usually `3 + leadingBlank + introH +
// trailingBlank + 1` for the standard "intro + boxed table" layout.
func titleBar(t *titleHintRow, label string, hints []titleHint, boxLeftX, boxInnerW int) string {
	leftRendered := GlowCyan.Render(label)
	leftW := lipgloss.Width(leftRendered)

	sep := Mute.Render("· ")
	sepW := lipgloss.Width(sep)
	segs := make([]string, len(hints))
	ws := make([]int, len(hints))
	total := 0
	for i, h := range hints {
		segs[i] = HintKey.Render("["+h.Key+"]") + Dim.Render(h.Label)
		ws[i] = lipgloss.Width(segs[i])
		total += ws[i]
	}
	if len(hints) > 1 {
		total += sepW * (len(hints) - 1)
	}
	gap := boxInnerW - leftW - total
	if gap < 1 {
		gap = 1
	}

	// Box outer X + border(1) + padding(1) + label + gap = first hint X.
	hintStartX := boxLeftX + 2 + leftW + gap
	if t != nil {
		t.startX = hintStartX
		t.segWs = ws
		t.hints = hints
	}

	var b strings.Builder
	b.WriteString(leftRendered)
	b.WriteString(strings.Repeat(" ", gap))
	for i, s := range segs {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(s)
	}
	return b.String()
}

// SetY records the absolute terminal Y of the title row so subsequent hit
// tests work against the same coordinate space as OnClick(x, y, width).
func (t *titleHintRow) SetY(y int) {
	if t != nil {
		t.y = y
	}
}

// hit returns the Cmd of the segment under (x, y), or nil when the click
// lands outside any segment or the row Y. Safe on a nil receiver.
func (t *titleHintRow) hit(x, y int) tea.Cmd {
	if t == nil || y != t.y || len(t.segWs) == 0 {
		return nil
	}
	sepW := lipgloss.Width(Mute.Render("· "))
	cursor := t.startX
	for i, w := range t.segWs {
		if x >= cursor && x < cursor+w {
			return t.hints[i].Cmd()
		}
		cursor += w + sepW
	}
	return nil
}

// keyCmd returns a Cmd factory that synthesizes a single-key KeyMsg. Use
// for [n]/[x]/[a]/[E]/[d] glyphs whose keyboard handler already exists —
// avoids inventing a parallel click-only msg type per action.
func keyCmd(key string) func() tea.Cmd {
	return func() tea.Cmd {
		return func() tea.Msg {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}
	}
}

// wrappedHeight returns the number of lines `s` occupies when soft-wrapped
// to `width` cells. Honours existing newlines in `s` (each \n counts as a
// hard break that adds at least one extra row regardless of width). Returns
// 1 for the degenerate cases (empty, zero/negative width).
func wrappedHeight(s string, width int) int {
	if s == "" || width <= 0 {
		return 1
	}
	total := 0
	for _, line := range strings.Split(s, "\n") {
		w := lipgloss.Width(line)
		if w == 0 {
			total++
			continue
		}
		total += (w + width - 1) / width
	}
	if total < 1 {
		return 1
	}
	return total
}
