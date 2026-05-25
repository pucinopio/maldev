package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Direction enumerates layout axes.
type Direction int

const (
	Horizontal Direction = iota
	Vertical
)

// FlexChild is a child plus layout constraints.
// Min is the minimum cells in the flow direction.
// Max is 0 = no maximum.
// Flex > 0 means share remaining space proportionally; Flex == 0 means use Min.
type FlexChild struct {
	W    Widget
	Min  int
	Max  int
	Flex int
}

// flexWidget is the composite returned by NewFlex().
type flexWidget struct {
	dir      Direction
	gap      int
	children []FlexChild
	bounds   Rect
}

// NewFlex lays out children in a row (Horizontal) or column (Vertical), assigning
// space proportionally by Flex factor. Returns a composite Widget.
func NewFlex(dir Direction, gap int, children ...FlexChild) Widget {
	return &flexWidget{dir: dir, gap: gap, children: children}
}

func (f *flexWidget) Bounds() Rect { return f.bounds }

func (f *flexWidget) Layout(bounds Rect) {
	f.bounds = bounds

	if len(f.children) == 0 {
		return
	}

	// Total space in the flow direction.
	total := bounds.W
	if f.dir == Vertical {
		total = bounds.H
	}

	// Account for gaps between children.
	gapTotal := gapCells(f.gap, len(f.children))
	available := total - gapTotal
	if available < 0 {
		available = 0
	}

	// Sum fixed allocations and flex factors.
	fixedSum := 0
	flexSum := 0
	for _, c := range f.children {
		if c.Flex <= 0 {
			fixedSum += c.Min
		} else {
			flexSum += c.Flex
		}
	}

	remaining := available - fixedSum
	if remaining < 0 {
		remaining = 0
	}

	// Assign sizes and compute bounds for each child.
	cursor := 0
	for i, c := range f.children {
		var size int
		if c.Flex <= 0 {
			size = c.Min
		} else {
			if flexSum > 0 {
				size = remaining * c.Flex / flexSum
			}
			if c.Min > 0 && size < c.Min {
				size = c.Min
			}
			if c.Max > 0 && size > c.Max {
				size = c.Max
			}
		}
		// Last flex child absorbs rounding remainder.
		if i == len(f.children)-1 && c.Flex > 0 {
			used := 0
			for _, prev := range f.children[:i] {
				if prev.Flex <= 0 {
					used += prev.Min
				}
			}
			size = available - used - cursor
			if size < 0 {
				size = 0
			}
		}

		var cb Rect
		if f.dir == Horizontal {
			cb = Rect{X: bounds.X + cursor, Y: bounds.Y, W: size, H: bounds.H}
		} else {
			cb = Rect{X: bounds.X, Y: bounds.Y + cursor, W: bounds.W, H: size}
		}
		c.W.Layout(cb)
		f.children[i] = c
		cursor += size + f.gap
	}
}

func (f *flexWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	var cmds []tea.Cmd
	for i, c := range f.children {
		updated, cmd := c.W.Update(msg)
		f.children[i].W = updated
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return f, tea.Batch(cmds...)
}

func (f *flexWidget) View() string {
	views := make([]string, 0, len(f.children)*2)
	for i, c := range f.children {
		if i > 0 && f.gap > 0 {
			// Layout reserved `gap` cells between children; insert a spacer at
			// View() time so JoinHorizontal/Vertical doesn't render them flush.
			// For Vertical: JoinVertical already inserts one \n between elements,
			// so the spacer is `gap-1` blank rows (a string of newlines).
			if f.dir == Horizontal {
				views = append(views, strings.Repeat(" ", f.gap))
			} else if f.gap >= 1 {
				views = append(views, strings.Repeat("\n", f.gap-1))
			}
		}
		views = append(views, c.W.View())
	}
	if f.dir == Horizontal {
		return lipgloss.JoinHorizontal(lipgloss.Top, views...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, views...)
}

// Children exposes child widgets for depth-first mouse dispatch.
func (f *flexWidget) Children() []Widget {
	cs := make([]Widget, len(f.children))
	for i, c := range f.children {
		cs[i] = c.W
	}
	return cs
}

// gapCells computes total gap cells for n children.
func gapCells(g, n int) int {
	if n <= 1 {
		return 0
	}
	return g * (n - 1)
}

// ── Grid ──────────────────────────────────────────────────────────────────────

// GridChild positions a Widget in a grid cell with optional spans.
type GridChild struct {
	W               Widget
	Row, Col        int
	RowSpan, ColSpan int
}

// gridWidget implements a fixed rows×cols grid.
type gridWidget struct {
	rows, cols int
	gap        int
	children   []GridChild
	bounds     Rect
	rowH       []int
	colW       []int
}

// NewGrid lays out children in a 2D grid. Row heights and column widths are
// distributed evenly; spans are respected.
func NewGrid(rows, cols int, gap int, children ...GridChild) Widget {
	return &gridWidget{rows: rows, cols: cols, gap: gap, children: children}
}

func (g *gridWidget) Bounds() Rect { return g.bounds }

func (g *gridWidget) Layout(bounds Rect) {
	g.bounds = bounds

	totalW := bounds.W - gapCells(g.gap, g.cols)
	totalH := bounds.H - gapCells(g.gap, g.rows)
	if totalW < 0 {
		totalW = 0
	}
	if totalH < 0 {
		totalH = 0
	}

	g.colW = make([]int, g.cols)
	g.rowH = make([]int, g.rows)
	for c := range g.colW {
		g.colW[c] = totalW / g.cols
	}
	for r := range g.rowH {
		g.rowH[r] = totalH / g.rows
	}
	// Absorb rounding into last cell.
	if g.cols > 0 {
		used := 0
		for _, w := range g.colW[:g.cols-1] {
			used += w
		}
		g.colW[g.cols-1] = totalW - used
	}
	if g.rows > 0 {
		used := 0
		for _, h := range g.rowH[:g.rows-1] {
			used += h
		}
		g.rowH[g.rows-1] = totalH - used
	}

	for i, ch := range g.children {
		if ch.Row >= g.rows || ch.Col >= g.cols {
			continue
		}
		rs := ch.RowSpan
		if rs <= 0 {
			rs = 1
		}
		cs := ch.ColSpan
		if cs <= 0 {
			cs = 1
		}
		x, y := bounds.X, bounds.Y
		for c := 0; c < ch.Col; c++ {
			x += g.colW[c] + g.gap
		}
		for r := 0; r < ch.Row; r++ {
			y += g.rowH[r] + g.gap
		}
		w, h := 0, 0
		for c := ch.Col; c < ch.Col+cs && c < g.cols; c++ {
			w += g.colW[c]
			if c > ch.Col {
				w += g.gap
			}
		}
		for r := ch.Row; r < ch.Row+rs && r < g.rows; r++ {
			h += g.rowH[r]
			if r > ch.Row {
				h += g.gap
			}
		}
		ch.W.Layout(Rect{X: x, Y: y, W: w, H: h})
		g.children[i] = ch
	}
}

func (g *gridWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	var cmds []tea.Cmd
	for i, ch := range g.children {
		updated, cmd := ch.W.Update(msg)
		g.children[i].W = updated
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return g, tea.Batch(cmds...)
}

func (g *gridWidget) View() string {
	// Build rows × cols cell grid.
	type cell struct {
		view    string
		x, y    int
	}
	placed := make([]cell, len(g.children))
	for i, ch := range g.children {
		b := ch.W.Bounds()
		placed[i] = cell{view: ch.W.View(), x: b.X, y: b.Y}
	}

	// Simple row-based rendering: collect children per row and join.
	rows := make([]string, g.rows)
	for r := 0; r < g.rows; r++ {
		var cols []string
		for c := 0; c < g.cols; c++ {
			rb := g.bounds.Y
			for ri := 0; ri < r; ri++ {
				rb += g.rowH[ri] + g.gap
			}
			cb := g.bounds.X
			for ci := 0; ci < c; ci++ {
				cb += g.colW[ci] + g.gap
			}
			for _, p := range placed {
				if p.x == cb && p.y == rb {
					cols = append(cols, p.view)
					break
				}
			}
		}
		rows[r] = lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	}
	return strings.Join(rows, "\n")
}

// Children exposes child widgets for depth-first mouse dispatch.
func (g *gridWidget) Children() []Widget {
	cs := make([]Widget, len(g.children))
	for i, c := range g.children {
		cs[i] = c.W
	}
	return cs
}

// clampToHeight trims s to at most h lines, padding short content with blank
// lines if necessary. This enforces a hard height ceiling that lipgloss.Height()
// cannot provide (it only pads, never truncates). The w parameter is unused but
// kept for call-site clarity — callers can pass 0 when padding is not needed.
func clampToHeight(s string, h, _ int) string {
	if h <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// truncateRunes returns s truncated to max visible rune positions, appending "…"
// when a cut is made. Used by sidebar and card layouts that must not wrap.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// renderHealthBar renders a health-style bar: green at 100 %, yellow ≈ 30 %,
// red near 0 %. Use for "remaining time" / "remaining quota" style metrics
// where filling represents *health*, not *progress*. pct is clamped 0..1.
func renderHealthBar(w int, pct float64) string {
	if w < 3 {
		return ""
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	// Pick the gradient endpoints from the current "health" percentage so the
	// fill always reads coherently — full = green→green, mid = orange→green,
	// low = red→red.
	var start, end string
	switch {
	case pct >= 0.66:
		start, end = string(Palette.Yellow), string(Palette.Green)
	case pct >= 0.33:
		start, end = string(Palette.Red), string(Palette.Yellow)
	default:
		start, end = string(Palette.Red), string(Palette.Orange)
	}
	p := progress.New(
		progress.WithScaledGradient(start, end),
		progress.WithoutPercentage(),
		progress.WithWidth(w),
	)
	return p.ViewAs(pct)
}

// renderProgressBar renders a horizontal progress bar of width w with cur/total
// filled cells using the Magenta accent for the filled portion and Border colour
// for the remainder. Both screens (wizard, onboarding) share this helper.
// renderProgressBar wraps the bubbles/progress widget for the wizard +
// onboarding strips. The widget handles gradient fill, sub-cell rounding,
// and overflow protection (no more strings.Repeat negative-count panics).
func renderProgressBar(w, cur, total int) string {
	if w < 3 {
		return ""
	}
	if total <= 0 {
		total = 1
	}
	// Scaled gradient: cyan at 0 % → magenta at 100 %. Width-scaled (not
	// position-scaled) so the same colour ramp shows whatever the bar width.
	p := progress.New(
		progress.WithScaledGradient(string(Palette.Cyan), string(Palette.Magenta)),
		progress.WithoutPercentage(),
		progress.WithWidth(w),
	)
	pct := float64(cur) / float64(total)
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	return p.ViewAs(pct)
}

// ── Pad ───────────────────────────────────────────────────────────────────────

type padWidget struct {
	inner                      Widget
	top, right, bottom, left int
	bounds                     Rect
}

// NewPad wraps a widget with N cells of padding on each side.
func NewPad(w Widget, top, right, bottom, left int) Widget {
	return &padWidget{inner: w, top: top, right: right, bottom: bottom, left: left}
}

func (p *padWidget) Bounds() Rect { return p.bounds }

func (p *padWidget) Layout(bounds Rect) {
	p.bounds = bounds
	inner := Rect{
		X: bounds.X + p.left,
		Y: bounds.Y + p.top,
		W: bounds.W - p.left - p.right,
		H: bounds.H - p.top - p.bottom,
	}
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}
	p.inner.Layout(inner)
}

func (p *padWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	updated, cmd := p.inner.Update(msg)
	p.inner = updated
	return p, cmd
}

func (p *padWidget) View() string {
	return lipgloss.NewStyle().
		PaddingTop(p.top).PaddingRight(p.right).
		PaddingBottom(p.bottom).PaddingLeft(p.left).
		Render(p.inner.View())
}

// Children exposes the inner widget for depth-first mouse dispatch.
func (p *padWidget) Children() []Widget { return []Widget{p.inner} }

// ── Box ───────────────────────────────────────────────────────────────────────

type boxWidget struct {
	inner     Widget
	title     string
	titleHint string // right-aligned hint in the title row (e.g. "[k] gérer")
	onHint    func() tea.Cmd
	focused   bool
	bounds    Rect
}

// NewBoxWithHintClick is NewBoxWithHint plus an OnClick callback that fires
// when the operator left-clicks the title-row hint (the [k] gérer style label
// on the far right of the title bar).
func NewBoxWithHintClick(w Widget, title, hint string, focused bool, onHint func() tea.Cmd) Widget {
	return &boxWidget{inner: w, title: title, titleHint: hint, focused: focused, onHint: onHint}
}

// OnClick is the Clickable hook. The title row is at y=1 relative (y=0 is the
// top border). The hint occupies the trailing characters of that row, x is
// relative to bounds.X.
func (b *boxWidget) OnClick(x, y int, _ tea.MouseButton) tea.Cmd {
	if b.onHint == nil || b.titleHint == "" || y != 1 {
		return nil
	}
	// Title row layout: hint is right-aligned within contentW = bounds.W - 2
	// (visual width = Width(contentW) + border = bounds.W). The hint pill
	// includes the HintKey style padding (1 cell each side).
	hintW := lipgloss.Width(HintKey.Render(b.titleHint))
	hintEnd := b.bounds.W - 2  // last cell before right border + right padding
	hintStart := hintEnd - hintW
	if x >= hintStart-1 && x <= hintEnd+1 { // ±1 cell tolerance for fat-finger
		return b.onHint()
	}
	return nil
}

// NewBox wraps a widget with a bordered frame + optional title.
// When focused is true the border uses the Magenta accent colour.
func NewBox(w Widget, title string, focused bool) Widget {
	return &boxWidget{inner: w, title: title, focused: focused}
}

// NewBoxWithHint is like NewBox but adds a right-aligned hint string
// rendered in the title row (e.g. "[k] gérer"). The hint is rendered
// in the HintKey accent colour; the title remains in GlowCyan.
func NewBoxWithHint(w Widget, title, hint string, focused bool) Widget {
	return &boxWidget{inner: w, title: title, titleHint: hint, focused: focused}
}

func (b *boxWidget) Bounds() Rect { return b.bounds }

func (b *boxWidget) Layout(bounds Rect) {
	b.bounds = bounds
	// BoxStyle visual overhead: border(1) + padding(1) each side = 2 per side.
	// lipgloss Width(n) includes the padding in n, so the actual text area is
	// Width(n) - padding(2) = (bounds.W - 4) - 2 = bounds.W - 6.
	// The Y offset depends on whether we render a title row: yOff=2 (border +
	// title) when there is a title, else yOff=1 (border only). Without this
	// distinction, child Clickable.OnClick receives a relative-Y that is off
	// by 1 vs what the user sees (the title eats a row inside the inner area).
	yOff := 1
	hOff := 2
	if b.title != "" {
		yOff = 2
		hOff = 3
	}
	inner := Rect{
		X: bounds.X + 2,
		Y: bounds.Y + yOff,
		W: bounds.W - 6,
		H: bounds.H - hOff,
	}
	if inner.W < 0 {
		inner.W = 0
	}
	if inner.H < 0 {
		inner.H = 0
	}
	b.inner.Layout(inner)
}

func (b *boxWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	updated, cmd := b.inner.Update(msg)
	b.inner = updated
	return b, cmd
}

func (b *boxWidget) View() string {
	// lipgloss.Width(N) already subsumes Padding(0,1) — the rendered visual
	// width is N + 2 (border only adds outside Width). So pass bounds.W - 2 to
	// hit bounds.W exactly. (Verified empirically; the prior `-4` formula left
	// 2 trailing cells per box, visible as misalignment on the dashboard right.)
	contentW := b.bounds.W - 2
	if contentW < 1 {
		contentW = 1
	}
	st := BoxStyle.Width(contentW)
	if b.focused {
		st = BoxFocused.Width(contentW)
	}
	content := b.inner.View()
	if b.title != "" {
		// BoxStyle has Padding(0,1): 1 char left + 1 char right = 2 chars consumed
		// inside contentW. The title row must fit in contentW-2 to avoid wrapping.
		titleRow := b.renderTitleRow(contentW - 2)
		return st.Render(titleRow + "\n" + content)
	}
	return st.Render(content)
}

// renderTitleRow builds the title line for the box.
// When titleHint is set it is right-aligned within contentW; the main title
// sits flush-left in GlowCyan and the hint in HintKey (magenta bold).
// If the hint would not fit (gap < 1 after a mandatory single space) the hint
// is silently omitted so the title never wraps to a second line.
func (b *boxWidget) renderTitleRow(contentW int) string {
	title := GlowCyan.Render(b.title)
	if b.titleHint == "" {
		return title
	}
	hint := HintKey.Render(b.titleHint)
	titleW := lipgloss.Width(title)
	hintW := lipgloss.Width(hint)
	// Need at least 1 space between title and hint; drop hint if it won't fit.
	if titleW+1+hintW > contentW {
		return title
	}
	gap := contentW - titleW - hintW
	return title + strings.Repeat(" ", gap) + hint
}

// Children exposes the inner widget for depth-first mouse dispatch.
func (b *boxWidget) Children() []Widget { return []Widget{b.inner} }

// ── Shared list-screen helpers ──────────────────────────────────────────────
// These five primitives used to live in screen_licenses.go and
// screen_settings.go but are called from every list screen. They belong in
// layout.go alongside BoxedInner/BoxedWidth so a new screen author doesn't
// have to guess which feature file owns them.

// stretchLastColumn enlarges the rightmost table column so the row highlight
// spans the available screen width. The bubbles/table package only
// highlights cells, so without this the selected-row background stops at
// the natural column sum. Safe to call when width == 0 (no-op).
func stretchLastColumn(t *table.Model, width int) {
	if width <= 0 {
		return
	}
	cols := t.Columns()
	if len(cols) == 0 {
		return
	}
	fixed := 0
	for i := 0; i < len(cols)-1; i++ {
		fixed += cols[i].Width
	}
	overhead := 2*len(cols) + 2 // padding (1 per col) + outer borders
	last := width - fixed - overhead
	if last < cols[len(cols)-1].Width {
		last = cols[len(cols)-1].Width
	}
	cols[len(cols)-1].Width = last
	t.SetColumns(cols)
}

// emptyTableHint returns a centered muted line shown under a table that has
// no rows, hinting at the keybind that creates one. Returns "" when rows > 0.
func emptyTableHint(rows int, width int, message string) string {
	if rows > 0 || width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Foreground(Palette.FgMute).
		Render(message)
}

// kvRow renders a dim-key / fg-value row with an explicit key field width.
// Shared by settingsKV and detail-panel helpers across screen files.
func kvRow(key, value string, keyW int) string {
	return Dim.Render(fmt.Sprintf("%-*s", keyW, key)) + Base.Render(value)
}

// detailColW returns the width of each column in a 2-column detail panel
// given the total screen width. Ensures a minimum of 20 chars so labels
// are never truncated on narrow terminals.
func detailColW(totalW int) int {
	w := totalW/2 - 4
	if w < 20 {
		return 20
	}
	return w
}

// clampTableHeight applies the three universal constraints every list
// screen's rebuildTable repeats: halve when detail panel is open, clamp
// to a minimum of 3 rows so the header + 2 data rows are always visible,
// and collapse to 1 row when there are no entries so an empty-state hint
// can render directly below the header instead of being pushed off-screen.
func clampTableHeight(h int, detailOpen, empty bool) int {
	if detailOpen {
		h /= 2
	}
	if h < 3 {
		h = 3
	}
	if empty {
		h = 1
	}
	return h
}

// tableRowCmd is the shared row-click handler for list screens. headerY is
// the absolute Y of the table header (1 row above the first data row);
// rowsLen is the number of rows currently in the model. Returns nil when
// the click misses the data rows.
func tableRowCmd(headerY, rowsLen, y int) tea.Cmd {
	row := y - headerY - 1
	if row < 0 || row >= rowsLen {
		return nil
	}
	return func() tea.Msg { return tableSelectRowMsg{row: row} }
}
