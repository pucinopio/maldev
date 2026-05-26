package tui

// Coverage gaps closed in Session 0004 (batch 5):
//   - screen_audit.handleAuditInputResult (CSV + JSON export)
//   - screen_revocation.handleRevocationConfirmResult / handleRevocationInputResult
//   - screen_servers helpers: activeServerCount, serverCountLabel,
//     startAllCmd, stopAllCmd, sparklineRequests degenerate width

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/service"
)

// ── audit export handlers ────────────────────────────────────────────────────

// TestHandleAuditInputResult_CSVWritesFile fires the audit-export-csv result
// with a temp-file path, executes the cmd, and asserts the file exists +
// contains the CSV header.
func TestHandleAuditInputResult_CSVWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.csv")
	m := auditModel{}
	_, cmd := m.handleAuditInputResult(InputResultMsg{ID: OverlayIDAuditExportCSV, Value: path})
	if cmd == nil {
		t.Fatal("csv export must emit cmd")
	}
	_ = cmd()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if !strings.Contains(string(raw), "timestamp,kind") {
		t.Fatalf("csv missing header; got:\n%s", raw)
	}
}

// TestHandleAuditInputResult_JSONWritesFile — same pattern for the JSON exporter.
func TestHandleAuditInputResult_JSONWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	m := auditModel{}
	_, cmd := m.handleAuditInputResult(InputResultMsg{ID: OverlayIDAuditExportJSON, Value: path})
	if cmd == nil {
		t.Fatal("json export must emit cmd")
	}
	_ = cmd()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("json file not created: %v", err)
	}
}

// TestHandleAuditInputResult_UnknownIDNoop — unknown IDs return nil cmd.
func TestHandleAuditInputResult_UnknownIDNoop(t *testing.T) {
	m := auditModel{}
	_, cmd := m.handleAuditInputResult(InputResultMsg{ID: "audit-no-such", Value: ""})
	if cmd != nil {
		t.Fatal("unknown ID must return nil cmd")
	}
}

// ── revocation overlay-result handlers ────────────────────────────────────────

// TestHandleRevocationConfirmResult_NilSvcNoop — nil-svc must no-op.
func TestHandleRevocationConfirmResult_NilSvcNoop(t *testing.T) {
	m := revocationModel{}
	_, cmd := m.handleRevocationConfirmResult(ConfirmResultMsg{ID: "revocation-remove", Confirm: true})
	if cmd != nil {
		t.Fatal("nil-svc + confirmed must still be no-op (no row selected)")
	}
}

// TestHandleRevocationConfirmResult_NotConfirmedNoop — cancel path no-ops.
func TestHandleRevocationConfirmResult_NotConfirmedNoop(t *testing.T) {
	svc, _ := newTestServices(t)
	m := revocationModel{svc: svc}
	_, cmd := m.handleRevocationConfirmResult(ConfirmResultMsg{ID: "revocation-remove", Confirm: false})
	if cmd != nil {
		t.Fatal("Confirm:false must no-op")
	}
}

// TestHandleRevocationConfirmResult_UnknownIDNoop — wrong ID, no row selected.
func TestHandleRevocationConfirmResult_UnknownIDNoop(t *testing.T) {
	svc, _ := newTestServices(t)
	m := revocationModel{svc: svc}
	_, cmd := m.handleRevocationConfirmResult(ConfirmResultMsg{ID: "wrong-id", Confirm: true})
	if cmd != nil {
		t.Fatal("unknown ID must no-op")
	}
}

// TestHandleRevocationConfirmResult_UnrevokeRoundtrip seeds a revoked license,
// fires the unrevoke confirm, and asserts the row disappears.
func TestHandleRevocationConfirmResult_UnrevokeRoundtrip(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()

	iss, err := svc.Issuer.Generate(ctx, "rev-iss", "k-rev", "operator")
	if err != nil {
		t.Fatalf("seed Issuer.Generate: %v", err)
	}
	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID,
		Subject:  "user-to-unrevoke",
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "operator",
	})
	if err != nil {
		t.Fatalf("seed License.Issue: %v", err)
	}
	if err := svc.Revoke.Revoke(ctx, out.Row.ID, "test", "operator"); err != nil {
		t.Fatalf("seed Revoke: %v", err)
	}

	m := newRevocationModel(svc)
	m.width = 120
	m.hgt = 40
	views, err := svc.Revoke.ListRevoked(ctx)
	if err != nil {
		t.Fatalf("seed ListRevoked: %v", err)
	}
	m.rows = views
	m.rebuildTable()
	m.table.SetCursor(0)

	_, cmd := m.handleRevocationConfirmResult(ConfirmResultMsg{ID: "revocation-remove", Confirm: true})
	if cmd == nil {
		t.Fatal("revocation-remove with seeded svc must emit cmd")
	}
	msg := cmd()
	loaded, ok := msg.(RevocationLoadedMsg)
	if !ok {
		t.Fatalf("expected RevocationLoadedMsg, got %T", msg)
	}
	if loaded.Err != nil {
		t.Fatalf("Unrevoke failed: %v", loaded.Err)
	}
	if len(loaded.Rows) != 0 {
		t.Fatalf("rows after unrevoke = %d, want 0", len(loaded.Rows))
	}
}

// TestHandleRevocationInputResult_NilSvcNoop — nil-svc export no-ops.
func TestHandleRevocationInputResult_NilSvcNoop(t *testing.T) {
	m := revocationModel{}
	_, cmd := m.handleRevocationInputResult(InputResultMsg{ID: "revocation-export", Value: "/tmp/x"})
	if cmd != nil {
		t.Fatal("nil-svc must no-op")
	}
}

// TestHandleRevocationInputResult_UnknownIDNoop — unknown ID with wired svc.
func TestHandleRevocationInputResult_UnknownIDNoop(t *testing.T) {
	svc, _ := newTestServices(t)
	m := revocationModel{svc: svc}
	_, cmd := m.handleRevocationInputResult(InputResultMsg{ID: "wrong", Value: ""})
	if cmd != nil {
		t.Fatal("unknown ID must no-op")
	}
}

// TestHandleRevocationInputResult_ExportPublishesCRL — seeds an issuer (so the
// signing key is available), fires the export, asserts the CRL PEM is written
// to disk.
func TestHandleRevocationInputResult_ExportPublishesCRL(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, err := svc.Issuer.Generate(ctx, "rev-iss", "k-rev", "operator")
	if err != nil {
		t.Fatalf("seed Issuer: %v", err)
	}
	if err := svc.Issuer.SetActive(ctx, iss.ID, "operator"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	path := filepath.Join(t.TempDir(), "crl.pem")

	m := revocationModel{svc: svc}
	_, cmd := m.handleRevocationInputResult(InputResultMsg{ID: "revocation-export", Value: path})
	if cmd == nil {
		t.Fatal("export must emit cmd")
	}
	if msg := cmd(); msg != nil {
		// Could be pushOverlayMsg{newErrorOverlay(...)} — surface its body.
		if pom, ok := msg.(pushOverlayMsg); ok {
			t.Fatalf("export returned an error overlay: %v", pom.overlay.View())
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("crl file not created: %v", err)
	}
}

// ── server helpers ──────────────────────────────────────────────────────────

// TestActiveServerCount_CountsRunning — only Running:true contributes.
func TestActiveServerCount_CountsRunning(t *testing.T) {
	statuses := map[string]httpsrv.Status{
		"revocation": {Running: true},
		"heartbeat":  {Running: false},
		"probe":      {Running: true},
	}
	if got := activeServerCount(statuses); got != 2 {
		t.Fatalf("activeServerCount = %d, want 2", got)
	}
}

// TestActiveServerCount_AllStopped — empty / all-stopped → 0.
func TestActiveServerCount_AllStopped(t *testing.T) {
	if got := activeServerCount(map[string]httpsrv.Status{}); got != 0 {
		t.Fatalf("empty map = %d, want 0", got)
	}
}

// TestServerCountLabel_Format — "<N>/3 running".
func TestServerCountLabel_Format(t *testing.T) {
	statuses := map[string]httpsrv.Status{
		"a": {Running: true}, "b": {Running: true}, "c": {Running: false},
	}
	if got := serverCountLabel(statuses); got != "2/3 running" {
		t.Fatalf("serverCountLabel = %q, want '2/3 running'", got)
	}
}

// TestStartAllCmd_FiresOncePerServer — startAllCmd batches 3 Start calls.
func TestStartAllCmd_FiresOncePerServer(t *testing.T) {
	tc := &testCtrl{}
	cmd := startAllCmd(tc)
	if cmd == nil {
		t.Fatal("startAllCmd must return non-nil cmd")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c != nil {
				c()
			}
		}
	}
	if len(tc.starts) != 3 {
		t.Fatalf("starts = %v, want 3 entries", tc.starts)
	}
}

// TestStopAllCmd_FiresOncePerServer — stopAllCmd batches 3 Stop calls.
func TestStopAllCmd_FiresOncePerServer(t *testing.T) {
	tc := &testCtrl{}
	cmd := stopAllCmd(tc)
	if cmd == nil {
		t.Fatal("stopAllCmd must return non-nil cmd")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c != nil {
				c()
			}
		}
	}
	if len(tc.stops) != 3 {
		t.Fatalf("stops = %v, want 3 entries", tc.stops)
	}
}

// TestSparklineRequests_TooNarrowReturnsEmpty — width<8 must return "".
func TestSparklineRequests_TooNarrowReturnsEmpty(t *testing.T) {
	m := serversModel{}
	if got := m.sparklineRequests(0, 4); got != "" {
		t.Fatalf("narrow sparkline = %q, want empty", got)
	}
}
