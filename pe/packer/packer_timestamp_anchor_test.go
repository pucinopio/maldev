package packer

import (
	"testing"
	"time"
)

// TestDeterministicTimestampAnchor_NotTooStale enforces the
// "bump at minor-release boundaries" promise from
// [deterministicTimestampAnchor]'s doc comment. The anchor drives
// reproducible-build TimeDateStamps when [PackBinaryOptions.Seed]
// is non-zero — if it drifts too far behind wall clock, generated
// stamps fall outside the plausible "recently linked" window and
// become the same threat-intel pivot signal the randomisation is
// meant to defeat. Two-year grace gives a comfortable release
// cadence without inviting silent rot.
func TestDeterministicTimestampAnchor_NotTooStale(t *testing.T) {
	const twoYears = int64(2 * 365 * 24 * 3600)
	drift := time.Now().Unix() - int64(deterministicTimestampAnchor)
	if drift > twoYears {
		t.Errorf("deterministicTimestampAnchor is %d seconds behind wall clock (>2y) — "+
			"bump it to a recent epoch in packer.go", drift)
	}
}
