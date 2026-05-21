// Package tui implements the bubbletea-based terminal UI for license-manager.
package tui

import "github.com/oioio-space/maldev/internal/manager/tui/core"

// Re-export core types so callers import only "tui", not "tui/core".

// Rect is a bounding box in terminal cell coordinates.
type Rect = core.Rect

// Widget is the base abstraction for every visible piece of UI.
type Widget = core.Widget

// Clickable is the optional interface for widgets that handle mouse clicks.
type Clickable = core.Clickable

// Focusable widgets receive Tab focus and explicit key events.
type Focusable = core.Focusable
