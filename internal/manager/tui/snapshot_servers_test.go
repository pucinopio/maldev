package tui_test

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/tui"
)

// ── mock controller ────────────────────────────────────────────────────────────

// mockController implements httpsrv.Controller for tests. It records Start/Stop
// calls and returns pre-set statuses.
type mockController struct {
	statuses  map[string]httpsrv.Status
	startCalls []string
	stopCalls  []string
	merged    chan httpsrv.Event
}

func newMockController() *mockController {
	return &mockController{
		statuses: map[string]httpsrv.Status{
			"revocation": {Running: true, ListenAddr: "127.0.0.1:8443", StartedAt: time.Now().Add(-5 * time.Minute), Requests: 12},
			"heartbeat":  {Running: true, ListenAddr: "127.0.0.1:8444", StartedAt: time.Now().Add(-3 * time.Minute), Requests: 4},
			"probe":      {Running: false, LastError: "bind: address already in use"},
		},
		merged: make(chan httpsrv.Event, 64),
	}
}

func (mc *mockController) Start(_ context.Context, name string) error {
	mc.startCalls = append(mc.startCalls, name)
	if s, ok := mc.statuses[name]; ok {
		s.Running = true
		mc.statuses[name] = s
	}
	return nil
}

func (mc *mockController) Stop(name string) error {
	mc.stopCalls = append(mc.stopCalls, name)
	if s, ok := mc.statuses[name]; ok {
		s.Running = false
		mc.statuses[name] = s
	}
	return nil
}

func (mc *mockController) Statuses() map[string]httpsrv.Status {
	out := make(map[string]httpsrv.Status, len(mc.statuses))
	for k, v := range mc.statuses {
		out[k] = v
	}
	return out
}

func (mc *mockController) MergedEvents() <-chan httpsrv.Event { return mc.merged }

// ── helpers ────────────────────────────────────────────────────────────────────

// sampleEvents returns five events of varying kinds for log snapshot tests.
func sampleEvents() []httpsrv.Event {
	base := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	return []httpsrv.Event{
		{At: base, Server: "revocation", Kind: "started", Note: "127.0.0.1:8443"},
		{At: base.Add(time.Second), Server: "heartbeat", Kind: "started", Note: "127.0.0.1:8444"},
		{At: base.Add(2 * time.Second), Server: "revocation", Kind: "request", Method: "GET", Path: "/revoked.pem", Status: 200, Remote: "10.0.0.1:52000"},
		{At: base.Add(3 * time.Second), Server: "probe", Kind: "error", Note: "bind: address already in use"},
		{At: base.Add(4 * time.Second), Server: "heartbeat", Kind: "request", Method: "POST", Path: "/heartbeat", Status: 401, Remote: "10.0.0.2:41000"},
	}
}

// buildServersRoot builds a root model with a mock controller wired.
func buildServersRoot(mc *mockController) tea.Model {
	// tui.New accepts *httpsrv.Bundle, not Controller — for snapshot tests we
	// navigate to the servers screen and inject serverEventMsg directly via
	// the exported New constructor with a nil bundle (no fan-in needed), then
	// directly update the servers field via messages.
	// For the routing test we use a separate helper that builds the model
	// and injects start/stop messages.
	root := tui.New(nil, nil, tui.SessionReady)
	return root
}

// ── snapshot tests ─────────────────────────────────────────────────────────────

func TestServerCardSnapshot_Running(t *testing.T) {
	root := buildServersRoot(nil)
	m := initModel(root)
	// Navigate to Servers screen (tab 7).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("7")})
	// Inject a "started" event for revocation so card shows running state.
	ev := httpsrv.Event{
		At:     time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC),
		Server: "revocation",
		Kind:   "started",
		Note:   "127.0.0.1:8443",
	}
	m, _ = m.Update(tui.ServerEventMsg(ev))
	compareOrUpdate(t, "server_card_running", m.View())
}

func TestServerCardSnapshot_Stopped(t *testing.T) {
	root := buildServersRoot(nil)
	m := initModel(root)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("7")})
	// Inject an error event for probe to show stopped + last error.
	ev := httpsrv.Event{
		At:     time.Date(2026, 5, 21, 12, 0, 5, 0, time.UTC),
		Server: "probe",
		Kind:   "error",
		Note:   "bind: address already in use",
	}
	m, _ = m.Update(tui.ServerEventMsg(ev))
	compareOrUpdate(t, "server_card_stopped", m.View())
}

func TestServerLogSnapshot(t *testing.T) {
	root := buildServersRoot(nil)
	m := initModel(root)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("7")})
	for _, ev := range sampleEvents() {
		m, _ = m.Update(tui.ServerEventMsg(ev))
	}
	compareOrUpdate(t, "server_log", m.View())
}

func TestServersScreenSnapshot(t *testing.T) {
	root := buildServersRoot(nil)
	m := initModel(root)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("7")})
	for _, ev := range sampleEvents() {
		m, _ = m.Update(tui.ServerEventMsg(ev))
	}
	compareOrUpdate(t, "servers_screen", m.View())
}

// ── routing test ───────────────────────────────────────────────────────────────

func TestServersScreen_StartStopRouting(t *testing.T) {
	mc := newMockController()

	// Build a servers model directly and drive it.
	m := tui.NewServersModelForTest(mc)
	m = tui.InitServersModel(m, 120, 40)

	// Simulate the "Start" button message for revocation.
	m, _ = tui.UpdateServersModel(m, tui.ServerStartMsg("revocation"))

	if len(mc.startCalls) != 1 || mc.startCalls[0] != "revocation" {
		t.Fatalf("expected Start(revocation), got startCalls=%v", mc.startCalls)
	}

	// Simulate the "Stop" button message for heartbeat.
	m, _ = tui.UpdateServersModel(m, tui.ServerStopMsg("heartbeat"))

	if len(mc.stopCalls) != 1 || mc.stopCalls[0] != "heartbeat" {
		t.Fatalf("expected Stop(heartbeat), got stopCalls=%v", mc.stopCalls)
	}
	_ = m
}
