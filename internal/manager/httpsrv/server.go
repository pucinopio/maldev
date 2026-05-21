// Package httpsrv exposes lifecycle-managed HTTP servers backed by the
// license-manager domain services. Three server implementations live in
// this package — revocation (T18), heartbeat (T19), probe (T20) — sharing
// a common Server interface so the TUI can manage them uniformly.
package httpsrv

import (
	"context"
	"time"
)

// Server is the lifecycle contract every license-manager HTTP server obeys.
// Start binds the listener and returns once the server is serving. Stop is
// graceful (in-flight requests complete up to timeout). Status is a
// non-blocking snapshot suitable for the TUI status bar. Events streams
// per-request notifications for the live log view; the channel is
// buffered (256) and drops the oldest event on overflow.
type Server interface {
	Name() string
	Start(ctx context.Context) error
	Stop(timeout time.Duration) error
	Status() Status
	Events() <-chan Event
}

// Status is a snapshot of a server's runtime state.
type Status struct {
	Running    bool
	ListenAddr string
	StartedAt  time.Time
	Requests   uint64
	LastReq    time.Time
	LastError  string
}

// Event describes a single HTTP request observed by a server, or a
// lifecycle transition. Server is the server's Name() so a merged event
// stream remains attributable.
type Event struct {
	At     time.Time
	Server string
	Kind   string // "started" | "stopped" | "request" | "error"
	Method string
	Path   string
	Status int
	Remote string
	Note   string
}
