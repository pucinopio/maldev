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
	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
)

const (
	testWidth  = 144
	testHeight = 44
)

// dashboardSnap is a pre-populated snapshot injected directly so these tests
// need no real DB or services.
var dashboardSnap = cmds.DashboardSnapshotMsg{
	Active:               47,
	Revoked:              6,
	Expired:              12,
	ExpiringSoon:         4,
	ActiveKeyID:          "k2026-04",
	ActiveKeyName:        "rshell-prod-2026Q2",
	ActiveKeyFingerprint: "4a:2f:88:d1:09:cc:fe:b3:72:1e:aa:5d:03:89:c0:f7",
	Servers: []cmds.ServerStatus{
		{Name: "revocation", On: true, URL: ":8443", Requests: 142},
		{Name: "heartbeat", On: true, URL: ":8444", Requests: 87},
		{Name: "probe", On: false, URL: ":8445"},
	},
	RecentAudit: []cmds.AuditEntry{
		{Kind: "license.issue", TargetID: "lic-e7a1", Actor: "operator"},
		{Kind: "license.revoke", TargetID: "lic-9d22", Actor: "operator"},
		{Kind: "server.start", TargetID: "revocation", Actor: "operator"},
	},
}

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
// Init → DashboardSnapshotMsg → key ID appears → navigate to Licenses → back.
//
// Marker strategy: bubbletea v1.3+ uses frame-diff rendering; only changed
// cells appear in each output frame. We wait for strings that first appear in
// the diff frame produced by each Send call, ensuring a forward-only search
// on the unconsumed part of the output buffer matches new content.
//
// "Dashboard" (tab strip) reliably appears in the first full frame.
// "k2026-04" appears in the diff frame when DashboardSnapshotMsg is processed.
// "/ to search" appears when the Licenses screen first renders.
// "rshell-prod-2026Q2" appears in the diff frame on return to dashboard.
func TestTeatest_DashboardLoads(t *testing.T) {
	m := tui.New(nil, nil, tui.SessionReady)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(testWidth, testHeight))
	t.Cleanup(func() { _ = tm.Quit() })

	// Initial full frame: tab strip writes "Dashboard" without ANSI fragmentation.
	waitFor(t, tm, "Dashboard")

	// Inject seeded data. The diff frame writes the key ID for the first time.
	tm.Send(dashboardSnap)
	waitFor(t, tm, "k2026-04")

	// Navigate to Licenses. The diff frame writes "/ to search" (unique to licenses body).
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	waitFor(t, tm, "/ to search")

	// Return to dashboard. Diff frame re-renders key name.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	waitFor(t, tm, "rshell-prod-2026Q2")

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
		{'2', "/ to search"},      // licenses: search hint in status bar
		{'3', "KEYID"},            // issuers: column header (NAME/KEYID/CREATED)
		{'4', "#SEALED"},          // recipients: column header (unique to recipients)
		{'5', "SHA256"},           // identities: column header
		{'6', "LICENSE"},          // revocation: column header
		{'7', "[s] Start"},        // servers: button label unique to servers screen
		{'8', "TIMESTAMP"},        // audit: column header
		{'9', "Settings"},         // settings: screen title in body
		{'1', "rshell-prod-2026Q2"}, // dashboard: key name re-rendered in diff
	}

	m := tui.New(nil, nil, tui.SessionReady)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(testWidth, testHeight))
	t.Cleanup(func() { _ = tm.Quit() })

	// Initial render then seed. Use sendAndWait to ensure the diff frame is
	// written before WaitFor begins polling the drained buffer.
	waitFor(t, tm, "Dashboard")
	sendAndWait(t, tm, dashboardSnap, "k2026-04")

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
// Active tile and that the navigation to Licenses completes cleanly.
// The precise synchronous assertion (SwitchToLicensesMsg emitted) lives in
// e2e_smoke_test.go.
func TestTeatest_MouseClickDoesNotPanic(t *testing.T) {
	m := tui.New(nil, nil, tui.SessionReady)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(testWidth, testHeight))
	t.Cleanup(func() { _ = tm.Quit() })

	waitFor(t, tm, "Dashboard")
	sendAndWait(t, tm, dashboardSnap, "k2026-04")

	// Click inside the first tile (Active), X=17 Y=4 — within the tile body
	// at 144-col layout (tile 0 spans roughly X=0..35).
	sendAndWait(t, tm, tea.MouseMsg{
		X:      17,
		Y:      4,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		Type:   tea.MouseLeft,
	}, "/ to search")

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
