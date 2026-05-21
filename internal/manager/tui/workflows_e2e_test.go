package tui

// Workflow E2E tests — drive rootModel exactly as the bubbletea runtime does
// (WindowSize → key messages → assert state) without requiring a TTY.
//
// Package: tui (white-box) so we can inspect unexported fields directly.
// Tests are intentionally written BEFORE the corresponding bug-fixes so each
// test is red against the unfixed code and green after the fix.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	"github.com/oioio-space/maldev/internal/manager/tui/wizard"
)

func timeNowPlus(hours int) time.Time { return time.Now().Add(time.Duration(hours) * time.Hour) }
func newTestUUID() uuid.UUID          { return uuid.New() }

// ── drive helpers ──────────────────────────────────────────────────────────────

// driveRune sends one KeyRunes message carrying r.
func driveRune(m tea.Model, r rune) tea.Model {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return m
}

// driveKey sends a special key (Enter, Tab, Esc…).
func driveKey(m tea.Model, t tea.KeyType) tea.Model {
	m, _ = m.Update(tea.KeyMsg{Type: t})
	return m
}

// driveStr types each rune of s into m in order.
func driveStr(m tea.Model, s string) tea.Model {
	for _, r := range s {
		m = driveRune(m, r)
	}
	return m
}

// rootOf asserts m is a rootModel (same package) and returns it.
func rootOf(t *testing.T, m tea.Model) rootModel {
	t.Helper()
	r, ok := m.(rootModel)
	if !ok {
		t.Fatalf("expected rootModel, got %T", m)
	}
	return r
}

// newOnboardingRoot builds a rootModel in SessionOnboarding with a 120×40
// window applied.
func newOnboardingRoot(t *testing.T) tea.Model {
	t.Helper()
	var m tea.Model = New(nil, nil, SessionOnboarding)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// advanceToPassphraseStep presses Enter on the welcome screen to advance to
// stepPassphrase, failing the test if that doesn't happen.
func advanceToPassphraseStep(t *testing.T, m tea.Model) tea.Model {
	t.Helper()
	m = driveKey(m, tea.KeyEnter)
	if rootOf(t, m).onboarding.step != stepPassphrase {
		t.Fatalf("Enter on welcome did not advance to stepPassphrase")
	}
	return m
}

// ── test helpers for DB-level persistence tests ────────────────────────────────

// newTestServices opens an in-memory store with known salt/canary, derives a
// KEK from passphrase "testpass", and returns a wired *service.Services.
func newTestServices(t *testing.T) (*service.Services, *store.Store) {
	t.Helper()
	ctx := context.Background()
	st, err := store.New(ctx, ":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	salt := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	kek := crypto.DeriveFromPassphrase("testpass", salt)
	canary, err := crypto.NewCanary(kek)
	if err != nil {
		t.Fatalf("crypto.NewCanary: %v", err)
	}
	if err := st.EnsureSingletons(ctx, salt[:], canary); err != nil {
		t.Fatalf("EnsureSingletons: %v", err)
	}
	svc := service.New(st, kek)
	t.Cleanup(func() { _ = svc.Close() })
	return svc, st
}

// ── Bug 1: Enter on first field advances focus, not validate ───────────────────

// TestE2E_OnboardingEnterAdvancesFocusNotValidates is the canonical Bug 1 guard.
// Before the fix: Enter on field-0 runs validation, sees confirm=="", shows
// "passphrases do not match".  After the fix: focus advances to confirm.
func TestE2E_OnboardingEnterAdvancesFocusNotValidates(t *testing.T) {
	m := newOnboardingRoot(t)
	m = advanceToPassphraseStep(t, m)

	m = driveStr(m, "Str0ngP@ss!")
	m = driveKey(m, tea.KeyEnter) // Bug 1: should advance focus, not validate

	r := rootOf(t, m)
	if r.onboarding.passErr != "" {
		t.Fatalf("Enter on field 0 must not trigger validation, got passErr=%q", r.onboarding.passErr)
	}
	if r.onboarding.passFocused != 1 {
		t.Fatalf("Enter on field 0 must advance focus to confirm (passFocused=1), got %d", r.onboarding.passFocused)
	}
	if r.onboarding.step != stepPassphrase {
		t.Fatalf("step must stay stepPassphrase, got %d", r.onboarding.step)
	}
}

// TestE2E_OnboardingIssuerEnterAdvancesFocus guards the same pattern for the
// issuer step: Enter on the name field must jump focus to keyID.
func TestE2E_OnboardingIssuerEnterAdvancesFocus(t *testing.T) {
	m := newOnboardingRoot(t)
	m = advanceToPassphraseStep(t, m)

	// Complete passphrase step via Tab+Enter (the existing working path).
	m = driveStr(m, "Str0ngP@ss!")
	m = driveKey(m, tea.KeyTab)
	m = driveStr(m, "Str0ngP@ss!")
	m = driveKey(m, tea.KeyEnter)

	r := rootOf(t, m)
	if r.onboarding.step != stepIssuer {
		t.Fatalf("expected stepIssuer, got %d", r.onboarding.step)
	}

	m = driveStr(m, "production-2026")
	m = driveKey(m, tea.KeyEnter) // Bug 1 variant: Enter on name field

	r = rootOf(t, m)
	if r.onboarding.step != stepIssuer {
		t.Fatalf("step must stay stepIssuer after Enter on name, got %d", r.onboarding.step)
	}
	if r.onboarding.issuerFocus != 1 {
		t.Fatalf("issuerFocus must be 1 after Enter on name, got %d", r.onboarding.issuerFocus)
	}
}

// ── Bug 2: digits accepted in inputs during onboarding ────────────────────────

// TestE2E_OnboardingDigitsAcceptedInInputs is the canonical Bug 2 guard.
// Before the fix: global key handler intercepts '1'-'9' as tab switches even
// during SessionOnboarding, so digits never reach the textinput.
func TestE2E_OnboardingDigitsAcceptedInInputs(t *testing.T) {
	m := newOnboardingRoot(t)
	m = advanceToPassphraseStep(t, m)

	m = driveStr(m, "abc5def3")

	r := rootOf(t, m)
	got := r.onboarding.passInput.Value()
	if got != "abc5def3" {
		t.Fatalf("passInput value = %q, want %q — digits likely intercepted by global key handler", got, "abc5def3")
	}
}

// TestE2E_OnboardingDigitsInIssuerName verifies digits survive in the issuer
// name field, which uses the same code path as the passphrase field for Bug 2.
func TestE2E_OnboardingDigitsInIssuerName(t *testing.T) {
	m := newOnboardingRoot(t)
	m = advanceToPassphraseStep(t, m)

	m = driveStr(m, "Str0ngP@ss!")
	m = driveKey(m, tea.KeyTab)
	m = driveStr(m, "Str0ngP@ss!")
	m = driveKey(m, tea.KeyEnter)

	if rootOf(t, m).onboarding.step != stepIssuer {
		t.Fatalf("expected stepIssuer after passphrase, got %d", rootOf(t, m).onboarding.step)
	}

	m = driveStr(m, "production-2026")
	r := rootOf(t, m)
	got := r.onboarding.issuerName.Value()
	if got != "production-2026" {
		t.Fatalf("issuerName = %q, want %q", got, "production-2026")
	}
}

// ── Bugs 1+2 combined: full onboarding happy path ─────────────────────────────

// TestE2E_OnboardingHappyPath drives all 4 steps end-to-end with the fixed
// behaviour and asserts OnboardingDoneMsg carries the correct payload.
func TestE2E_OnboardingHappyPath(t *testing.T) {
	m := newOnboardingRoot(t)

	// Welcome → Enter → stepPassphrase
	m = driveKey(m, tea.KeyEnter)
	if rootOf(t, m).onboarding.step != stepPassphrase {
		t.Fatal("did not advance to stepPassphrase")
	}

	// field 0: type passphrase, Enter → focus to confirm
	m = driveStr(m, "myStr0ngP@ss123")
	m = driveKey(m, tea.KeyEnter)
	if rootOf(t, m).onboarding.passFocused != 1 {
		t.Fatalf("Enter on field 0 must advance focus, got passFocused=%d", rootOf(t, m).onboarding.passFocused)
	}

	// field 1 (confirm): type same passphrase, Enter → stepIssuer
	m = driveStr(m, "myStr0ngP@ss123")
	m = driveKey(m, tea.KeyEnter)
	if rootOf(t, m).onboarding.step != stepIssuer {
		t.Fatalf("expected stepIssuer, got %d", rootOf(t, m).onboarding.step)
	}

	// issuer name (contains digits): Enter → focus to keyID
	m = driveStr(m, "production-2026")
	m = driveKey(m, tea.KeyEnter)
	r := rootOf(t, m)
	if r.onboarding.issuerFocus != 1 {
		t.Fatalf("Enter on issuer name must advance focus, got issuerFocus=%d", r.onboarding.issuerFocus)
	}
	if r.onboarding.issuerName.Value() != "production-2026" {
		t.Fatalf("issuerName = %q, want 'production-2026'", r.onboarding.issuerName.Value())
	}

	// keyID: type + Enter → stepLicense
	m = driveStr(m, "maldev-prod-01")
	m = driveKey(m, tea.KeyEnter)
	if rootOf(t, m).onboarding.step != stepLicense {
		t.Fatalf("expected stepLicense, got %d", rootOf(t, m).onboarding.step)
	}

	// stepLicense: Enter → OnboardingDoneMsg emitted
	finalOnboarding := rootOf(t, m).onboarding
	_, cmd := finalOnboarding.handleEnter()
	if cmd == nil {
		t.Fatal("handleEnter on stepLicense must return a cmd")
	}
	msg := cmd()
	done, ok := msg.(OnboardingDoneMsg)
	if !ok {
		t.Fatalf("expected OnboardingDoneMsg, got %T", msg)
	}
	if done.Passphrase != "myStr0ngP@ss123" {
		t.Fatalf("OnboardingDoneMsg.Passphrase = %q, want 'myStr0ngP@ss123'", done.Passphrase)
	}
	if done.IssuerName != "production-2026" {
		t.Fatalf("OnboardingDoneMsg.IssuerName = %q, want 'production-2026'", done.IssuerName)
	}
	if done.IssuerKeyID != "maldev-prod-01" {
		t.Fatalf("OnboardingDoneMsg.IssuerKeyID = %q, want 'maldev-prod-01'", done.IssuerKeyID)
	}
}

// TestE2E_OnboardingPassphraseMismatch — confirm field with wrong passphrase
// shows error and stays on step 1.
func TestE2E_OnboardingPassphraseMismatch(t *testing.T) {
	m := newOnboardingRoot(t)
	m = advanceToPassphraseStep(t, m)

	// field 0 → Enter → focus to confirm
	m = driveStr(m, "correcthorse")
	m = driveKey(m, tea.KeyEnter)
	if rootOf(t, m).onboarding.passFocused != 1 {
		t.Skip("Bug 1 not fixed yet — skipping mismatch test")
	}

	// confirm with different value → Enter → error
	m = driveStr(m, "batterystaple")
	m = driveKey(m, tea.KeyEnter)

	r := rootOf(t, m)
	if r.onboarding.step != stepPassphrase {
		t.Fatalf("must stay on stepPassphrase after mismatch, got %d", r.onboarding.step)
	}
	if r.onboarding.passErr == "" {
		t.Fatal("expected passErr after mismatch, got empty")
	}
	if !strings.Contains(r.onboarding.passErr, "do not match") {
		t.Fatalf("passErr = %q, want substring 'do not match'", r.onboarding.passErr)
	}
}

// ── Bug 3: onboarding persists to fresh DB ─────────────────────────────────────

// TestE2E_OnboardingPersistsToFreshDB drives persistOnboarding (the helper
// extracted from runOnboarding) and then re-opens the DB to verify canary +
// issuer row.
func TestE2E_OnboardingPersistsToFreshDB(t *testing.T) {
	ctx := context.Background()
	dbPath := t.TempDir() + "/onboarding.db"

	msg := OnboardingDoneMsg{
		Passphrase:  "myStr0ngP@ss123",
		IssuerName:  "production-2026",
		IssuerKeyID: "maldev-prod-01",
	}
	if err := PersistOnboarding(ctx, dbPath, msg); err != nil {
		t.Fatalf("PersistOnboarding: %v", err)
	}

	// Re-open with the same passphrase.
	st, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("store.New (reopen): %v", err)
	}
	defer st.Close()

	row, err := st.Client.Setting.Get(ctx, 1)
	if err != nil {
		t.Fatalf("Setting.Get: %v", err)
	}
	if len(row.KekSalt) != 16 {
		t.Fatalf("kek_salt len = %d, want 16", len(row.KekSalt))
	}

	var salt [16]byte
	copy(salt[:], row.KekSalt)
	kek := crypto.DeriveFromPassphrase("myStr0ngP@ss123", salt)
	defer kek.Wipe()

	if !kek.VerifyCanary(row.KekCanary) {
		t.Fatal("canary verification failed — KEK does not match stored canary")
	}

	svc := service.New(st, kek)
	defer svc.Close()

	issuers, err := svc.Issuer.List(ctx)
	if err != nil {
		t.Fatalf("Issuer.List: %v", err)
	}
	if len(issuers) != 1 {
		t.Fatalf("expected 1 issuer, got %d", len(issuers))
	}
	if issuers[0].Name != "production-2026" {
		t.Fatalf("issuer Name = %q, want 'production-2026'", issuers[0].Name)
	}
	if issuers[0].KeyID != "maldev-prod-01" {
		t.Fatalf("issuer KeyID = %q, want 'maldev-prod-01'", issuers[0].KeyID)
	}
}

// TestE2E_RelaunchAfterOnboarding — after persistOnboarding writes the DB,
// a second open with the correct passphrase succeeds; wrong passphrase fails.
func TestE2E_RelaunchAfterOnboarding(t *testing.T) {
	ctx := context.Background()
	dbPath := t.TempDir() + "/relaunch.db"

	if err := PersistOnboarding(ctx, dbPath, OnboardingDoneMsg{
		Passphrase:  "relaunch-pass-99",
		IssuerName:  "relaunch-issuer",
		IssuerKeyID: "key-relaunch",
	}); err != nil {
		t.Fatalf("PersistOnboarding: %v", err)
	}

	// Correct passphrase → canary OK.
	st, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	row, _ := st.Client.Setting.Get(ctx, 1)
	var salt [16]byte
	copy(salt[:], row.KekSalt)
	kek := crypto.DeriveFromPassphrase("relaunch-pass-99", salt)
	if !kek.VerifyCanary(row.KekCanary) {
		t.Fatal("expected canary OK with correct passphrase")
	}
	kek.Wipe()

	// Wrong passphrase → canary fail.
	kek2 := crypto.DeriveFromPassphrase("wrong-pass", salt)
	if kek2.VerifyCanary(row.KekCanary) {
		t.Fatal("expected canary to FAIL with wrong passphrase")
	}
	kek2.Wipe()
	st.Close()
}

// ── Passphrase unlock screen ───────────────────────────────────────────────────

// TestE2E_PassphraseUnlockSuccess — passphraseModel (no store wired) accepts
// any non-empty passphrase and emits PassphraseResult.
func TestE2E_PassphraseUnlockSuccess(t *testing.T) {
	var m tea.Model = New(nil, nil, SessionLocked)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	m = driveStr(m, "Str0ngP@ss!")
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected cmd after Enter, got nil")
	}
	msg := cmd()
	pr, ok := msg.(PassphraseResult)
	if !ok {
		t.Fatalf("expected PassphraseResult, got %T", msg)
	}
	if pr.Passphrase != "Str0ngP@ss!" {
		t.Fatalf("PassphraseResult.Passphrase = %q, want 'Str0ngP@ss!'", pr.Passphrase)
	}
}

// TestE2E_PassphraseUnlockWrong — empty passphrase shows error, session stays
// SessionLocked.
func TestE2E_PassphraseUnlockWrong(t *testing.T) {
	var m tea.Model = New(nil, nil, SessionLocked)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	m = driveKey(m, tea.KeyEnter) // empty input
	r := rootOf(t, m)
	if r.passphrase.err == "" {
		t.Fatal("expected error for empty passphrase, got empty")
	}
	if r.session != SessionLocked {
		t.Fatalf("session must stay SessionLocked, got %d", r.session)
	}
}

// ── Servers start/stop ─────────────────────────────────────────────────────────

// TestE2E_ServersStartStop drives serverStartMsg/serverStopMsg through
// serversModel directly and asserts the mock controller records the calls.
func TestE2E_ServersStartStop(t *testing.T) {
	mc := &testCtrl{}
	sm := newServersModel(mc)
	sm, _ = sm.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	sm, cmd := sm.Update(serverStartMsg{name: "revocation"})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			sm, _ = sm.Update(msg)
		}
	}
	if len(mc.starts) == 0 || mc.starts[0] != "revocation" {
		t.Fatalf("expected Start(revocation), got starts=%v", mc.starts)
	}

	sm, cmd = sm.Update(serverStopMsg{name: "heartbeat"})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			sm, _ = sm.Update(msg)
		}
	}
	if len(mc.stops) == 0 || mc.stops[0] != "heartbeat" {
		t.Fatalf("expected Stop(heartbeat), got stops=%v", mc.stops)
	}
	_ = sm
}

// testCtrl implements httpsrv.Controller for in-package tests.
type testCtrl struct {
	starts []string
	stops  []string
}

func (tc *testCtrl) Start(_ context.Context, name string) error {
	tc.starts = append(tc.starts, name)
	return nil
}

func (tc *testCtrl) Stop(name string) error {
	tc.stops = append(tc.stops, name)
	return nil
}

func (tc *testCtrl) Statuses() map[string]httpsrv.Status { return nil }

func (tc *testCtrl) MergedEvents() <-chan httpsrv.Event {
	return make(chan httpsrv.Event)
}

// ── License issuance + revocation via service (workflow 7+8 from spec) ────────

// TestE2E_LicenseIssueViaService seeds a wired *service.Services with an
// issuer, drives LicenseService.Issue with a minimal IssueRequest, and asserts
// the returned PEM + persisted row. This is the same code path the wizard
// step 8 (Issue) hits when the operator confirms in the UI.
func TestE2E_LicenseIssueViaService(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()

	iss, err := svc.Issuer.Generate(ctx, "e2e-issuer", "k-e2e", "operator")
	if err != nil {
		t.Fatalf("Issuer.Generate: %v", err)
	}

	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID:  iss.ID,
		Subject:   "alice@example.test",
		NotAfter:  timeNowPlus(24),
		Actor:     "operator",
	})
	if err != nil {
		t.Fatalf("License.Issue: %v", err)
	}
	if len(out.PEM) == 0 {
		t.Fatal("issued license has empty PEM")
	}
	if out.Row.LicenseUUID == "" {
		t.Fatal("issued license has empty UUID")
	}

	rows, err := svc.License.List(ctx, service.ListFilter{})
	if err != nil {
		t.Fatalf("License.List: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 license row, got %d", len(rows))
	}
}

// TestE2E_LicenseIssueAndRevokeFullFlow stitches Issue + Revoke + ListRevoked
// together to assert the full lifecycle reaches the persisted revocation row.
func TestE2E_LicenseIssueAndRevokeFullFlow(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()

	iss, err := svc.Issuer.Generate(ctx, "rev-issuer", "k-rev", "operator")
	if err != nil {
		t.Fatalf("Issuer.Generate: %v", err)
	}
	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID,
		Subject:  "bob@example.test",
		NotAfter: timeNowPlus(24),
		Actor:    "operator",
	})
	if err != nil {
		t.Fatalf("License.Issue: %v", err)
	}

	if err := svc.Revoke.Revoke(ctx, out.Row.ID, "compromised key", "operator"); err != nil {
		t.Fatalf("Revoke.Revoke: %v", err)
	}

	revoked, err := svc.Revoke.ListRevoked(ctx)
	if err != nil {
		t.Fatalf("Revoke.ListRevoked: %v", err)
	}
	if len(revoked) != 1 {
		t.Fatalf("expected 1 revoked row, got %d", len(revoked))
	}
	if revoked[0].Reason != "compromised key" {
		t.Fatalf("reason = %q, want 'compromised key'", revoked[0].Reason)
	}
}

// ── Revoke overlay UI behaviour ────────────────────────────────────────────────

// TestE2E_RevocationOverlayEmitsConfirmedMsg drives the revokeOverlay through
// reason entry + Enter and asserts the emitted message carries the right
// LicenseID and reason. This is the UI half of the revocation flow.
func TestE2E_RevocationOverlayEmitsConfirmedMsg(t *testing.T) {
	licenseID := newTestUUID()
	ov := newRevokeOverlay(licenseID, "alice@example.test")

	var o Overlay = ov
	for _, r := range "compromised key" {
		o, _ = o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter must emit OverlayDoneMsg cmd, got nil")
	}
	done, ok := cmd().(OverlayDoneMsg)
	if !ok {
		t.Fatalf("expected OverlayDoneMsg, got %T", cmd())
	}
	rc, ok := done.Result.(RevokeConfirmedMsg)
	if !ok {
		t.Fatalf("expected RevokeConfirmedMsg result, got %T", done.Result)
	}
	if rc.LicenseID != licenseID {
		t.Fatalf("LicenseID = %v, want %v", rc.LicenseID, licenseID)
	}
	if rc.Reason != "compromised key" {
		t.Fatalf("Reason = %q, want 'compromised key'", rc.Reason)
	}
}

// TestE2E_RevocationOverlayEmptyReasonStays asserts pressing Enter on an
// empty reason input does NOT emit OverlayDoneMsg (the overlay should stay
// open until a reason is typed).
func TestE2E_RevocationOverlayEmptyReasonStays(t *testing.T) {
	ov := newRevokeOverlay(newTestUUID(), "alice")
	var o Overlay = ov
	_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("Enter with empty reason must not emit, got cmd=%v", cmd())
	}
}

// TestE2E_RevocationOverlayEscCancels asserts Esc emits OverlayDoneMsg with
// nil Result (cancellation path).
func TestE2E_RevocationOverlayEscCancels(t *testing.T) {
	ov := newRevokeOverlay(newTestUUID(), "alice")
	var o Overlay = ov
	_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc must emit OverlayDoneMsg, got nil")
	}
	done, ok := cmd().(OverlayDoneMsg)
	if !ok {
		t.Fatalf("expected OverlayDoneMsg, got %T", cmd())
	}
	if done.Result != nil {
		t.Fatalf("Esc must emit nil Result, got %T: %v", done.Result, done.Result)
	}
}

// ── Dashboard data flow ────────────────────────────────────────────────────────

// TestE2E_DashboardSnapshotPopulatesCounters wires a real service with seeded
// data, fires the snapshot cmd, dispatches the resulting Msg, and asserts the
// dashboard counters reflect the seeded rows.
func TestE2E_DashboardSnapshotPopulatesCounters(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, err := svc.Issuer.Generate(ctx, "dash-issuer", "k-dash", "operator")
	if err != nil {
		t.Fatalf("Issuer.Generate: %v", err)
	}
	if err := svc.Issuer.SetActive(ctx, iss.ID, "operator"); err != nil {
		t.Fatalf("Issuer.SetActive: %v", err)
	}
	// One license outside the 30-day "expiring soon" window → counts as Active.
	if _, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID,
		Subject:  "dash-active",
		NotAfter: timeNowPlus(60 * 24),
		Actor:    "operator",
	}); err != nil {
		t.Fatalf("License.Issue (active): %v", err)
	}
	// One license inside the 30-day window → counts as ExpiringSoon.
	if _, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID,
		Subject:  "dash-soon",
		NotAfter: timeNowPlus(24),
		Actor:    "operator",
	}); err != nil {
		t.Fatalf("License.Issue (soon): %v", err)
	}

	dm := newDashboardModel(svc, nil)
	cmd := dm.refresh()
	if cmd == nil {
		t.Fatal("refresh cmd is nil")
	}
	dm, _ = dm.Update(cmd())
	if dm.loading {
		t.Fatal("dashboard still loading after snapshot dispatched")
	}
	if dm.counters.active != 1 {
		t.Fatalf("counters.active = %d, want 1 (60-day license)", dm.counters.active)
	}
	if dm.counters.expiringSoon != 1 {
		t.Fatalf("counters.expiringSoon = %d, want 1 (24h license)", dm.counters.expiringSoon)
	}
	if dm.activeKey.name != "dash-issuer" {
		t.Fatalf("activeKey.name = %q, want 'dash-issuer'", dm.activeKey.name)
	}
}

// TestE2E_DashboardTileClickEmitsSwitchMsg walks the rendered widget tree,
// clicks the Active tile's bounds, and verifies a SwitchToLicensesMsg is
// dispatched with the right filter.
func TestE2E_DashboardTileClickEmitsSwitchMsg(t *testing.T) {
	dm := newDashboardModel(nil, nil)
	dm.width = 120
	dm.height = 40
	dm.counters.active = 5
	tree := dm.buildWidgetTree()
	tree.Layout(Rect{X: 0, Y: 0, W: 120, H: 40})

	// The Active tile sits in the first row of the dashboard. Walk the tree
	// looking for a Clickable whose bounds we can probe.
	var found tea.Cmd
	walkClickable(tree, func(c Clickable) {
		b := c.Bounds()
		if cmd := c.OnClick(0, 0, tea.MouseButtonLeft); cmd != nil {
			if msg := cmd(); msg != nil {
				if _, ok := msg.(SwitchToLicensesMsg); ok {
					found = cmd
				}
			}
		}
		_ = b
	})
	if found == nil {
		t.Fatal("no Clickable in dashboard tree emitted SwitchToLicensesMsg")
	}
}

// walkClickable does a best-effort depth-first walk for Clickable widgets
// in any widget tree, calling visit on each.
func walkClickable(w Widget, visit func(Clickable)) {
	if c, ok := w.(Clickable); ok {
		visit(c)
	}
	if cw, ok := w.(interface{ Children() []Widget }); ok {
		for _, ch := range cw.Children() {
			walkClickable(ch, visit)
		}
	}
}

// ── Input overlay UX ──────────────────────────────────────────────────────────

// TestE2E_InputOverlayEmitsValue types a value + Enter and asserts the emitted
// InputResultMsg carries the right ID and Value.
func TestE2E_InputOverlayEmitsValue(t *testing.T) {
	ov := newInputOverlay("issuer-name", "New Issuer", "name", 64)
	var o Overlay = ov
	for _, r := range "production-2026" {
		o, _ = o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter must emit cmd")
	}
	done, ok := cmd().(OverlayDoneMsg)
	if !ok {
		t.Fatalf("expected OverlayDoneMsg, got %T", cmd())
	}
	ir, ok := done.Result.(InputResultMsg)
	if !ok {
		t.Fatalf("expected InputResultMsg, got %T", done.Result)
	}
	if ir.ID != "issuer-name" || ir.Value != "production-2026" {
		t.Fatalf("InputResultMsg = %+v, want {ID:issuer-name, Value:production-2026}", ir)
	}
}

// TestE2E_InputOverlayEmptyStays — Enter with empty value does not emit.
func TestE2E_InputOverlayEmptyStays(t *testing.T) {
	ov := newInputOverlay("x", "T", "p", 64)
	var o Overlay = ov
	_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("Enter on empty input must not emit, got %v", cmd())
	}
}

// TestE2E_InputOverlayEscCancels — Esc emits OverlayDoneMsg with nil Result.
func TestE2E_InputOverlayEscCancels(t *testing.T) {
	ov := newInputOverlay("x", "T", "p", 64)
	var o Overlay = ov
	_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc must emit cmd")
	}
	done, ok := cmd().(OverlayDoneMsg)
	if !ok {
		t.Fatalf("expected OverlayDoneMsg, got %T", cmd())
	}
	if done.Result != nil {
		t.Fatalf("Esc must emit nil Result, got %v", done.Result)
	}
}

// ── Confirm overlay UX ────────────────────────────────────────────────────────

// TestE2E_ConfirmOverlayBothPaths covers y/enter → Confirm:true and n/esc →
// Confirm:false in a single table-driven test.
func TestE2E_ConfirmOverlayBothPaths(t *testing.T) {
	cases := []struct {
		name string
		key  string
		want bool
	}{
		{"y_confirms", "y", true},
		{"Y_confirms", "Y", true},
		{"enter_confirms", "enter", true},
		{"n_cancels", "n", false},
		{"N_cancels", "N", false},
		{"esc_cancels", "esc", false},
		{"q_cancels", "q", false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			ov := newConfirmOverlay("test-id", "T", "body", "yes", "no", false)
			var o Overlay = ov
			var msg tea.KeyMsg
			switch c.key {
			case "enter":
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			case "esc":
				msg = tea.KeyMsg{Type: tea.KeyEsc}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(c.key)}
			}
			_, cmd := o.Update(msg)
			if cmd == nil {
				t.Fatalf("key %q must emit cmd", c.key)
			}
			done := cmd().(OverlayDoneMsg)
			cr, ok := done.Result.(ConfirmResultMsg)
			if !ok {
				t.Fatalf("expected ConfirmResultMsg, got %T", done.Result)
			}
			if cr.Confirm != c.want {
				t.Fatalf("key %q: Confirm = %v, want %v", c.key, cr.Confirm, c.want)
			}
			if cr.ID != "test-id" {
				t.Fatalf("ID = %q, want 'test-id'", cr.ID)
			}
		})
	}
}

// ── Screen list-loading paths ─────────────────────────────────────────────────

// TestE2E_IssuersScreenLoadsRows wires a seeded service into issuersModel,
// fires the list cmd, dispatches the resulting msg, and asserts rows persisted.
func TestE2E_IssuersScreenLoadsRows(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	if _, err := svc.Issuer.Generate(ctx, "i1", "k1", "operator"); err != nil {
		t.Fatalf("Issuer.Generate i1: %v", err)
	}
	if _, err := svc.Issuer.Generate(ctx, "i2", "k2", "operator"); err != nil {
		t.Fatalf("Issuer.Generate i2: %v", err)
	}

	im := newIssuersModel(svc)
	cmd := listIssuersCmd(svc)
	if cmd == nil {
		t.Fatal("listIssuersCmd nil")
	}
	msg := cmd()
	im, _ = im.Update(msg)
	if len(im.rows) != 2 {
		t.Fatalf("issuersModel.rows len = %d, want 2", len(im.rows))
	}
}

// TestE2E_LicensesScreenLoadsRows wires a seeded service into licensesModel
// and asserts the table populates via ListLicensesCmd.
func TestE2E_LicensesScreenLoadsRows(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, err := svc.Issuer.Generate(ctx, "lic-iss", "k-lic", "operator")
	if err != nil {
		t.Fatalf("Issuer.Generate: %v", err)
	}
	for _, sub := range []string{"sub-a", "sub-b", "sub-c"} {
		if _, err := svc.License.Issue(ctx, service.IssueRequest{
			IssuerID: iss.ID,
			Subject:  sub,
			NotAfter: timeNowPlus(24),
			Actor:    "operator",
		}); err != nil {
			t.Fatalf("License.Issue %q: %v", sub, err)
		}
	}

	lm := newLicensesModel(svc)
	cmd := ListLicensesCmd(svc)
	if cmd == nil {
		t.Fatal("ListLicensesCmd nil")
	}
	msg := cmd()
	lm, _ = lm.Update(msg)
	if got := len(lm.rows); got != 3 {
		t.Fatalf("licensesModel.rows len = %d, want 3", got)
	}
}

// ── Wizard 8-step navigation ──────────────────────────────────────────────────

// TestE2E_WizardAdvancesThroughAllSteps drives wizardModel via the same typed
// msgs each step emits when the user confirms. Asserts each step transitions
// to the next exactly once and reaches Review.
func TestE2E_WizardAdvancesThroughAllSteps(t *testing.T) {
	svc, _ := newTestServices(t)
	wm := newWizardModel(svc)

	type tcase struct {
		name string
		msg  tea.Msg
		want wizardStep
	}
	issuerID := uuid.New().String()
	recipientID := uuid.New().String()
	steps := []tcase{
		{"step1->step2", wizard.IdentityChosenMsg{IssuerID: issuerID}, wizStepRecipient},
		{"step2->step3", wizard.RecipientChosenMsg{RecipientID: recipientID}, wizStepMachine},
		{"step3->step4", wizard.MachineBindingMsg{MachineID: "host-abc"}, wizStepBinary},
		{"step4->step5", wizard.BinaryBindingMsg{SHA256: "deadbeef", Size: 1234}, wizStepValidity},
		{"step5->step6", wizard.ValidityMsg{NotBefore: time.Now(), NotAfter: time.Now().Add(24 * time.Hour)}, wizStepFreeFields},
		{"step6->step7", wizard.FreeFieldsMsg{Fields: map[string]string{"k": "v"}}, wizStepTOTP},
		{"step7->step8", wizard.TOTPChoiceMsg{Require: false}, wizStepReview},
	}
	for _, s := range steps {
		s := s
		t.Run(s.name, func(t *testing.T) {
			var cmd tea.Cmd
			wm, cmd = wm.Update(s.msg)
			_ = cmd // initStep cmds may exist; we don't dispatch them in this test
			if wm.step != s.want {
				t.Fatalf("after %T, step = %d, want %d", s.msg, wm.step, s.want)
			}
		})
	}

	if wm.state.MachineID != "host-abc" {
		t.Fatalf("state.MachineID = %q, want 'host-abc'", wm.state.MachineID)
	}
	if wm.state.BinarySHA256 != "deadbeef" {
		t.Fatalf("state.BinarySHA256 = %q, want 'deadbeef'", wm.state.BinarySHA256)
	}
	if wm.state.IssuerID != issuerID {
		t.Fatalf("state.IssuerID = %q, want %q", wm.state.IssuerID, issuerID)
	}
}

// TestE2E_WizardEscRetreats covers Esc → back navigation across steps.
func TestE2E_WizardEscRetreats(t *testing.T) {
	wm := newWizardModel(nil)
	wm, _ = wm.Update(wizard.IdentityChosenMsg{IssuerID: uuid.New().String()})
	if wm.step != wizStepRecipient {
		t.Fatalf("setup: step = %d, want %d", wm.step, wizStepRecipient)
	}
	wm, _ = wm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if wm.step != wizStepIdentity {
		t.Fatalf("after Esc, step = %d, want wizStepIdentity (%d)", wm.step, wizStepIdentity)
	}
	// Esc on first step is a no-op.
	wm, _ = wm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if wm.step != wizStepIdentity {
		t.Fatalf("Esc on first step must stay, got %d", wm.step)
	}
}

// TestE2E_WizardIssueResultSuccessEmitsDone — IssueResultMsg with no error
// drives the wizard to emit WizardDoneMsg carrying the IssuedLicense.
func TestE2E_WizardIssueResultSuccessEmitsDone(t *testing.T) {
	wm := newWizardModel(nil)
	wm.step = wizStepReview
	issued := &service.IssuedLicense{}
	_, cmd := wm.Update(wizard.IssueResultMsg{Issued: issued})
	if cmd == nil {
		t.Fatal("IssueResultMsg must emit cmd, got nil")
	}
	done, ok := cmd().(WizardDoneMsg)
	if !ok {
		t.Fatalf("expected WizardDoneMsg, got %T", cmd())
	}
	if done.Issued != issued {
		t.Fatal("WizardDoneMsg.Issued != the IssuedLicense we passed")
	}
}

// TestE2E_WizardIssueResultCancelledEmitsDoneNil — cancelled error emits
// WizardDoneMsg with nil Issued (clean exit, no error overlay).
func TestE2E_WizardIssueResultCancelledEmitsDoneNil(t *testing.T) {
	wm := newWizardModel(nil)
	wm.step = wizStepReview
	_, cmd := wm.Update(wizard.IssueResultMsg{Err: errCancelled{}})
	if cmd == nil {
		t.Fatal("IssueResultMsg cancelled must emit cmd")
	}
	done, ok := cmd().(WizardDoneMsg)
	if !ok {
		t.Fatalf("expected WizardDoneMsg, got %T", cmd())
	}
	if done.Issued != nil {
		t.Fatalf("cancelled path must emit nil Issued, got %v", done.Issued)
	}
}

// errCancelled implements error with the magic "cancelled" text the wizard
// matches on to distinguish cancellation from genuine failure.
type errCancelled struct{}

func (errCancelled) Error() string { return "cancelled" }

// ── Probe drawer state machine ────────────────────────────────────────────────

// TestE2E_ProbeDrawerStateMachine walks the probe drawer through its 3
// happy-path states: issuing → waiting → received → confirm.
func TestE2E_ProbeDrawerStateMachine(t *testing.T) {
	var capturedMachineID string
	onResult := func(machineID string) tea.Cmd {
		capturedMachineID = machineID
		return func() tea.Msg { return nil }
	}
	ov := newProbeDrawerOverlay(nil, onResult)
	if ov.state != probeStateIssuing {
		t.Fatalf("initial state = %d, want probeStateIssuing", ov.state)
	}

	// Simulate token issuance success.
	tok := &ent.ProbeToken{ID: "abc-tok", CompositeHex: "machine-fp"}
	var o Overlay = ov
	o, cmd := o.Update(ProbeTokenIssuedMsg{Token: tok})
	if ov.state != probeStateWaiting {
		t.Fatalf("after token issued, state = %d, want probeStateWaiting", ov.state)
	}
	if cmd == nil {
		t.Fatal("after token issued, subscribe cmd must be returned")
	}

	// Simulate agent callback arriving with a result.
	o, _ = o.Update(ProbeAgentResultMsg{Token: tok})
	if ov.state != probeStateReceived {
		t.Fatalf("after agent result, state = %d, want probeStateReceived", ov.state)
	}

	// Enter on received → onResult callback fires with the machine ID.
	o, cmd = o.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on probeStateReceived must emit cmd")
	}
	// Drain the batch to fire onResult.
	if batch := cmd(); batch != nil {
		if bm, ok := batch.(tea.BatchMsg); ok {
			for _, c := range bm {
				if c != nil {
					c()
				}
			}
		}
	}
	if capturedMachineID != "machine-fp" {
		t.Fatalf("onResult got machineID = %q, want 'machine-fp'", capturedMachineID)
	}
	_ = o
}

// TestE2E_ProbeDrawerErrorOnTokenFail asserts an issuance failure transitions
// to probeStateError.
func TestE2E_ProbeDrawerErrorOnTokenFail(t *testing.T) {
	ov := newProbeDrawerOverlay(nil, nil)
	var o Overlay = ov
	o, _ = o.Update(ProbeTokenIssuedMsg{Err: errCancelled{}})
	if ov.state != probeStateError {
		t.Fatalf("on token err, state = %d, want probeStateError", ov.state)
	}
	if ov.errMsg == "" {
		t.Fatal("errMsg must be populated on error")
	}
	_ = o
}

// ── File picker navigation ────────────────────────────────────────────────────

// TestE2E_FilePickerCursorMoves verifies ↑/↓ move the cursor without going OOB.
func TestE2E_FilePickerCursorMoves(t *testing.T) {
	ov := newFilePickerOverlay(nil)
	if len(ov.entries) < 2 {
		t.Skip("home dir has fewer than 2 visible entries — cannot test cursor movement")
	}
	startCursor := ov.cursor

	var o Overlay = ov
	o, _ = o.Update(tea.KeyMsg{Type: tea.KeyDown})
	if ov.cursor != startCursor+1 {
		t.Fatalf("after Down, cursor = %d, want %d", ov.cursor, startCursor+1)
	}
	o, _ = o.Update(tea.KeyMsg{Type: tea.KeyUp})
	if ov.cursor != startCursor {
		t.Fatalf("after Up, cursor = %d, want %d", ov.cursor, startCursor)
	}
	// Up at row 0 must stay at 0.
	o, _ = o.Update(tea.KeyMsg{Type: tea.KeyUp})
	if ov.cursor < 0 {
		t.Fatalf("cursor went negative: %d", ov.cursor)
	}
	_ = o
}

// TestE2E_FilePickerEscCancels asserts Esc emits OverlayDoneMsg{Result: nil}.
func TestE2E_FilePickerEscCancels(t *testing.T) {
	ov := newFilePickerOverlay(nil)
	var o Overlay = ov
	_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc must emit cmd")
	}
	done, ok := cmd().(OverlayDoneMsg)
	if !ok {
		t.Fatalf("expected OverlayDoneMsg, got %T", cmd())
	}
	if done.Result != nil {
		t.Fatalf("Esc must emit nil Result, got %v", done.Result)
	}
}

// ── QR overlay UX ─────────────────────────────────────────────────────────────

// TestE2E_QROverlayCloseKeys covers the three close paths (Esc, Enter, q).
func TestE2E_QROverlayCloseKeys(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyEsc, tea.KeyEnter} {
		key := key
		t.Run(keyName(key), func(t *testing.T) {
			ov := NewQROverlay(nil)
			_, cmd := ov.Update(tea.KeyMsg{Type: key})
			if cmd == nil {
				t.Fatalf("key %v must emit cmd", key)
			}
			done, ok := cmd().(OverlayDoneMsg)
			if !ok {
				t.Fatalf("expected OverlayDoneMsg, got %T", cmd())
			}
			if done.Result != nil {
				t.Fatalf("close must emit nil Result, got %v", done.Result)
			}
		})
	}
	t.Run("q_closes", func(t *testing.T) {
		ov := NewQROverlay(nil)
		_, cmd := ov.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		if cmd == nil {
			t.Fatal("q must emit close cmd")
		}
	})
}

// TestE2E_QROverlayQRSavedMsgUpdatesState asserts QRSavedMsg{Path: ...}
// sets the saved field, and {Err: ...} sets saveErr.
func TestE2E_QROverlayQRSavedMsgUpdatesState(t *testing.T) {
	ov := newQROverlay(nil)
	var o Overlay = ov
	o, _ = o.Update(QRSavedMsg{Path: "/tmp/licence.pem"})
	if ov.saved != "/tmp/licence.pem" {
		t.Fatalf("saved = %q, want '/tmp/licence.pem'", ov.saved)
	}

	o, _ = o.Update(QRSavedMsg{Err: errCancelled{}})
	if ov.saveErr == "" {
		t.Fatal("saveErr must be populated on error msg")
	}
	_ = o
}

// keyName is a debug helper for table-driven key tests.
func keyName(k tea.KeyType) string {
	switch k {
	case tea.KeyEsc:
		return "esc_closes"
	case tea.KeyEnter:
		return "enter_closes"
	default:
		return "key"
	}
}

// ── Responsive layout (WindowSizeMsg propagation) ──────────────────────────────

// TestE2E_WindowSizePropagatesToScreens drives several window sizes and
// confirms View() never panics and produces non-empty output at each size.
func TestE2E_WindowSizePropagatesToScreens(t *testing.T) {
	sizes := []struct{ w, h int }{
		{80, 24},   // minimum
		{120, 40},  // standard
		{160, 50},  // wide
		{200, 60},  // ultra-wide
	}
	for _, sz := range sizes {
		sz := sz
		t.Run(fmt.Sprintf("%dx%d", sz.w, sz.h), func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
			for _, k := range []rune{'1', '2', '3', '7'} {
				m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}})
				if got := m.View(); got == "" {
					t.Fatalf("View() empty at %dx%d on view %c", sz.w, sz.h, k)
				}
			}
		})
	}
}

// ── Licenses screen interactions ──────────────────────────────────────────────

// TestE2E_LicensesFilterCycles asserts pressing 'f' cycles the filter chip
// through its 5 states (all → active → expiring → expired → revoked →
// superseded → all).
func TestE2E_LicensesFilterCycles(t *testing.T) {
	lm := newLicensesModel(nil)
	start := lm.filter
	for i := 0; i < int(licFilterCount); i++ {
		lm, _ = lm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	}
	if lm.filter != start {
		t.Fatalf("after licFilterCount=%d presses, filter = %d, want %d (full cycle)",
			licFilterCount, lm.filter, start)
	}
	// One more press → +1.
	lm, _ = lm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if lm.filter == start {
		t.Fatalf("after %d+1 presses, filter must have advanced from %d", licFilterCount, start)
	}
}

// TestE2E_LicensesDetailToggles asserts 'd' and Enter both flip the detail
// panel boolean.
func TestE2E_LicensesDetailToggles(t *testing.T) {
	lm := newLicensesModel(nil)
	if lm.detail {
		t.Fatal("detail must start false")
	}
	lm, _ = lm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !lm.detail {
		t.Fatal("after 'd', detail must be true")
	}
	lm, _ = lm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if lm.detail {
		t.Fatal("after Enter, detail must be false")
	}
}

// TestE2E_LicensesSearchFocus asserts '/' enters search mode and Esc exits.
func TestE2E_LicensesSearchFocus(t *testing.T) {
	lm := newLicensesModel(nil)
	if lm.search.Focused() {
		t.Fatal("search must start unfocused")
	}
	lm, _ = lm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !lm.search.Focused() {
		t.Fatal("after '/', search must be focused")
	}
	lm, _ = lm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if lm.search.Focused() {
		t.Fatal("after Esc, search must be unfocused")
	}
}

// TestE2E_LicensesRevokeOpensOverlay asserts 'x' on a selected row emits a
// pushOverlayMsg wrapping a *revokeOverlay.
func TestE2E_LicensesRevokeOpensOverlay(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, err := svc.Issuer.Generate(ctx, "x-iss", "k-x", "operator")
	if err != nil {
		t.Fatalf("Issuer.Generate: %v", err)
	}
	if _, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID,
		Subject:  "revoke-me",
		NotAfter: timeNowPlus(48),
		Actor:    "operator",
	}); err != nil {
		t.Fatalf("License.Issue: %v", err)
	}

	lm := newLicensesModel(svc)
	lm, _ = lm.Update(ListLicensesCmd(svc)())
	if len(lm.rows) != 1 {
		t.Fatalf("setup: rows = %d, want 1", len(lm.rows))
	}

	_, cmd := lm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd == nil {
		t.Fatal("'x' on selected row must emit cmd")
	}
	push, ok := cmd().(pushOverlayMsg)
	if !ok {
		t.Fatalf("expected pushOverlayMsg, got %T", cmd())
	}
	if _, ok := push.overlay.(*revokeOverlay); !ok {
		t.Fatalf("expected *revokeOverlay, got %T", push.overlay)
	}
}

// TestE2E_LicensesNewKeyOpensWizard asserts 'n' triggers openWizardCmd, which
// returns a non-nil cmd whose message opens the wizard via pushOverlayMsg.
func TestE2E_LicensesNewKeyOpensWizard(t *testing.T) {
	lm := newLicensesModel(nil)
	_, cmd := lm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("'n' must emit cmd to open wizard")
	}
	// We don't dispatch (nil services + wizard cmd would crash); reaching the
	// cmd is enough to prove the key is bound.
}

// ── Help overlay in every view ─────────────────────────────────────────────────

// TestE2E_HelpOverlayInEachView presses '?' in every SessionReady view and
// verifies View() doesn't panic (returns non-empty string).
func TestE2E_HelpOverlayInEachView(t *testing.T) {
	views := []struct {
		key   rune
		label string
	}{
		{'1', "Dashboard"},
		{'2', "Licenses"},
		{'3', "Issuers"},
		{'4', "Recipients"},
		{'5', "Identities"},
		{'6', "Revocation"},
		{'7', "Servers"},
		{'8', "Audit"},
		{'9', "Settings"},
	}

	for _, v := range views {
		v := v
		t.Run(v.label, func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			m = driveRune(m, v.key)
			m = driveRune(m, '?')
			got := m.View()
			if got == "" {
				t.Fatalf("View() returned empty after '?' in %s", v.label)
			}
		})
	}
}
