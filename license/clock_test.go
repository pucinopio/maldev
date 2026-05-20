package license

import (
	"testing"
	"time"
)

func TestRealClockNonZero(t *testing.T) {
	var c realClock
	if c.Now().IsZero() {
		t.Fatal("realClock returned zero time")
	}
}

func TestFakeClockUsable(t *testing.T) {
	ref := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := &FakeClock{T: ref}
	if !c.Now().Equal(ref) {
		t.Fatal("FakeClock did not return configured time")
	}
}
