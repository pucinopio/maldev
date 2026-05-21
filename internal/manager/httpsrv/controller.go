package httpsrv

import "context"

// Controller is the subset of Bundle behaviour the TUI needs: start/stop
// individual servers, snapshot statuses, and access the merged event stream.
// Defining the interface here keeps httpsrv self-contained and lets the TUI
// test with a mock without importing the real Bundle.
type Controller interface {
	// Start starts the named server ("revocation", "heartbeat", "probe").
	Start(ctx context.Context, name string) error

	// Stop stops the named server with a 5-second graceful timeout.
	Stop(name string) error

	// Statuses returns a snapshot of all three server statuses, keyed by name.
	Statuses() map[string]Status

	// MergedEvents returns the fan-in channel — same semantics as Bundle.MergedEvents.
	MergedEvents() <-chan Event
}
