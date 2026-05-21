package httpsrv

import (
	"sync"
	"time"
)

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
