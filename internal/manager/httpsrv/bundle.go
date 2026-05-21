package httpsrv

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Ensure Bundle satisfies Controller at compile time.
var _ Controller = (*Bundle)(nil)

// Bundle aggregates the three server instances and exposes a single merged
// event stream for the TUI live-log view.
type Bundle struct {
	Revocation *RevocationServer
	Heartbeat  *HeartbeatServer
	Probe      *ProbeServer

	mu     sync.Mutex
	merged chan Event
	done   chan struct{}
}

// NewBundle constructs the bundle but does NOT start any server.
func NewBundle(rev *RevocationServer, hb *HeartbeatServer, pb *ProbeServer) *Bundle {
	return &Bundle{Revocation: rev, Heartbeat: hb, Probe: pb}
}

// MergedEvents returns a fan-in channel of events from all three servers. The
// first call starts the fan-in goroutines; subsequent calls return the same
// channel. Buffer capacity 512 with drop-oldest semantics on overflow.
func (b *Bundle) MergedEvents() <-chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.merged != nil {
		return b.merged
	}
	b.merged = make(chan Event, 512)
	b.done = make(chan struct{})
	// Use typed nil checks — a non-nil Server interface holding a nil pointer
	// would pass an interface == nil test but panic on method call.
	if b.Revocation != nil {
		go b.forward(b.Revocation.Events())
	}
	if b.Heartbeat != nil {
		go b.forward(b.Heartbeat.Events())
	}
	if b.Probe != nil {
		go b.forward(b.Probe.Events())
	}
	return b.merged
}

// forward fans one server's event channel into the merged channel.
func (b *Bundle) forward(src <-chan Event) {
	for {
		select {
		case e, ok := <-src:
			if !ok {
				return
			}
			select {
			case b.merged <- e:
			default:
				// Drop oldest so the live log stays responsive.
				select {
				case <-b.merged:
				default:
				}
				select {
				case b.merged <- e:
				default:
				}
			}
		case <-b.done:
			return
		}
	}
}

// Start starts the named server ("revocation", "heartbeat", "probe").
// It satisfies Controller so the TUI can manage servers uniformly.
func (b *Bundle) Start(ctx context.Context, name string) error {
	switch name {
	case "revocation":
		if b.Revocation == nil {
			return fmt.Errorf("revocation server not configured")
		}
		return b.Revocation.Start(ctx)
	case "heartbeat":
		if b.Heartbeat == nil {
			return fmt.Errorf("heartbeat server not configured")
		}
		return b.Heartbeat.Start(ctx)
	case "probe":
		if b.Probe == nil {
			return fmt.Errorf("probe server not configured")
		}
		return b.Probe.Start(ctx)
	default:
		return fmt.Errorf("unknown server %q", name)
	}
}

// Stop stops the named server with a 5-second graceful timeout.
// It satisfies Controller so the TUI can manage servers uniformly.
func (b *Bundle) Stop(name string) error {
	const timeout = 5 * time.Second
	switch name {
	case "revocation":
		if b.Revocation == nil {
			return nil
		}
		return b.Revocation.Stop(timeout)
	case "heartbeat":
		if b.Heartbeat == nil {
			return nil
		}
		return b.Heartbeat.Stop(timeout)
	case "probe":
		if b.Probe == nil {
			return nil
		}
		return b.Probe.Stop(timeout)
	default:
		return fmt.Errorf("unknown server %q", name)
	}
}

// Statuses returns a snapshot of all three servers, keyed by Name().
// It satisfies Controller so the TUI can render status cards without knowing
// concrete server types.
func (b *Bundle) Statuses() map[string]Status {
	out := make(map[string]Status, 3)
	if b.Revocation != nil {
		out[b.Revocation.Name()] = b.Revocation.Status()
	}
	if b.Heartbeat != nil {
		out[b.Heartbeat.Name()] = b.Heartbeat.Status()
	}
	if b.Probe != nil {
		out[b.Probe.Name()] = b.Probe.Status()
	}
	return out
}

// StopAll stops every server with the given timeout and closes the fan-in
// goroutines. Returns the first error encountered; remaining servers are
// still stopped.
func (b *Bundle) StopAll(timeout time.Duration) error {
	var first error
	if b.Revocation != nil {
		if err := b.Revocation.Stop(timeout); err != nil && first == nil {
			first = err
		}
	}
	if b.Heartbeat != nil {
		if err := b.Heartbeat.Stop(timeout); err != nil && first == nil {
			first = err
		}
	}
	if b.Probe != nil {
		if err := b.Probe.Stop(timeout); err != nil && first == nil {
			first = err
		}
	}
	b.mu.Lock()
	if b.done != nil {
		close(b.done)
		b.done = nil
	}
	b.mu.Unlock()
	return first
}
