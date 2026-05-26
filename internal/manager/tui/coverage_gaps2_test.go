package tui

// Coverage gaps closed in Session 0004 (batch 2):
//   - overlay_ok.Init / overlay_quit.Init
//   - overlay_qr.handleMouse (save / copy / dismiss strips) + saveCmd
//   - screen_audit.renderPayload + auditKindFilter.String
//   - screen_licenses.renderDetail{PEM,Audit,Chain,Bindings}

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// ── overlay Init no-ops ──────────────────────────────────────────────────────

// TestOverlayOK_InitReturnsNil — okOverlay has no async setup; Init() must
// return nil.
func TestOverlayOK_InitReturnsNil(t *testing.T) {
	o := NewOKOverlay("done", "thing was done")
	if cmd := o.Init(); cmd != nil {
		t.Fatalf("okOverlay.Init() must return nil, got non-nil")
	}
}

// TestOverlayQuit_InitReturnsNil — same guarantee for the quit modal.
func TestOverlayQuit_InitReturnsNil(t *testing.T) {
	o := newQuitOverlay(true)
	if cmd := o.Init(); cmd != nil {
		t.Fatal("quitOverlay.Init() must return nil")
	}
}

// ── overlay_qr handleMouse paths ─────────────────────────────────────────────

// fixtureIssuedLicense returns a minimal *service.IssuedLicense suitable for
// rendering the QR overlay (PEM populated, no TOTP secrets).
func fixtureIssuedLicense() *service.IssuedLicense {
	return &service.IssuedLicense{
		PEM: []byte("-----BEGIN LICENSE-----\nlicfixture\n-----END LICENSE-----\n"),
		Row: &ent.License{LicenseUUID: "fixture-uuid"},
	}
}

// TestQROverlay_HandleMouse_SaveLine — clicking on the "save" strip emits the
// saveCmd path (which downstream produces QRSavedMsg).
func TestQROverlay_HandleMouse_SaveLine(t *testing.T) {
	ov := newQROverlay(fixtureIssuedLicense())
	lines := strings.Split(ov.View(), "\n")
	saveY := -1
	for i, l := range lines {
		if strings.Contains(l, "save") && !strings.Contains(l, "saved") {
			saveY = i
			break
		}
	}
	if saveY < 0 {
		t.Skip("no 'save' strip in current QR overlay layout — skipping click test")
	}
	cmd := ov.handleMouse(0, saveY)
	if cmd == nil {
		t.Fatal("click on save strip must return cmd")
	}
}

// TestQROverlay_HandleMouse_CopyLine — clicking on the "copy PEM" strip does
// the clipboard write inline and returns nil (no follow-up msg needed).
func TestQROverlay_HandleMouse_CopyLine(t *testing.T) {
	ov := newQROverlay(fixtureIssuedLicense())
	lines := strings.Split(ov.View(), "\n")
	copyY := -1
	for i, l := range lines {
		if strings.Contains(l, "copy PEM") {
			copyY = i
			break
		}
	}
	if copyY < 0 {
		t.Skip("no 'copy PEM' strip — skipping")
	}
	// We don't assert clipboard side-effect (no clipboard in CI); just
	// that the call doesn't panic.
	_ = ov.handleMouse(0, copyY)
}

// TestQROverlay_HandleMouse_OutOfBoundsDismisses — clicking above row 0 or
// past the last line dismisses the overlay.
func TestQROverlay_HandleMouse_OutOfBoundsDismisses(t *testing.T) {
	ov := newQROverlay(fixtureIssuedLicense())
	cmd := ov.handleMouse(0, -1)
	if cmd == nil {
		t.Fatal("out-of-bounds click must emit dismiss cmd")
	}
	msg := cmd()
	done, ok := msg.(OverlayDoneMsg)
	if !ok {
		t.Fatalf("expected OverlayDoneMsg, got %T", msg)
	}
	if done.Result != nil {
		t.Fatalf("dismiss must emit nil Result, got %v", done.Result)
	}
}

// TestQROverlay_SaveCmdNilLicenseReturnsError — calling saveCmd on an overlay
// that has no licence yields a QRSavedMsg with an error.
func TestQROverlay_SaveCmdNilLicenseReturnsError(t *testing.T) {
	ov := newQROverlay(nil)
	msg := ov.saveCmd()()
	saved, ok := msg.(QRSavedMsg)
	if !ok {
		t.Fatalf("expected QRSavedMsg, got %T", msg)
	}
	if saved.Err == nil {
		t.Fatal("saveCmd with nil licence must emit error")
	}
}

// ── auditKindFilter.String ───────────────────────────────────────────────────

// TestAuditKindFilterString_NonEmpty — every filter constant has a non-empty
// label that round-trips through String().
func TestAuditKindFilterString_NonEmpty(t *testing.T) {
	for _, f := range []auditKindFilter{
		auditFilterAll, auditFilterLicense, auditFilterKey,
		auditFilterServer, auditFilterIdentity, auditFilterProbe,
	} {
		if s := f.String(); s == "" {
			t.Errorf("filter %d returned empty label", f)
		}
	}
}

// ── screen_audit renderPayload ───────────────────────────────────────────────

// TestAuditRenderPayload_IncludesAllMetadata renders a fully-populated audit
// event and verifies every metadata field appears in the output.
func TestAuditRenderPayload_IncludesAllMetadata(t *testing.T) {
	m := auditModel{}
	row := &ent.AuditEvent{
		ID:         uuid.MustParse("11111111-2222-3333-4444-555555555555"),
		Kind:       "license.issue",
		Actor:      "operator",
		TargetKind: "License",
		TargetID:   "abc-def",
		CreatedAt:  time.Date(2026, 5, 21, 7, 41, 2, 0, time.UTC),
		Payload:    map[string]any{"subject": "alice"},
	}
	out := m.renderPayload(row)
	for _, want := range []string{
		"11111111", "license.issue", "operator", "License", "abc-def", "2026-05-21",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("renderPayload missing %q in output:\n%s", want, out)
		}
	}
}

// ── licenses detail panel renderers ──────────────────────────────────────────

// fixtureLicense returns a representative License row exercising every
// detail-panel renderer.
func fixtureLicense() *ent.License {
	return &ent.License{
		ID:             uuid.New(),
		LicenseUUID:    "01abcdef-1111-2222-3333-444444444444",
		Subject:        "alice@example.test",
		IssuerName:     "rshell-prod-2026Q2",
		Pem:            []byte("-----BEGIN LICENSE-----\npem\n-----END LICENSE-----\n"),
		BindingsMeta:   map[string]any{"machine": "fp-deadbeef"},
		IdentitySha256: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		BinarySha256:   "fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
		PayloadKind:    "sealed",
		Status:         "active",
	}
}

// TestRenderDetailBindings_ShowsKVAndPinning — Bindings tab content includes
// each binding entry and the truncated identity/binary fingerprints.
func TestRenderDetailBindings_ShowsKVAndPinning(t *testing.T) {
	m := licensesModel{}
	out := m.renderDetailBindings(fixtureLicense())
	for _, want := range []string{"machine", "fp-deadbeef", "identity", "binary", "sealed"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderDetailBindings missing %q in output:\n%s", want, out)
		}
	}
}

// TestRenderDetailBindings_NoBindingsRendersPlaceholder — empty BindingsMeta
// shows the "(no bindings)" hint instead of an empty section.
func TestRenderDetailBindings_NoBindingsRendersPlaceholder(t *testing.T) {
	m := licensesModel{}
	row := fixtureLicense()
	row.BindingsMeta = nil
	out := m.renderDetailBindings(row)
	if !strings.Contains(out, "no bindings") {
		t.Fatalf("expected '(no bindings)' placeholder; got:\n%s", out)
	}
}

// TestRenderDetailPEM_ContainsPEMBody — the PEM tab shows the full PEM and a
// copy hint.
func TestRenderDetailPEM_ContainsPEMBody(t *testing.T) {
	m := licensesModel{}
	out := m.renderDetailPEM(fixtureLicense())
	if !strings.Contains(out, "BEGIN LICENSE") {
		t.Fatalf("renderDetailPEM missing PEM body; got:\n%s", out)
	}
	if !strings.Contains(out, "[c]") {
		t.Fatalf("renderDetailPEM missing [c] copy hint")
	}
}

// TestRenderDetailPEM_EmptyPemShowsPlaceholder — when row.Pem is empty (DB
// integrity issue) a French placeholder replaces the body.
func TestRenderDetailPEM_EmptyPemShowsPlaceholder(t *testing.T) {
	m := licensesModel{}
	row := fixtureLicense()
	row.Pem = nil
	out := m.renderDetailPEM(row)
	if !strings.Contains(out, "PEM absent") {
		t.Fatalf("expected PEM absent placeholder; got:\n%s", out)
	}
}

// TestRenderDetailAudit_LoadingShowsHint — while detailAuditLoading is true
// the Audit tab shows a chargement hint, not an empty list.
func TestRenderDetailAudit_LoadingShowsHint(t *testing.T) {
	m := licensesModel{detailAuditLoading: true}
	out := m.renderDetailAudit(fixtureLicense())
	if !strings.Contains(out, "chargement") {
		t.Fatalf("loading state missing 'chargement' hint; got:\n%s", out)
	}
}

// TestRenderDetailAudit_EmptyShowsPlaceholder — done loading with zero rows
// shows the French "aucun évènement" placeholder.
func TestRenderDetailAudit_EmptyShowsPlaceholder(t *testing.T) {
	m := licensesModel{}
	out := m.renderDetailAudit(fixtureLicense())
	if !strings.Contains(out, "aucun") {
		t.Fatalf("empty-rows state missing 'aucun' placeholder; got:\n%s", out)
	}
}

// TestRenderDetailAudit_PopulatedShowsRows — with rows, each kind appears.
func TestRenderDetailAudit_PopulatedShowsRows(t *testing.T) {
	m := licensesModel{
		detailAuditRows: []*ent.AuditEvent{
			{Kind: "license.issue", Actor: "operator", CreatedAt: time.Now()},
			{Kind: "license.revoke", Actor: "operator", CreatedAt: time.Now()},
		},
	}
	out := m.renderDetailAudit(fixtureLicense())
	for _, want := range []string{"license.issue", "license.revoke", "operator"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderDetailAudit missing %q; got:\n%s", want, out)
		}
	}
}

// TestRenderDetailChain_RendersUUIDAndSubject — the lineage stub still surfaces
// the licence UUID prefix and subject.
func TestRenderDetailChain_RendersUUIDAndSubject(t *testing.T) {
	m := licensesModel{}
	out := m.renderDetailChain(fixtureLicense())
	if !strings.Contains(out, "01abcdef") {
		t.Errorf("renderDetailChain missing UUID prefix '01abcdef'")
	}
	if !strings.Contains(out, "alice@example.test") {
		t.Errorf("renderDetailChain missing subject")
	}
}

// TestRenderDetailChain_SupersededShowsWarning — when status is superseded a
// warning banner is appended.
func TestRenderDetailChain_SupersededShowsWarning(t *testing.T) {
	m := licensesModel{}
	row := fixtureLicense()
	row.Status = "superseded"
	out := m.renderDetailChain(row)
	if !strings.Contains(out, "SUPERSEDED") {
		t.Fatalf("superseded row missing warning banner; got:\n%s", out)
	}
}
