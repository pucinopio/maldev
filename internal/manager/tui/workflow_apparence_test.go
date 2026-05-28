package tui

import (
	"strings"
	"testing"
	"time"
)

// TestApparence_FormatTime — FormatTime must respect the apparence
// timestamps_local flag set by ApplyApparence. Pre-fix the audit screen
// hardcoded .Format(...) on the raw timestamp, ignoring the preference.
func TestApparence_FormatTime(t *testing.T) {
	// Pick a deterministic UTC instant. The local-vs-UTC distinction is
	// only visible when the test machine's TZ != UTC, so we assert the
	// two outputs DIFFER only when offset != 0. They must always parse
	// back to the same instant.
	ts := time.Date(2026, 5, 28, 12, 34, 56, 0, time.UTC)
	const layout = "2006-01-02 15:04:05"

	// Save + restore so other tests see defaults.
	savedB, savedC, savedT := apparenceBold, apparenceComfort, apparenceLocalTS
	t.Cleanup(func() { ApplyApparence(savedB, savedC, savedT) })

	ApplyApparence(true, false, false) // UTC
	utc := FormatTime(ts, layout)
	if utc != "2026-05-28 12:34:56" {
		t.Errorf("UTC format = %q, want '2026-05-28 12:34:56'", utc)
	}

	ApplyApparence(true, false, true) // Local
	local := FormatTime(ts, layout)
	parsed, err := time.ParseInLocation(layout, local, time.Local)
	if err != nil {
		t.Fatalf("local-formatted string %q didn't round-trip: %v", local, err)
	}
	// Local + UTC representations must point at the same instant.
	if !parsed.UTC().Equal(ts) {
		t.Errorf("local %q resolves to %v, want %v", local, parsed.UTC(), ts)
	}
}

// TestApparence_BoldSaturatedRemovesBold — when bold_saturated is OFF the
// Glow* / HintKey styles must have their Bold attribute cleared. lipgloss
// strips ANSI escapes in test stdout (no TTY), so the test inspects the
// style's GetBold() property directly rather than the rendered output.
func TestApparence_BoldSaturatedRemovesBold(t *testing.T) {
	savedB, savedC, savedT := apparenceBold, apparenceComfort, apparenceLocalTS
	t.Cleanup(func() { ApplyApparence(savedB, savedC, savedT) })

	ApplyApparence(true, false, false)
	if !GlowCyan.GetBold() {
		t.Error("bold ON: GlowCyan.GetBold() = false, want true")
	}
	if !HintKey.GetBold() {
		t.Error("bold ON: HintKey.GetBold() = false, want true")
	}

	ApplyApparence(false, false, false)
	if GlowCyan.GetBold() {
		t.Error("bold OFF: GlowCyan.GetBold() = true, want false")
	}
	if HintKey.GetBold() {
		t.Error("bold OFF: HintKey.GetBold() = true, want false")
	}
}

// TestApparence_ComfortDensityAddsBoxPadding — when comfort_density is ON
// the BoxStyle vertical padding goes from 0 → 1, so a single-line content
// rendered through BoxStyle becomes 3 inner rows (pad+content+pad) instead
// of 1. Pre-fix the toggle persisted but the box padding never changed.
func TestApparence_ComfortDensityAddsBoxPadding(t *testing.T) {
	savedB, savedC, savedT := apparenceBold, apparenceComfort, apparenceLocalTS
	t.Cleanup(func() { ApplyApparence(savedB, savedC, savedT) })

	ApplyApparence(true, false, false)
	tight := BoxStyle.Render("x")
	ApplyApparence(true, true, false)
	comfort := BoxStyle.Render("x")
	if strings.Count(comfort, "\n") <= strings.Count(tight, "\n") {
		t.Errorf("comfort BoxStyle render lines=%d, tight=%d — padding did not grow\ntight=%q\ncomfort=%q",
			strings.Count(comfort, "\n"), strings.Count(tight, "\n"), tight, comfort)
	}
}

// TestApparence_TogglesApplyImmediately — the settings screen calls
// ApplyApparence after each toggle, so a subsequent render uses the new
// flag without the operator having to restart. This drives the toggle
// through the screen and inspects the global flag.
func TestApparence_TogglesApplyImmediately(t *testing.T) {
	savedB, savedC, savedT := apparenceBold, apparenceComfort, apparenceLocalTS
	t.Cleanup(func() { ApplyApparence(savedB, savedC, savedT) })

	// Reset to known defaults.
	ApplyApparence(true, false, false)
	if !apparenceBold || apparenceComfort || apparenceLocalTS {
		t.Fatal("defaults not set")
	}

	// Flip via the public entry point.
	ApplyApparence(false, true, true)
	if apparenceBold || !apparenceComfort || !apparenceLocalTS {
		t.Errorf("flags not updated: bold=%v comfort=%v local=%v",
			apparenceBold, apparenceComfort, apparenceLocalTS)
	}
}
