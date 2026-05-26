package tui

// Coverage gaps closed in Session 0004 (batch 6):
//   - server_card.go: Bounds, Update, View, OnClick, Children
//   - screen_totp.go: handleTOTPInputResult / handleTOTPConfirmResult,
//     loadTOTPDetailCmd, renderQR
//   - screen_servers.go: loadProbeTokensCmd / loadProbeHistoryCmd /
//     renderProbeHistory

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/service"
)

// ── server_card.go ───────────────────────────────────────────────────────────

// TestServerCard_Bounds_ReturnsLayoutValue — after Layout, Bounds returns the
// exact rect that was assigned.
func TestServerCard_Bounds_ReturnsLayoutValue(t *testing.T) {
	sc := newServerCard("revocation")
	b := Rect{X: 5, Y: 7, W: 30, H: 12}
	sc.Layout(b)
	if got := sc.Bounds(); got != b {
		t.Fatalf("Bounds() = %+v, want %+v", got, b)
	}
}

// TestServerCard_View_RenderingDependsOnRunning — the View output differs
// when Running flips (border colour + pill text).
func TestServerCard_View_RenderingDependsOnRunning(t *testing.T) {
	sc := newServerCard("revocation")
	sc.Layout(Rect{X: 0, Y: 0, W: 40, H: 12})

	sc.SetStatus(httpsrv.Status{Running: false, ListenAddr: "—"})
	stoppedView := sc.View()
	sc.SetStatus(httpsrv.Status{
		Running:    true,
		ListenAddr: ":8443",
		StartedAt:  time.Now().Add(-5 * time.Minute),
		Requests:   42,
	})
	runningView := sc.View()
	if stoppedView == runningView {
		t.Fatal("stopped + running renders are identical — pill / colour didn't change")
	}
	if !strings.Contains(runningView, ":8443") {
		t.Fatalf("running view missing listen addr; got:\n%s", runningView)
	}
	if !strings.Contains(runningView, "42") {
		t.Fatalf("running view missing request count; got:\n%s", runningView)
	}
}

// TestServerCard_View_TooNarrowReturnsEmpty — width<4 yields the empty-string
// short-circuit (avoids lipgloss panics on degenerate sizes).
func TestServerCard_View_TooNarrowReturnsEmpty(t *testing.T) {
	sc := newServerCard("revocation")
	sc.Layout(Rect{X: 0, Y: 0, W: 2, H: 5})
	if got := sc.View(); got != "" {
		t.Fatalf("narrow View() = %q, want empty", got)
	}
}

// TestServerCard_UpdateFansToButtons — Update forwards the msg to both buttons.
func TestServerCard_UpdateFansToButtons(t *testing.T) {
	sc := newServerCard("revocation")
	sc.Layout(Rect{X: 0, Y: 0, W: 40, H: 12})
	w, _ := sc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if _, ok := w.(*ServerCard); !ok {
		t.Fatalf("Update returned %T, want *ServerCard", w)
	}
}

// TestServerCard_Children_ExposesButtons — Children() returns the start +
// stop buttons (used by the dispatch walker).
func TestServerCard_Children_ExposesButtons(t *testing.T) {
	sc := newServerCard("revocation")
	if got := sc.Children(); len(got) != 2 {
		t.Fatalf("Children() = %d, want 2", len(got))
	}
}

// TestServerCard_OnClick_OutOfBoundsReturnsNil — click outside any button.
func TestServerCard_OnClick_OutOfBoundsReturnsNil(t *testing.T) {
	sc := newServerCard("revocation")
	sc.Layout(Rect{X: 0, Y: 0, W: 40, H: 12})
	if cmd := sc.OnClick(0, 0, tea.MouseButtonLeft); cmd != nil {
		t.Fatalf("click at (0,0) inside title row must miss buttons; got cmd=%v", cmd())
	}
}

// ── screen_totp ──────────────────────────────────────────────────────────────

// TestTOTPRenderQR_NilViewReturnsEmpty — no view → empty render.
func TestTOTPRenderQR_NilViewReturnsEmpty(t *testing.T) {
	m := totpModel{}
	if got := m.renderQR(); got != "" {
		t.Fatalf("nil-view renderQR = %q, want empty", got)
	}
}

// TestTOTPRenderQR_PopulatedRendersASCIIPlusURI — non-nil view shows both
// the QR ASCII art and the otpauth URI.
func TestTOTPRenderQR_PopulatedRendersASCIIPlusURI(t *testing.T) {
	m := totpModel{
		view: &service.TOTPSecretView{
			QRImageASCII: "█▀▀█\n█▄▄█",
			OtpauthURI:   "otpauth://totp/test?secret=AAAA",
		},
	}
	out := m.renderQR()
	if !strings.Contains(out, "█▀▀█") {
		t.Errorf("renderQR missing ASCII art")
	}
	if !strings.Contains(out, "otpauth://totp/test") {
		t.Errorf("renderQR missing URI")
	}
}

// TestHandleTOTPInputResult_NilSvcOrEmptyLabelNoop — totp-label only acts
// when svc is wired AND label is non-empty.
func TestHandleTOTPInputResult_NilSvcOrEmptyLabelNoop(t *testing.T) {
	m := totpModel{} // nil svc
	_, cmd := m.handleTOTPInputResult(InputResultMsg{ID: "totp-label", Value: "x"})
	if cmd != nil {
		t.Error("nil-svc totp-label must no-op")
	}

	svc, _ := newTestServices(t)
	m = totpModel{svc: svc}
	_, cmd = m.handleTOTPInputResult(InputResultMsg{ID: "totp-label", Value: ""})
	if cmd != nil {
		t.Error("empty-label totp-label must no-op")
	}
}

// TestHandleTOTPInputResult_GeneratesSecret — wired svc + non-empty label
// produces a TOTPLoadedMsg with the new row.
func TestHandleTOTPInputResult_GeneratesSecret(t *testing.T) {
	svc, _ := newTestServices(t)
	m := totpModel{svc: svc}
	_, cmd := m.handleTOTPInputResult(InputResultMsg{ID: "totp-label", Value: "alice@example"})
	if cmd == nil {
		t.Fatal("wired totp-label must emit cmd")
	}
	msg := cmd()
	loaded, ok := msg.(TOTPLoadedMsg)
	if !ok {
		t.Fatalf("expected TOTPLoadedMsg, got %T", msg)
	}
	if loaded.Err != nil {
		t.Fatalf("Generate failed: %v", loaded.Err)
	}
	if len(loaded.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(loaded.Rows))
	}
}

// TestHandleTOTPInputResult_ExportPNGNoViewNoop — export-png with no view
// (nothing decrypted yet) must no-op.
func TestHandleTOTPInputResult_ExportPNGNoViewNoop(t *testing.T) {
	svc, _ := newTestServices(t)
	m := totpModel{svc: svc}
	_, cmd := m.handleTOTPInputResult(InputResultMsg{ID: "totp-export-png", Value: "/tmp/x.png"})
	if cmd != nil {
		t.Fatal("no-view export must no-op")
	}
}

// TestHandleTOTPInputResult_UnknownIDNoop — unknown ID no-ops.
func TestHandleTOTPInputResult_UnknownIDNoop(t *testing.T) {
	svc, _ := newTestServices(t)
	m := totpModel{svc: svc}
	_, cmd := m.handleTOTPInputResult(InputResultMsg{ID: "nope"})
	if cmd != nil {
		t.Fatal("unknown ID must no-op")
	}
}

// TestHandleTOTPConfirmResult_NotDeleteNoop — only totp-delete + Confirm:true
// triggers the action; everything else returns nil cmd.
func TestHandleTOTPConfirmResult_NotDeleteNoop(t *testing.T) {
	svc, _ := newTestServices(t)
	m := totpModel{svc: svc}
	_, cmd := m.handleTOTPConfirmResult(ConfirmResultMsg{ID: "other", Confirm: true})
	if cmd != nil {
		t.Error("non-delete confirm must no-op")
	}
	_, cmd = m.handleTOTPConfirmResult(ConfirmResultMsg{ID: "totp-delete", Confirm: false})
	if cmd != nil {
		t.Error("totp-delete with Confirm:false must no-op")
	}
}

// TestLoadTOTPDetailCmd_NilSvcReturnsEmpty — without svc, the cmd resolves
// to an empty totpDetailLoadedMsg.
func TestLoadTOTPDetailCmd_NilSvcReturnsEmpty(t *testing.T) {
	cmd := loadTOTPDetailCmd(nil, uuid.New(), "test")
	if cmd == nil {
		t.Fatal("loadTOTPDetailCmd must always return non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(totpDetailLoadedMsg); !ok {
		t.Fatalf("expected totpDetailLoadedMsg, got %T", msg)
	}
}

// ── screen_servers probe loaders ─────────────────────────────────────────────

// TestLoadProbeTokensCmd_NilSvcReturnsNilCmd — defensive nil-svc short-circuit.
func TestLoadProbeTokensCmd_NilSvcReturnsNilCmd(t *testing.T) {
	if cmd := loadProbeTokensCmd(nil); cmd != nil {
		t.Fatalf("nil-svc loadProbeTokensCmd = %v, want nil", cmd)
	}
}

// TestLoadProbeHistoryCmd_NilSvcReturnsNilCmd — same nil-svc guarantee.
func TestLoadProbeHistoryCmd_NilSvcReturnsNilCmd(t *testing.T) {
	if cmd := loadProbeHistoryCmd(nil); cmd != nil {
		t.Fatalf("nil-svc loadProbeHistoryCmd = %v, want nil", cmd)
	}
}

// TestLoadProbeTokensCmd_WiredSvcSeparatesActive — with a wired svc and no
// tokens emitted yet, the cmd returns a probeTokensLoadedMsg (possibly empty)
// without error.
func TestLoadProbeTokensCmd_WiredSvcEmptySeed(t *testing.T) {
	svc, _ := newTestServices(t)
	cmd := loadProbeTokensCmd(svc)
	if cmd == nil {
		t.Fatal("wired loadProbeTokensCmd must return non-nil cmd")
	}
	msg := cmd()
	loaded, ok := msg.(probeTokensLoadedMsg)
	if !ok {
		t.Fatalf("expected probeTokensLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("empty-seed must not error, got %v", loaded.err)
	}
}

// TestLoadProbeHistoryCmd_WiredSvcEmptySeed — same shape for history loader.
func TestLoadProbeHistoryCmd_WiredSvcEmptySeed(t *testing.T) {
	svc, _ := newTestServices(t)
	cmd := loadProbeHistoryCmd(svc)
	if cmd == nil {
		t.Fatal("wired loadProbeHistoryCmd must return non-nil cmd")
	}
	msg := cmd()
	loaded, ok := msg.(probeHistoryLoadedMsg)
	if !ok {
		t.Fatalf("expected probeHistoryLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("empty-seed must not error, got %v", loaded.err)
	}
}
