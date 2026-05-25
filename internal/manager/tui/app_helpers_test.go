package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
)

// httpsrvEventStub builds a deterministic Event for ring-buffer tests.
func httpsrvEventStub(i int) httpsrv.Event {
	return httpsrv.Event{Server: "test", Kind: "tick", Note: fmt.Sprintf("ev-%d", i)}
}

// TestRootModel_OnboardingResult — nil before completion, points to the
// captured msg after the wizard fires OnboardingDoneMsg.
func TestRootModel_OnboardingResult(t *testing.T) {
	m := New(nil, nil, SessionOnboarding)
	if got := m.OnboardingResult(); got != nil {
		t.Errorf("fresh model: OnboardingResult() = %+v, want nil", got)
	}
	// Inject a Done msg through Update and confirm the result is stashed.
	want := OnboardingDoneMsg{Passphrase: "p", IssuerName: "issuer", Skipped: true}
	updated, _ := m.Update(want)
	r := updated.(rootModel)
	got := r.OnboardingResult()
	if got == nil {
		t.Fatal("after OnboardingDoneMsg: OnboardingResult() = nil, want non-nil")
	}
	if got.Passphrase != want.Passphrase || got.IssuerName != want.IssuerName || !got.Skipped {
		t.Errorf("OnboardingResult() = %+v, want %+v", *got, want)
	}
}

// TestAnyServerRunning_NilBundle — returns false (no panic).
func TestAnyServerRunning_NilBundle(t *testing.T) {
	if anyServerRunning(nil) {
		t.Error("anyServerRunning(nil) = true, want false")
	}
}

// TestBundleAsController_NilSafe — returns the typed-nil interface
// equivalent so dispatch doesn't panic on missing bundle.
func TestBundleAsController_NilSafe(t *testing.T) {
	got := bundleAsController(nil)
	if got != nil {
		t.Errorf("bundleAsController(nil) = %T, want nil", got)
	}
}

// TestNextView walks through the 10 tab positions and wraps both forward
// and backward.
func TestNextView_Wraps(t *testing.T) {
	last := viewOrder[len(viewOrder)-1]
	if got := nextView(last, +1); got != viewOrder[0] {
		t.Errorf("forward wrap from %s: got %s, want %s", last, got, viewOrder[0])
	}
	if got := nextView(viewOrder[0], -1); got != last {
		t.Errorf("backward wrap from %s: got %s, want %s", viewOrder[0], got, last)
	}
	if got := nextView(viewOrder[0], +1); got != viewOrder[1] {
		t.Errorf("forward from %s: got %s, want %s", viewOrder[0], got, viewOrder[1])
	}
}

// TestEventRing — append wraps after eventRingCap entries; replay yields
// them in oldest-first order so the live log retains chronological order.
func TestEventRing_WrapAndReplay(t *testing.T) {
	m := New(nil, nil, SessionReady)
	// Fill the ring with eventRingCap + 5 events; ring should keep only the
	// most recent eventRingCap.
	for i := 0; i < eventRingCap+5; i++ {
		m.appendEventRing(httpsrvEventStub(i))
	}
	if len(m.eventRing) != eventRingCap {
		t.Errorf("ring length = %d, want eventRingCap=%d", len(m.eventRing), eventRingCap)
	}
	// Replay should yield exactly eventRingCap msgs.
	replay := m.replayEventRing()
	if replay == nil {
		t.Fatal("replayEventRing returned nil for non-empty ring")
	}
	batch := replay().(tea.BatchMsg)
	if len(batch) != eventRingCap {
		t.Errorf("replay batch len = %d, want %d", len(batch), eventRingCap)
	}
}
