# TUI Widget System

## What the Widget interface buys

Before Phase 2.5, every screen's `View()` was a flat lipgloss composition — correct,
but hard to evolve: adding a border, reshuffling columns, or wiring mouse events required
editing deep string-concatenation code. The `Widget` interface turns UI composition into
a tree of typed objects with clear responsibilities:

- **Layout** receives bounds (set on resize, not per frame).
- **Update** processes any `tea.Msg` (keyboard, mouse, data, timers).
- **View** renders within bounds — never overflows.

## Package structure

```
internal/manager/tui/
├── core/core.go        — Widget, Clickable, Focusable interfaces + Rect (cycle-free base)
├── widget.go           — type aliases re-exporting core types as tui.Widget / tui.Rect
├── layout.go           — Flex, Grid, Pad, Box composites
└── widgets/            — leaf widgets: Text, Spacer, Button, Tile, TabBar, StatusBar,
                          WrappedTable, WrappedTextInput, WrappedViewport
```

The `tui/core` sub-package exists solely to break the import cycle:
`tui` → `tui/widgets` → `tui`. Both sides import `tui/core`; callers use
`tui.Widget` / `tui.Rect` via the type aliases in `widget.go`.

## Rect / bounds model

`tui.Rect{X, Y, W, H}` uses terminal cell coordinates (column, row).
`Rect.Contains(x, y)` is used by the mouse dispatcher to hit-test.

Bounds are assigned top-down: the root calls `Layout` on the top-level widget,
composite widgets (Flex, Grid, Pad, Box) recursively assign child bounds, and leaf
widgets store the result and use it in `View()`.

## Layout primitives

| Primitive | Description |
|-----------|-------------|
| `NewFlex(dir, gap, children...)` | Row or column; `FlexChild.Flex > 0` = proportional share of remaining space; `Flex == 0` = fixed `Min` cells. |
| `NewGrid(rows, cols, gap, children...)` | Fixed 2D grid; `GridChild.RowSpan/ColSpan` for merged cells. |
| `NewPad(w, top, right, bottom, left)` | Insets inner widget. |
| `NewBox(w, title, focused)` | Bordered frame; Magenta border when focused. |

Composites implement a private `Children() []Widget` interface used by the mouse
dispatcher for depth-first traversal.

## Mouse dispatching

Mouse is enabled with `tea.WithMouseCellMotion()` in `cmd/license-manager/main.go`.
On `tea.MouseActionRelease + tea.MouseButtonLeft`, `app.go` calls
`dispatchClick(tree, msg.X, msg.Y)` which:

1. Checks `Bounds().Contains` — returns `nil` if outside.
2. Recurses into children first (deepest widget wins).
3. If the matched widget implements `tui.Clickable`, calls `OnClick(relX, relY, button)`.

Tab bar clicks (Y == 1) are dispatched directly to the `TabBar` widget before
the general tree walk, since the tab bar is part of chrome rather than a screen widget tree.

## Mouse-clickable surfaces (Phase 2.5)

| Surface | Action |
|---------|--------|
| Dashboard counter tiles | Switches to Licenses view with matching filter |
| Tab bar tabs | Switches to the clicked view (`widgets.SwitchViewMsg`) |
| Buttons (all screens) | Fires `OnPress` handler |
| `WrappedTable` rows | Emits `widgets.RowClickedMsg{Index}` |
| `WrappedTextInput` | Focuses the input |

## Migration recipe for legacy screens

Screens in `screen_licenses.go` and others still use direct lipgloss rendering.
To migrate a screen:

1. **Extract content helpers** — pull `renderXxxCard()` into methods that return `string`.
2. **Wrap helpers in `widgets.Text`** — `widgets.NewText(content, style)`.
3. **Build a Flex/Box tree** — replace `lipgloss.JoinVertical/Horizontal` with `tui.NewFlex(…)`.
4. **Call `root.Layout(tui.Rect{…})`** at the end of `buildWidgetTree()`.
5. **Return `root.View()`** from `View()`.
6. **Wire clicks** — in `app.go`'s `handleMouse`, add a case for the new screen's
   active view that calls `m.theScreen.buildWidgetTree()` then `dispatchClick(…)`.
7. **Add any `Clickable` handlers** — `SwitchViewMsg`, `RowClickedMsg`, etc.

The migration is non-breaking: the other screens continue working unchanged.

## Examples

### Horizontal three-column layout

```go
left   := widgets.NewText("Left",   lipgloss.NewStyle())
center := widgets.NewText("Center", lipgloss.NewStyle())
right  := widgets.NewText("Right",  lipgloss.NewStyle())

row := tui.NewFlex(tui.Horizontal, 1,
    tui.FlexChild{W: left,   Flex: 1},
    tui.FlexChild{W: center, Flex: 2}, // center gets 2× the space
    tui.FlexChild{W: right,  Flex: 1},
)
row.Layout(tui.Rect{X: 0, Y: 0, W: 120, H: 20})
fmt.Print(row.View())
```

### Clickable tile

```go
tile := widgets.NewTile("Active", 42, "", tui.Palette.Green, func() tea.Cmd {
    return func() tea.Msg { return SwitchToLicensesMsg{Filter: "active"} }
})
tile.Layout(tui.Rect{X: 0, Y: 0, W: 28, H: 5})
// tile.OnClick(x, y, tea.MouseButtonLeft) fires the handler
```

### Bordered box with title

```go
content := widgets.NewText(someText, lipgloss.NewStyle())
box     := tui.NewBox(content, "Section Title", false)
box.Layout(tui.Rect{X: 0, Y: 0, W: 60, H: 10})
fmt.Print(box.View())
```
