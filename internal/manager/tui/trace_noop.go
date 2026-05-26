//go:build !tui_trace

package tui

// traceMsg is the no-op variant of the trace hook. The real implementation
// lives in trace_enabled.go behind the `tui_trace` build tag. Both stub +
// real versions share the same signature so call sites never branch.
func traceMsg(_ string, _ interface{}) {}
