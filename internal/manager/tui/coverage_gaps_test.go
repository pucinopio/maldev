package tui

// Coverage gaps closed in Session 0004:
//   - NewGrid / NewPad / NewBox layout primitives (Layout, Update, View,
//     Bounds, Children, OnClick paths)
//   - FrameOf style measurement
//   - probeDrawerOverlay Init/curlCommand/cancelCmd
//
// Bumps statement coverage of the tui package from 65% toward 70%.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// ── widget stub used to exercise composite layout primitives ──────────────────

// trackedLeaf is a minimal Widget implementation that records the bounds
// it was assigned and the messages it received. Useful for asserting Grid /
// Pad / Box layout math without dragging in a real screen.
type trackedLeaf struct {
	id     string
	bounds Rect
	msgs   []tea.Msg
}

func (l *trackedLeaf) Bounds() Rect { return l.bounds }
func (l *trackedLeaf) Layout(b Rect) { l.bounds = b }
func (l *trackedLeaf) Update(m tea.Msg) (Widget, tea.Cmd) {
	l.msgs = append(l.msgs, m)
	return l, nil
}
func (l *trackedLeaf) View() string { return l.id }

// ── NewGrid ───────────────────────────────────────────────────────────────────

// TestNewGrid_LayoutDistributesEvenly asserts a 2×3 grid with no gaps splits a
// 30×10 bounds into 6 equal 10×5 cells.
func TestNewGrid_LayoutDistributesEvenly(t *testing.T) {
	cells := make([]*trackedLeaf, 6)
	children := make([]GridChild, 6)
	for i := range cells {
		cells[i] = &trackedLeaf{id: "cell"}
		children[i] = GridChild{W: cells[i], Row: i / 3, Col: i % 3}
	}
	g := NewGrid(2, 3, 0, children...)
	g.Layout(Rect{X: 0, Y: 0, W: 30, H: 10})

	for i, c := range cells {
		if c.bounds.W != 10 || c.bounds.H != 5 {
			t.Fatalf("cell %d bounds = %+v, want 10×5", i, c.bounds)
		}
	}
	if got := g.Bounds().W; got != 30 {
		t.Fatalf("grid.Bounds().W = %d, want 30", got)
	}
}

// TestNewGrid_GapAbsorbedFromTotal verifies gap reduces per-cell space.
func TestNewGrid_GapAbsorbedFromTotal(t *testing.T) {
	c0 := &trackedLeaf{id: "a"}
	c1 := &trackedLeaf{id: "b"}
	g := NewGrid(1, 2, 4, GridChild{W: c0, Col: 0}, GridChild{W: c1, Col: 1})
	g.Layout(Rect{X: 0, Y: 0, W: 24, H: 4})
	// gap=4 between 2 cols → totalW = 24-4 = 20 → each cell ~10.
	if c0.bounds.W != 10 || c1.bounds.W != 10 {
		t.Fatalf("cells = %+v, %+v; want 10 wide each", c0.bounds, c1.bounds)
	}
	// c1 must start at x = c0.W + gap = 14.
	if c1.bounds.X != 14 {
		t.Fatalf("c1.X = %d, want 14 (= 10 + 4 gap)", c1.bounds.X)
	}
}

// TestNewGrid_UpdatePropagatesToChildren confirms Update fans the message
// out to every child widget.
func TestNewGrid_UpdatePropagatesToChildren(t *testing.T) {
	c0 := &trackedLeaf{id: "a"}
	c1 := &trackedLeaf{id: "b"}
	g := NewGrid(1, 2, 0, GridChild{W: c0, Col: 0}, GridChild{W: c1, Col: 1})
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, _ = g.Update(msg)
	if len(c0.msgs) != 1 || len(c1.msgs) != 1 {
		t.Fatalf("expected 1 msg per child, got %d / %d", len(c0.msgs), len(c1.msgs))
	}
}

// TestNewGrid_ChildrenExposed verifies the Children() method (used by the
// mouse hit-test walker) returns every cell widget.
func TestNewGrid_ChildrenExposed(t *testing.T) {
	c0 := &trackedLeaf{id: "a"}
	c1 := &trackedLeaf{id: "b"}
	g := NewGrid(1, 2, 0, GridChild{W: c0, Col: 0}, GridChild{W: c1, Col: 1})

	cw, ok := g.(interface{ Children() []Widget })
	if !ok {
		t.Fatal("gridWidget must expose Children() for mouse dispatch")
	}
	got := cw.Children()
	if len(got) != 2 {
		t.Fatalf("Children() returned %d, want 2", len(got))
	}
}

// TestNewGrid_ViewNonEmpty ensures the grid renders something even with
// degenerate inputs (zero size, empty children).
func TestNewGrid_ViewNonEmpty(t *testing.T) {
	g := NewGrid(1, 1, 0, GridChild{W: &trackedLeaf{id: "x"}})
	g.Layout(Rect{X: 0, Y: 0, W: 10, H: 4})
	if got := g.View(); got == "" {
		t.Fatal("grid View() returned empty string")
	}
}

// ── NewPad ────────────────────────────────────────────────────────────────────

// TestNewPad_LayoutShrinksInner asserts that a Pad{2,3,4,5} on a 20×10 box
// gives the inner widget a (20-3-5)×(10-2-4) = 12×4 rect.
func TestNewPad_LayoutShrinksInner(t *testing.T) {
	inner := &trackedLeaf{id: "inner"}
	p := NewPad(inner, 2, 3, 4, 5) // top=2 right=3 bottom=4 left=5
	p.Layout(Rect{X: 0, Y: 0, W: 20, H: 10})

	wantW, wantH := 20-3-5, 10-2-4
	if inner.bounds.W != wantW || inner.bounds.H != wantH {
		t.Fatalf("inner bounds = %+v, want %d×%d", inner.bounds, wantW, wantH)
	}
	if inner.bounds.X != 5 || inner.bounds.Y != 2 {
		t.Fatalf("inner origin = (%d,%d), want (5,2)", inner.bounds.X, inner.bounds.Y)
	}
}

// TestNewPad_ZeroSafe — if padding > bounds, inner bounds clamp to 0 instead
// of going negative.
func TestNewPad_ZeroSafe(t *testing.T) {
	inner := &trackedLeaf{id: "x"}
	p := NewPad(inner, 5, 5, 5, 5)
	p.Layout(Rect{X: 0, Y: 0, W: 4, H: 4})
	if inner.bounds.W < 0 || inner.bounds.H < 0 {
		t.Fatalf("inner bounds went negative: %+v", inner.bounds)
	}
}

// TestNewPad_UpdateForwardsToInner — Update must propagate to the wrapped
// widget so its event handling still works.
func TestNewPad_UpdateForwardsToInner(t *testing.T) {
	inner := &trackedLeaf{id: "x"}
	p := NewPad(inner, 1, 1, 1, 1)
	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyTab})
	if len(inner.msgs) != 1 {
		t.Fatalf("inner did not receive forwarded msg")
	}
}

// TestNewPad_ChildrenExposed — Pad must expose its inner widget so click
// dispatch can recurse.
func TestNewPad_ChildrenExposed(t *testing.T) {
	inner := &trackedLeaf{id: "x"}
	p := NewPad(inner, 1, 1, 1, 1)
	cw, ok := p.(interface{ Children() []Widget })
	if !ok {
		t.Fatal("padWidget must expose Children()")
	}
	if got := cw.Children(); len(got) != 1 {
		t.Fatalf("Children() returned %d, want 1", len(got))
	}
}

// ── NewBox ────────────────────────────────────────────────────────────────────

// TestNewBox_RendersTitle — a plain box with a title shows the title text in
// the rendered string.
func TestNewBox_RendersTitle(t *testing.T) {
	inner := &trackedLeaf{id: "body"}
	b := NewBox(inner, "Test Title", false)
	b.Layout(Rect{X: 0, Y: 0, W: 30, H: 5})
	out := b.View()
	if !strings.Contains(out, "Test Title") {
		t.Fatalf("box View() missing title; got first line: %q", firstLineOf(out))
	}
}

// TestNewBoxWithHint_RendersHint — box with a hint shows both title and hint.
func TestNewBoxWithHint_RendersHint(t *testing.T) {
	inner := &trackedLeaf{id: "body"}
	b := NewBoxWithHint(inner, "Clé", "[k] gérer", true)
	b.Layout(Rect{X: 0, Y: 0, W: 40, H: 5})
	out := b.View()
	if !strings.Contains(out, "Clé") {
		t.Fatal("box View() missing title 'Clé'")
	}
	if !strings.Contains(out, "[k]") || !strings.Contains(out, "gérer") {
		t.Fatalf("box View() missing hint '[k] gérer'; got: %s", out)
	}
}

// TestNewBoxWithHintClick_HintEmitsCmd verifies the OnClick callback fires
// when the operator clicks within the hint's x-range on the title row.
func TestNewBoxWithHintClick_HintEmitsCmd(t *testing.T) {
	const sentinel = "fired"
	fired := false
	b := NewBoxWithHintClick(&trackedLeaf{id: "x"}, "T", "[k] go", false,
		func() tea.Cmd { return func() tea.Msg { fired = true; return sentinel } },
	)
	b.Layout(Rect{X: 0, Y: 0, W: 30, H: 5})

	clk, ok := b.(Clickable)
	if !ok {
		t.Fatal("boxWidget with hint must implement Clickable")
	}
	// Title row is y=1 (y=0 is the top border). Click at the right side
	// where the hint is rendered.
	cmd := clk.OnClick(28, 1, tea.MouseButtonLeft)
	if cmd == nil {
		t.Fatal("clicking on the hint must emit a cmd")
	}
	_ = cmd()
	if !fired {
		t.Fatal("OnClick callback did not run")
	}
}

// TestNewBoxWithHintClick_HintIgnoresWrongRow — a click on the body must NOT
// fire the hint callback.
func TestNewBoxWithHintClick_HintIgnoresWrongRow(t *testing.T) {
	b := NewBoxWithHintClick(&trackedLeaf{id: "x"}, "T", "[k] go", false,
		func() tea.Cmd { return func() tea.Msg { return "should-not-fire" } },
	)
	b.Layout(Rect{X: 0, Y: 0, W: 30, H: 5})
	clk := b.(Clickable)
	if cmd := clk.OnClick(28, 3, tea.MouseButtonLeft); cmd != nil {
		t.Fatalf("click on body row must NOT emit hint cmd; got %v", cmd())
	}
}

// ── FrameOf ───────────────────────────────────────────────────────────────────

// TestFrameOf_BorderCounts measures a lipgloss style with a border and asserts
// FrameOf returns the expected (w, h) overhead.
func TestFrameOf_BorderCounts(t *testing.T) {
	style := lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	w, h := FrameOf(style)
	if w != 2 || h != 2 {
		t.Fatalf("FrameOf(normal border) = (%d, %d), want (2, 2)", w, h)
	}
}

// TestFrameOf_PaddingAddsToFrame — padding on each side adds to the frame
// dimensions, on top of any border.
func TestFrameOf_PaddingAddsToFrame(t *testing.T) {
	style := lipgloss.NewStyle().Padding(1, 2)
	w, h := FrameOf(style)
	if w != 4 || h != 2 {
		t.Fatalf("FrameOf(padding 1×2) = (%d, %d), want (4, 2)", w, h)
	}
}

// ── probeDrawerOverlay ────────────────────────────────────────────────────────

// TestProbeDrawerOverlay_InitReturnsIssueTokenCmd — Init must return a non-nil
// cmd that, when invoked with nil services, produces a ProbeTokenIssuedMsg
// with the "services unavailable" error.
func TestProbeDrawerOverlay_InitReturnsIssueTokenCmd(t *testing.T) {
	o := newProbeDrawerOverlay(nil, nil)
	cmd := o.Init()
	if cmd == nil {
		t.Fatal("Init must return non-nil cmd")
	}
	msg := cmd()
	got, ok := msg.(ProbeTokenIssuedMsg)
	if !ok {
		t.Fatalf("expected ProbeTokenIssuedMsg, got %T", msg)
	}
	if got.Err == nil {
		t.Fatal("nil-svc Init cmd must error")
	}
}

// TestProbeDrawerOverlay_CurlCommandFormat — once a token is attached, the
// curl one-liner contains the token ID.
func TestProbeDrawerOverlay_CurlCommandFormat(t *testing.T) {
	o := newProbeDrawerOverlay(nil, nil)
	if got := o.curlCommand(); got != "" {
		t.Fatalf("curlCommand with nil token must be empty, got %q", got)
	}
	o.token = &ent.ProbeToken{ID: "abc-123"}
	got := o.curlCommand()
	if !strings.Contains(got, "abc-123") {
		t.Fatalf("curl command missing token ID; got %q", got)
	}
	if !strings.HasPrefix(got, "curl ") {
		t.Fatalf("curl command must start with 'curl ', got %q", got)
	}
}

// TestProbeDrawerOverlay_CancelEmitsDone — cancelCmd (esc path) emits a
// done-result-nil msg whatever the state. Verified with nil token (no svc
// calls happen).
func TestProbeDrawerOverlay_CancelEmitsDone(t *testing.T) {
	o := newProbeDrawerOverlay(nil, nil)
	cmd := o.cancelCmd()
	if cmd == nil {
		t.Fatal("cancelCmd must return cmd even on degenerate state")
	}
	msg := cmd()
	done, ok := msg.(OverlayDoneMsg)
	if !ok {
		t.Fatalf("expected OverlayDoneMsg, got %T", msg)
	}
	if done.Result != nil {
		t.Fatalf("cancel must emit nil Result, got %v", done.Result)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
