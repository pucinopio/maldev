package widgets

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/tui"
)

// RowClickedMsg is dispatched when the user clicks a row in a WrappedTable.
type RowClickedMsg struct{ Index int }

// WrappedTable wraps bubbles/table.Model as a tui.Widget.
// Clicking a row sends RowClickedMsg; keyboard navigation works via bubbles.
type WrappedTable struct {
	inner  table.Model
	bounds tui.Rect
}

// NewWrappedTable constructs a WrappedTable from an already-configured bubbles table.
func NewWrappedTable(t table.Model) *WrappedTable {
	return &WrappedTable{inner: t}
}

func (wt *WrappedTable) Layout(bounds tui.Rect) {
	wt.bounds = bounds
	wt.inner.SetWidth(bounds.W)
	wt.inner.SetHeight(bounds.H)
}

func (wt *WrappedTable) Bounds() tui.Rect { return wt.bounds }

func (wt *WrappedTable) Update(msg tea.Msg) (tui.Widget, tea.Cmd) {
	updated, cmd := wt.inner.Update(msg)
	wt.inner = updated
	return wt, cmd
}

func (wt *WrappedTable) View() string { return wt.inner.View() }

// OnClick implements tui.Clickable. y is relative to bounds.Y.
// bubbles/table rows start at row 1 (row 0 is the header).
func (wt *WrappedTable) OnClick(_, y int, _ tea.MouseButton) tea.Cmd {
	// Subtract 1 for header row; clamp to valid range.
	idx := y - 1
	rows := wt.inner.Rows()
	if idx < 0 || idx >= len(rows) {
		return nil
	}
	return func() tea.Msg { return RowClickedMsg{Index: idx} }
}

// Inner returns the underlying bubbles table (for direct manipulation).
func (wt *WrappedTable) Inner() table.Model { return wt.inner }
