package transport

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeTransport is the test double used across the router suite.
// Each instance owns a control surface that lets the test decide
// whether Connect succeeds, whether Read/Write surface errors, and
// when each operation should block. Implements [Transport].
type fakeTransport struct {
	name string

	// Behaviour knobs (write-once at setup).
	connectErr  error
	readErr     error
	writeErr    error
	connectHold time.Duration // delay before Connect returns

	// State (atomic).
	connectCalls atomic.Int32
	readCalls    atomic.Int32
	writeCalls   atomic.Int32
	closeCalls   atomic.Int32
}

func (f *fakeTransport) Connect(ctx context.Context) error {
	f.connectCalls.Add(1)
	if f.connectHold > 0 {
		select {
		case <-time.After(f.connectHold):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.connectErr
}
func (f *fakeTransport) Read(p []byte) (int, error) {
	f.readCalls.Add(1)
	if f.readErr != nil {
		return 0, f.readErr
	}
	copy(p, []byte("ok"))
	return 2, nil
}
func (f *fakeTransport) Write(p []byte) (int, error) {
	f.writeCalls.Add(1)
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(p), nil
}
func (f *fakeTransport) Close() error            { f.closeCalls.Add(1); return nil }
func (f *fakeTransport) RemoteAddr() net.Addr    { return fakeAddr(f.name) }

type fakeAddr string

func (a fakeAddr) Network() string { return "fake" }
func (a fakeAddr) String() string  { return string(a) }

// TestNewRouter_EmptyTransportsErrors pins the ErrNoTransports
// sentinel for the empty-input edge.
func TestNewRouter_EmptyTransportsErrors(t *testing.T) {
	if _, err := NewRouter(nil, RouterConfig{}); !errors.Is(err, ErrNoTransports) {
		t.Errorf("nil transports: got %v, want ErrNoTransports", err)
	}
	if _, err := NewRouter([]Transport{}, RouterConfig{}); !errors.Is(err, ErrNoTransports) {
		t.Errorf("empty transports: got %v, want ErrNoTransports", err)
	}
}

// TestNewRouter_DefaultsApplied confirms zero-value RouterConfig
// fields fall back to sane defaults.
func TestNewRouter_DefaultsApplied(t *testing.T) {
	r, err := NewRouter([]Transport{&fakeTransport{name: "a"}}, RouterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if r.cfg.InitialBackoff != time.Second {
		t.Errorf("InitialBackoff default = %v, want 1s", r.cfg.InitialBackoff)
	}
	if r.cfg.MaxBackoff != time.Minute {
		t.Errorf("MaxBackoff default = %v, want 1m", r.cfg.MaxBackoff)
	}
}

// TestRouter_Connect_FirstChannelWins exercises the happy path:
// transports[0] dials cleanly, the Router selects it, and Active
// reflects the choice without touching the fallback.
func TestRouter_Connect_FirstChannelWins(t *testing.T) {
	primary := &fakeTransport{name: "primary"}
	fallback := &fakeTransport{name: "fallback"}
	r := mustRouter(t, []Transport{primary, fallback}, RouterConfig{})

	if err := r.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if r.ActiveIndex() != 0 {
		t.Errorf("ActiveIndex = %d, want 0", r.ActiveIndex())
	}
	if fallback.connectCalls.Load() != 0 {
		t.Errorf("fallback dialed despite primary success: %d", fallback.connectCalls.Load())
	}
}

// TestRouter_Connect_FallsOverOnFailure verifies the linear
// fallback: transports[0] fails its only attempt, Router moves to
// transports[1] which succeeds.
func TestRouter_Connect_FallsOverOnFailure(t *testing.T) {
	primary := &fakeTransport{name: "primary", connectErr: errors.New("dead")}
	fallback := &fakeTransport{name: "fallback"}
	r := mustRouter(t, []Transport{primary, fallback}, RouterConfig{
		InitialBackoff: time.Millisecond,
		MaxAttempts:    1,
	})

	if err := r.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if r.ActiveIndex() != 1 {
		t.Errorf("ActiveIndex = %d, want 1 (fallback)", r.ActiveIndex())
	}
	if primary.connectCalls.Load() == 0 {
		t.Error("primary should have been dialed first")
	}
}

// TestRouter_Connect_AllFailAggregates verifies that every
// transport's error is captured in the joined output when no
// channel succeeds.
func TestRouter_Connect_AllFailAggregates(t *testing.T) {
	a := &fakeTransport{name: "a", connectErr: errors.New("dead-a")}
	b := &fakeTransport{name: "b", connectErr: errors.New("dead-b")}
	r := mustRouter(t, []Transport{a, b}, RouterConfig{
		InitialBackoff: time.Millisecond,
		MaxAttempts:    1,
	})

	err := r.Connect(context.Background())
	if err == nil {
		t.Fatal("expected aggregated failure error")
	}
	msg := err.Error()
	for _, want := range []string{"dead-a", "dead-b", "all channels failed"} {
		if !strings.Contains(msg, want) {
			t.Errorf("aggregated error missing %q\nfull: %s", want, msg)
		}
	}
}

// TestRouter_Connect_ContextCancel asserts ctx-cancellation aborts
// the retry/backoff loop and surfaces context.Canceled.
func TestRouter_Connect_ContextCancel(t *testing.T) {
	slow := &fakeTransport{name: "slow", connectErr: errors.New("retry-forever")}
	r := mustRouter(t, []Transport{slow}, RouterConfig{
		InitialBackoff: time.Hour, // would block forever without cancel
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Connect(ctx) }()
	// Let the first attempt run before cancelling.
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("got %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Connect did not return within 1s of cancel")
	}
}

// TestRouter_Connect_KillSwitchAborts proves the kill switch
// trips Connect before any dial happens and persists.
func TestRouter_Connect_KillSwitchAborts(t *testing.T) {
	primary := &fakeTransport{name: "primary"}
	var killed atomic.Bool
	killed.Store(true)
	r := mustRouter(t, []Transport{primary}, RouterConfig{
		KillSwitch: killed.Load,
	})

	if err := r.Connect(context.Background()); !errors.Is(err, ErrKilled) {
		t.Errorf("Connect: got %v, want ErrKilled", err)
	}
	if primary.connectCalls.Load() != 0 {
		t.Error("kill switch should have prevented any dial")
	}
}

// TestRouter_Read_DelegatesToActive verifies a connected Router
// forwards Read to the chosen transport.
func TestRouter_Read_DelegatesToActive(t *testing.T) {
	primary := &fakeTransport{name: "primary"}
	r := mustRouter(t, []Transport{primary}, RouterConfig{})
	if err := r.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buf[:n]) != "ok" {
		t.Errorf("Read got %q, want 'ok'", buf[:n])
	}
}

// TestRouter_Read_BeforeConnectErrors verifies the ErrChannelLost
// sentinel for pre-Connect Read.
func TestRouter_Read_BeforeConnectErrors(t *testing.T) {
	r := mustRouter(t, []Transport{&fakeTransport{name: "a"}}, RouterConfig{})
	_, err := r.Read(make([]byte, 1))
	if !errors.Is(err, ErrChannelLost) {
		t.Errorf("got %v, want ErrChannelLost", err)
	}
}

// TestRouter_Read_MarksLostOnError verifies a Read failure clears
// the active pointer; the next Read returns ErrChannelLost.
func TestRouter_Read_MarksLostOnError(t *testing.T) {
	primary := &fakeTransport{name: "primary", readErr: errors.New("conn reset")}
	r := mustRouter(t, []Transport{primary}, RouterConfig{})
	if err := r.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := r.Read(make([]byte, 1))
	if err == nil || err.Error() != "conn reset" {
		t.Fatalf("first Read err = %v, want 'conn reset'", err)
	}
	_, err = r.Read(make([]byte, 1))
	if !errors.Is(err, ErrChannelLost) {
		t.Errorf("second Read after lost: got %v, want ErrChannelLost", err)
	}
}

// TestRouter_Write_MarksLostOnError mirrors the Read test for the
// Write path.
func TestRouter_Write_MarksLostOnError(t *testing.T) {
	primary := &fakeTransport{name: "primary", writeErr: errors.New("broken pipe")}
	r := mustRouter(t, []Transport{primary}, RouterConfig{})
	if err := r.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Write([]byte("hi")); err == nil {
		t.Fatal("Write should have surfaced the writeErr")
	}
	if _, err := r.Write([]byte("hi")); !errors.Is(err, ErrChannelLost) {
		t.Errorf("Write after lost: got %v, want ErrChannelLost", err)
	}
}

// TestRouter_Close_TerminalAndIdempotent verifies Close kills the
// router permanently and is safe to call multiple times.
func TestRouter_Close_TerminalAndIdempotent(t *testing.T) {
	primary := &fakeTransport{name: "primary"}
	r := mustRouter(t, []Transport{primary}, RouterConfig{})
	if err := r.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if primary.closeCalls.Load() != 1 {
		t.Errorf("primary closed %d times, want 1", primary.closeCalls.Load())
	}
	if err := r.Close(); err != nil {
		t.Errorf("second Close (idempotent): %v", err)
	}
	if err := r.Connect(context.Background()); !errors.Is(err, ErrKilled) {
		t.Errorf("Connect after Close: got %v, want ErrKilled", err)
	}
}

// TestRouter_RemoteAddr_NilBeforeConnect / non-nil after.
func TestRouter_RemoteAddr_NilBeforeConnect(t *testing.T) {
	r := mustRouter(t, []Transport{&fakeTransport{name: "a"}}, RouterConfig{})
	if r.RemoteAddr() != nil {
		t.Error("RemoteAddr should be nil before Connect")
	}
	_ = r.Connect(context.Background())
	if r.RemoteAddr() == nil || r.RemoteAddr().String() != "a" {
		t.Errorf("RemoteAddr after Connect = %v, want fake('a')", r.RemoteAddr())
	}
}

// TestRouter_ActiveIndex_TracksFallover verifies the diagnostic
// matches the actual fallback progression.
func TestRouter_ActiveIndex_TracksFallover(t *testing.T) {
	a := &fakeTransport{name: "a", connectErr: errors.New("dead")}
	b := &fakeTransport{name: "b"}
	r := mustRouter(t, []Transport{a, b}, RouterConfig{
		InitialBackoff: time.Millisecond,
		MaxAttempts:    1,
	})
	if r.ActiveIndex() != -1 {
		t.Errorf("pre-Connect ActiveIndex = %d, want -1", r.ActiveIndex())
	}
	_ = r.Connect(context.Background())
	if r.ActiveIndex() != 1 {
		t.Errorf("post-fallover ActiveIndex = %d, want 1", r.ActiveIndex())
	}
}

// TestRouter_Connect_RetriesBeforeFallover verifies MaxAttempts is
// honoured per-transport before moving on.
func TestRouter_Connect_RetriesBeforeFallover(t *testing.T) {
	primary := &fakeTransport{name: "primary", connectErr: errors.New("flaky")}
	fallback := &fakeTransport{name: "fallback"}
	r := mustRouter(t, []Transport{primary, fallback}, RouterConfig{
		InitialBackoff: time.Millisecond,
		MaxAttempts:    3,
	})
	if err := r.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if primary.connectCalls.Load() != 3 {
		t.Errorf("primary tried %d times, want 3 (MaxAttempts)", primary.connectCalls.Load())
	}
	if r.ActiveIndex() != 1 {
		t.Errorf("ActiveIndex = %d, want 1", r.ActiveIndex())
	}
}

// TestRouter_NestedComposability is the composability proof: a
// Router wraps another Router (a "tier 2" fallback group) inside
// a "tier 1" preference. Because Router itself implements
// Transport, this is just one slice extra — no special wiring.
func TestRouter_NestedComposability(t *testing.T) {
	primary := &fakeTransport{name: "primary", connectErr: errors.New("dead-primary")}
	tier2A := &fakeTransport{name: "tier2A", connectErr: errors.New("dead-2a")}
	tier2B := &fakeTransport{name: "tier2B"}

	tier2, err := NewRouter([]Transport{tier2A, tier2B}, RouterConfig{
		InitialBackoff: time.Millisecond,
		MaxAttempts:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	top := mustRouter(t, []Transport{primary, tier2}, RouterConfig{
		InitialBackoff: time.Millisecond,
		MaxAttempts:    1,
	})

	if err := top.Connect(context.Background()); err != nil {
		t.Fatalf("nested Connect: %v", err)
	}
	if top.ActiveIndex() != 1 {
		t.Errorf("top.ActiveIndex = %d, want 1 (tier2)", top.ActiveIndex())
	}
	if tier2.ActiveIndex() != 1 {
		t.Errorf("tier2.ActiveIndex = %d, want 1 (tier2B)", tier2.ActiveIndex())
	}
}

// TestRouter_ConcurrentReadWriteSerialises confirms that two
// goroutines hammering Read + Write on the same Router don't
// race the internal mutex — the test runs with -race and fails
// on detection.
func TestRouter_ConcurrentReadWriteSerialises(t *testing.T) {
	r := mustRouter(t, []Transport{&fakeTransport{name: "a"}}, RouterConfig{})
	if err := r.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = r.Read(make([]byte, 4))
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = r.Write([]byte("ping"))
			}
		}()
	}
	wg.Wait()
}

// mustRouter is a t.Helper-friendly wrapper for tests that don't
// want to repeat the NewRouter + err-check boilerplate.
func mustRouter(t *testing.T, ts []Transport, cfg RouterConfig) *Router {
	t.Helper()
	r, err := NewRouter(ts, cfg)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return r
}

