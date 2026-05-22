package tui_test

// teatest_smoke_test.go — end-to-end TUI tests using charmbracelet/x/exp/teatest.
// Unlike the snapshot tests that call View() directly, these drive a full
// tea.Program loop (Init → Update → View → re-render) so mouse, keyboard and
// async Cmd delivery are exercised through the real runtime.
//
// # Output reader constraints
//
// teatest wraps a *bytes.Buffer behind a safeReadWriter. Read() is destructive:
// bytes drained by one Read call are gone. A background goroutine reading the
// same reader races with and steals bytes from subsequent WaitFor calls.
//
// Strategy: use WaitFor directly on tm.Output() — each call accumulates bytes
// from the current read position forward. We issue Send() before each WaitFor
// so that new content is produced after the previous drain point. This matches
// the pattern in teatest's own app_test.go (TestAppInteractive).
//
// For navigation assertions we wait for strings that ONLY appear AFTER the
// navigation message is processed (diff-frame content), not strings already
// present in earlier frames.

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/oioio-space/maldev/internal/manager/tui"
)

const (
	testWidth  = 144
	testHeight = 44
)

// waitFor is a thin wrapper around teatest.WaitFor with project-standard timeouts.
// It waits up to 5 s, polling every 50 ms, for needle to appear in the output
// accumulated since the last WaitFor call drained the reader.
func waitFor(t *testing.T, tm *teatest.TestModel, needle string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte(needle)) },
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(5*time.Second),
	)
}

// sendAndWait sends msg to the program and waits for needle to appear in the
// output. The 20 ms pause before WaitFor starts lets the program goroutine
// enqueue and process the message, writing at least one diff frame, before
// the first io.ReadAll poll empties the buffer.
func sendAndWait(t *testing.T, tm *teatest.TestModel, msg tea.Msg, needle string) {
	t.Helper()
	tm.Send(msg)
	time.Sleep(20 * time.Millisecond)
	waitFor(t, tm, needle)
}

// TestTeatest_DashboardLoads exercises the full bubbletea program loop:
// Init → chrome renders → navigate to Licenses → back to Dashboard.
//
// Marker strategy: bubbletea v1.3+ uses frame-diff rendering. We only assert
// on strings that appear in the first FULL frame (tab strip, box titles) or
// in navigation diff frames (screen-specific body content). Dynamic content
// injected via DashboardSnapshotMsg is verified in the synchronous e2e tests
// (TestE2E_DashboardSnapshotPopulatesCounters etc.) which don't depend on the
// async diff-renderer.
func TestTeatest_DashboardLoads(t *testing.T) {
	m := tui.New(nil, nil, tui.SessionReady)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(testWidth, testHeight))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial full frame — tab strip and box titles render immediately.
	waitFor(t, tm, "Raccourcis")

	// Navigate to Licenses — "rechercher dans subject" appears in the diff frame.
	sendAndWait(t, tm, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}, "rechercher dans subject")

	// Return to dashboard — shortcuts box title re-appears in the diff.
	sendAndWait(t, tm, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}, "Raccourcis")

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
	_ = tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestTeatest_TabNavigation verifies that pressing keys 2-9 each write content
// unique to the newly active view into the output stream, and that returning to
// dashboard (key 1) re-renders the issuer key.
func TestTeatest_TabNavigation(t *testing.T) {
	// marker is text that appears in the diff frame produced when the navigation
	// key is processed. Use column headers / unique status-bar hints that are
	// only present in each screen's body — verified against actual test output.
	tabs := []struct {
		key    rune
		marker string
	}{
		{'2', "rechercher dans subject"},      // licenses: search hint in status bar
		{'3', "KEYID"},            // issuers: column header (NAME/KEYID/CREATED)
		{'4', "#SEALED"},          // recipients: column header (unique to recipients)
		{'5', "SHA256"},           // identities: column header
		{'6', "LICENSE"},          // revocation: column header
		{'7', "Fingerprint probe"}, // servers: sub-tab label unique to servers screen
		{'8', "TIMESTAMP"},        // audit: column header
		{'9', "default_issuer_name"}, // settings: field label unique to settings grid body
		{'1', "Raccourcis"}, // dashboard: shortcuts box title re-rendered in diff
	}

	m := tui.New(nil, nil, tui.SessionReady)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(testWidth, testHeight))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial full frame — box titles from the dashboard are present.
	waitFor(t, tm, "Raccourcis")

	for _, tc := range tabs {
		t.Run(string(tc.key), func(t *testing.T) {
			sendAndWait(t, tm, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}}, tc.marker)
		})
	}

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestTeatest_MouseClickDoesNotPanic is the teatest-runtime regression guard
// for mouse dispatch. It confirms the program survives a left-click on the
// Active tile without panicking. The precise synchronous assertion
// (SwitchToLicensesMsg emitted) lives in e2e_smoke_test.go.
func TestTeatest_MouseClickDoesNotPanic(t *testing.T) {
	m := tui.New(nil, nil, tui.SessionReady)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(testWidth, testHeight))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for the full frame, then click a tile and confirm the program
	// stays alive (no panic, continues to render).
	waitFor(t, tm, "Raccourcis")

	// Click inside the first tile (Active), X=17 Y=4.
	// At 144-col layout with 5 equal tiles, tile 0 spans roughly X=0..28.
	sendAndWait(t, tm, tea.MouseMsg{
		X:      17,
		Y:      4,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		Type:   tea.MouseLeft,
	}, "rechercher dans subject")

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
