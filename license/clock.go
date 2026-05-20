package license

import "time"

// Clock is the abstraction used by Verify for all time-dependent checks.
// Inject a FakeClock in tests; production uses realClock via the default.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

// FakeClock returns a fixed time. Tests may mutate T between calls.
type FakeClock struct {
	T time.Time
}

func (f *FakeClock) Now() time.Time { return f.T }
