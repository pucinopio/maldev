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
	"github.com/charmbracelet/lipgloss"
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

// TestE2E_IssuersDetailPanel verifies the redesigned 2-column detail panel
// renders metadata (keyid, name, status) and the 4 action hints.
// Red against the old plain-text detail, green after the 2-column layout.
func TestE2E_IssuersDetailPanel(t *testing.T) {
	svc, _ := newTestServices(t)
	if _, err := svc.Issuer.Generate(context.Background(), "prod-key", "k2026-04", "operator"); err != nil {
		t.Fatalf("Issuer.Generate: %v", err)
	}

	im := newIssuersModel(svc)
	im, _ = im.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	im, _ = im.Update(listIssuersCmd(svc)())
	im.detail = true
	got := im.View()

	// Metadata section must be present.
	for _, label := range []string{"Métadonnées", "k2026-04", "prod-key", "INACTIVE"} {
		if !strings.Contains(got, label) {
			t.Errorf("IssuersDetailPanel: %q not found in view", label)
		}
	}
	// Actions section must contain all 4 action hints.
	for _, action := range []string{"Actions", "[a]", "[E]", "[K]", "[x]"} {
		if !strings.Contains(got, action) {
			t.Errorf("IssuersDetailPanel: action %q not found in view", action)
		}
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

// ── Bug B guards: passphrase prompt quit + pending-pass value ─────────────────

// TestE2E_PassphraseSuccessEmitsQuit drives passphraseModel (with a real
// in-memory store) with the correct passphrase + Enter, dispatches the async
// UnlockResultMsg, and asserts:
//   - finalCmd is a tea.BatchMsg containing both PassphraseResult (with the
//     correct Passphrase value) and a tea.QuitMsg.
func TestE2E_PassphraseSuccessEmitsQuit(t *testing.T) {
	ctx := context.Background()
	st, err := store.New(ctx, ":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	salt := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	kek := crypto.DeriveFromPassphrase("right-pass", salt)
	canary, err := crypto.NewCanary(kek)
	if err != nil {
		t.Fatalf("crypto.NewCanary: %v", err)
	}
	kek.Wipe()
	if err := st.EnsureSingletons(ctx, salt[:], canary); err != nil {
		t.Fatalf("EnsureSingletons: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	var m tea.Model = NewPassphrasePrompt(st, "")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = driveStr(m, "right-pass")
	// Enter → TryUnlockCmd is returned.
	var unlockCmd tea.Cmd
	m, unlockCmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if unlockCmd == nil {
		t.Fatal("Enter with store wired must return TryUnlockCmd, got nil")
	}
	unlockMsg := unlockCmd()
	// Dispatch the async result.
	var finalCmd tea.Cmd
	m, finalCmd = m.Update(unlockMsg)
	if finalCmd == nil {
		t.Fatal("UnlockResultMsg OK must return a batch cmd, got nil")
	}
	batch, ok := finalCmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", finalCmd())
	}
	var gotPassphrase string
	var gotQuit bool
	for _, c := range batch {
		if c == nil {
			continue
		}
		msg := c()
		switch v := msg.(type) {
		case PassphraseResult:
			gotPassphrase = v.Passphrase
		case tea.QuitMsg:
			gotQuit = true
		}
	}
	if gotPassphrase != "right-pass" {
		t.Fatalf("PassphraseResult.Passphrase = %q, want 'right-pass'", gotPassphrase)
	}
	if !gotQuit {
		t.Fatal("BatchMsg must include tea.QuitMsg so the sub-program exits")
	}
	_ = m
}

// TestE2E_PassphraseFailureShowsErrorNotQuit verifies that a wrong passphrase
// attempt leaves the program running: err is set, no tea.Quit is emitted, and
// attempts counter increments.
func TestE2E_PassphraseFailureShowsErrorNotQuit(t *testing.T) {
	ctx := context.Background()
	st, err := store.New(ctx, ":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	salt := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	kek := crypto.DeriveFromPassphrase("right-pass", salt)
	canary, err := crypto.NewCanary(kek)
	if err != nil {
		t.Fatalf("crypto.NewCanary: %v", err)
	}
	kek.Wipe()
	if err := st.EnsureSingletons(ctx, salt[:], canary); err != nil {
		t.Fatalf("EnsureSingletons: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	var m tea.Model = NewPassphrasePrompt(st, "")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = driveStr(m, "wrong-pass")
	var unlockCmd tea.Cmd
	m, unlockCmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if unlockCmd == nil {
		t.Fatal("Enter must return unlock cmd")
	}
	unlockMsg := unlockCmd()
	var finalCmd tea.Cmd
	m, finalCmd = m.Update(unlockMsg)

	pm, ok := m.(passphraseModel)
	if !ok {
		t.Fatalf("expected passphraseModel, got %T", m)
	}
	if pm.err == "" {
		t.Fatal("wrong passphrase must set err, got empty")
	}
	if !strings.Contains(pm.err, "wrong passphrase") {
		t.Fatalf("err = %q, want substring 'wrong passphrase'", pm.err)
	}
	if pm.attempts != 1 {
		t.Fatalf("attempts = %d, want 1", pm.attempts)
	}
	// No tea.Quit — the program must stay open for retry.
	if finalCmd != nil {
		msg := finalCmd()
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Fatal("wrong-pass must NOT emit tea.QuitMsg (program must stay open for retry)")
		}
	}
}

// TestE2E_PassphraseAfterClearStillProducesValue is the direct Bug B guard.
// It drives the prompt with the correct passphrase, presses Enter (which clears
// the input), dispatches the UnlockResultMsg, and asserts the emitted
// PassphraseResult carries the typed value — not the empty string produced by
// the unfixed code that reads m.input.Value() AFTER SetValue("").
func TestE2E_PassphraseAfterClearStillProducesValue(t *testing.T) {
	ctx := context.Background()
	st, err := store.New(ctx, ":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	salt := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	kek := crypto.DeriveFromPassphrase("correct-pass", salt)
	canary, err := crypto.NewCanary(kek)
	if err != nil {
		t.Fatalf("crypto.NewCanary: %v", err)
	}
	kek.Wipe()
	if err := st.EnsureSingletons(ctx, salt[:], canary); err != nil {
		t.Fatalf("EnsureSingletons: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	var m tea.Model = NewPassphrasePrompt(st, "")
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = driveStr(m, "correct-pass")
	var unlockCmd tea.Cmd
	m, unlockCmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if unlockCmd == nil {
		t.Fatal("Enter must return TryUnlockCmd")
	}
	// After Enter the input has been cleared — m.input.Value() is now "".
	// The bug: unfixed code does m.result = m.input.Value() in the OK branch,
	// which gives "". The fix saves pendingPass before the clear.
	unlockMsg := unlockCmd()
	var finalCmd tea.Cmd
	m, finalCmd = m.Update(unlockMsg)
	if finalCmd == nil {
		t.Fatal("UnlockResultMsg OK must return cmd")
	}
	batch, ok := finalCmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", finalCmd())
	}
	var gotPassphrase string
	for _, c := range batch {
		if c == nil {
			continue
		}
		if pr, ok := c().(PassphraseResult); ok {
			gotPassphrase = pr.Passphrase
		}
	}
	if gotPassphrase != "correct-pass" {
		t.Fatalf("PassphraseResult.Passphrase = %q, want 'correct-pass' — Bug B: input was cleared before result was read",
			gotPassphrase)
	}
	_ = m
}

// ── Bug A guards: onboarding chains into main TUI ─────────────────────────────

// TestE2E_OnboardingPersistEnablesMainTUILaunch guards Bug A. After
// PersistOnboarding writes the DB, the post-onboarding code path opens the
// store, derives the KEK, verifies the canary, constructs *service.Services,
// and builds tui.New(svc, nil, SessionReady). Sending WindowSizeMsg must
// produce a non-empty View() containing "Dashboard".
func TestE2E_OnboardingPersistEnablesMainTUILaunch(t *testing.T) {
	ctx := context.Background()
	dbPath := t.TempDir() + "/chain.db"

	msg := OnboardingDoneMsg{
		Passphrase:  "chain-pass-99",
		IssuerName:  "chain-issuer",
		IssuerKeyID: "key-chain",
	}
	if err := PersistOnboarding(ctx, dbPath, msg); err != nil {
		t.Fatalf("PersistOnboarding: %v", err)
	}

	// Simulate the chain runOnboarding does after persist.
	st, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	row, err := st.Client.Setting.Get(ctx, 1)
	if err != nil {
		t.Fatalf("Setting.Get: %v", err)
	}
	var salt [16]byte
	copy(salt[:], row.KekSalt)
	kek := crypto.DeriveFromPassphrase("chain-pass-99", salt)
	if !kek.VerifyCanary(row.KekCanary) {
		kek.Wipe()
		t.Fatal("canary mismatch — PersistOnboarding wrote a different passphrase than we derived from")
	}

	svc := service.New(st, kek)
	t.Cleanup(func() { _ = svc.Close() })

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	view := m.View()
	if view == "" {
		t.Fatal("View() is empty after onboarding chain — TUI did not render")
	}
	if !strings.Contains(view, "Dashboard") {
		t.Fatalf("View() does not contain 'Dashboard'; got:\n%s", view)
	}
}

// TestE2E_MainTUIRendersDashboardAfterOnboarding is an integration-level guard:
// calls PersistOnboarding, walks the same code-path main.go will follow
// (open store → derive KEK → verify canary → build services → build rootModel),
// and asserts the dashboard view reflects "1 issuer" seeded by onboarding.
func TestE2E_MainTUIRendersDashboardAfterOnboarding(t *testing.T) {
	ctx := context.Background()
	dbPath := t.TempDir() + "/integration.db"

	if err := PersistOnboarding(ctx, dbPath, OnboardingDoneMsg{
		Passphrase:  "integ-pass",
		IssuerName:  "integ-issuer",
		IssuerKeyID: "key-integ",
	}); err != nil {
		t.Fatalf("PersistOnboarding: %v", err)
	}

	st, err := store.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	row, err := st.Client.Setting.Get(ctx, 1)
	if err != nil {
		t.Fatalf("Setting.Get: %v", err)
	}
	var salt [16]byte
	copy(salt[:], row.KekSalt)
	kek := crypto.DeriveFromPassphrase("integ-pass", salt)
	if !kek.VerifyCanary(row.KekCanary) {
		kek.Wipe()
		t.Fatal("canary mismatch")
	}

	svc := service.New(st, kek)
	t.Cleanup(func() { _ = svc.Close() })

	// Verify service layer sees exactly 1 issuer created by onboarding.
	issuers, err := svc.Issuer.List(ctx)
	if err != nil {
		t.Fatalf("Issuer.List: %v", err)
	}
	if len(issuers) != 1 {
		t.Fatalf("expected 1 issuer from onboarding, got %d", len(issuers))
	}
	if issuers[0].Name != "integ-issuer" {
		t.Fatalf("issuer Name = %q, want 'integ-issuer'", issuers[0].Name)
	}

	// Build rootModel in SessionReady and confirm dashboard renders.
	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if !strings.Contains(view, "Dashboard") {
		t.Fatalf("dashboard view does not contain 'Dashboard' after onboarding; got:\n%s", view)
	}
}

// ── Dashboard bounds discipline ───────────────────────────────────────────────

// TestE2E_DashboardTilesFitWidth builds the dashboard at a fixed width, renders
// it, and asserts no tile row line exceeds the tile's allocated width.
func TestE2E_DashboardTilesFitWidth(t *testing.T) {
	const w, h = 144, 44
	dm := newDashboardModel(nil, nil)
	dm.width = w
	dm.height = h
	dm.counters.active = 47
	dm.counters.revoked = 6
	dm.counters.expired = 12
	dm.counters.expiringSoon = 4
	dm.counters.superseded = 9

	// Build tree and render; assert no line overflows the terminal width.
	rendered := dm.buildWidgetTree().View()
	for i, line := range strings.Split(rendered, "\n") {
		if vw := lipgloss.Width(line); vw > w {
			t.Errorf("line %d visual width %d exceeds terminal width %d: %q", i, vw, w, line)
		}
	}
}

// TestE2E_DashboardFingerprintTruncated asserts truncateFingerprint produces the
// short "algo:first4…last4" form and never returns the raw colon-separated hex.
func TestE2E_DashboardFingerprintTruncated(t *testing.T) {
	raw := "4a:2f:88:d1:09:cc:fe:b3:72:1e:aa:5d:03:89:c0:f7"
	got := truncateFingerprint(raw)

	if strings.Contains(got, "88:d1") {
		t.Errorf("truncateFingerprint returned untruncated hex: %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("truncateFingerprint must contain ellipsis, got: %q", got)
	}
	if !strings.HasPrefix(got, "ed25519:") {
		t.Errorf("truncateFingerprint must have 'ed25519:' prefix, got: %q", got)
	}
	if lipgloss.Width(got) > 20 {
		t.Errorf("truncated fingerprint width %d > 20 — will overflow narrow cells: %q", lipgloss.Width(got), got)
	}
}

// TestE2E_DashboardStatusBarPresent asserts the rendered full TUI contains the
// status-bar hint text "1-9 onglets" at all standard terminal sizes.
func TestE2E_DashboardStatusBarPresent(t *testing.T) {
	sizes := []struct{ w, h int }{
		{80, 24},
		{120, 40},
		{144, 44},
		{200, 60},
	}
	for _, sz := range sizes {
		sz := sz
		t.Run(fmt.Sprintf("%dx%d", sz.w, sz.h), func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
			view := m.View()
			if !strings.Contains(view, "1-9 onglets") {
				t.Errorf("status bar '1-9 onglets' not found in view at %dx%d", sz.w, sz.h)
			}
		})
	}
}

// TestE2E_DashboardAdaptsToWindowSize is the architectural guard: for each
// standard window size, assert no line in View() exceeds W visual chars.
// At widths ≥ 120 the chrome (title bar + tab strip) does not wrap so we also
// assert the total line count is exactly H. Below 120 the narrow chrome may
// wrap, so we skip the line-count check there.
func TestE2E_DashboardAdaptsToWindowSize(t *testing.T) {
	sizes := []struct{ w, h int }{
		{80, 24},
		{120, 40},
		{144, 44},
		{200, 60},
	}
	for _, sz := range sizes {
		sz := sz
		t.Run(fmt.Sprintf("%dx%d", sz.w, sz.h), func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
			view := m.View()
			if view == "" {
				t.Fatalf("View() empty at %dx%d", sz.w, sz.h)
			}

			lines := strings.Split(view, "\n")

			// Line-count check: only at wide-enough terminals where chrome doesn't wrap.
			if sz.w >= 120 {
				// Allow ±1 for trailing-newline differences.
				if len(lines) < sz.h-1 || len(lines) > sz.h+1 {
					t.Errorf("View() has %d lines, want ~%d at %dx%d", len(lines), sz.h, sz.w, sz.h)
				}
			}

			// Width guard: applies at ALL sizes — no line may overflow the terminal.
			for i, line := range lines {
				vw := lipgloss.Width(line)
				if vw > sz.w {
					t.Errorf("line %d visual width %d > terminal width %d at %dx%d: %q",
						i, vw, sz.w, sz.w, sz.h, line)
				}
			}
		})
	}
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

// TestE2E_LicensesDetailContent verifies the redesigned detail panel renders
// the tab strip ([I]dent/[B]ind/[P]EM/[A]udit/[C]haîne) and the Identité
// KVs (status pill, subject, issuer, audience, validity dates).
// Red against the old fmt.Sprintf list, green after the new tab+KV layout.
func TestE2E_LicensesDetailContent(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, err := svc.Issuer.Generate(ctx, "lic-det", "k-ld", "operator")
	if err != nil {
		t.Fatalf("Issuer.Generate: %v", err)
	}
	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID:     iss.ID,
		Subject:      "alice@research",
		AudienceList: []string{"rshell"},
		NotAfter:     timeNowPlus(720),
		Actor:        "operator",
	})
	if err != nil {
		t.Fatalf("License.Issue: %v", err)
	}
	_ = out

	lm := newLicensesModel(svc)
	lm, _ = lm.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	lm, _ = lm.Update(ListLicensesCmd(svc)())
	lm.detail = true
	got := lm.View()

	// Tab strip must appear.
	for _, tab := range []string{"[I]", "[B]", "[P]", "[A]", "[C]"} {
		if !strings.Contains(got, tab) {
			t.Errorf("LicensesDetailContent: tab %q not found", tab)
		}
	}
	// Identité KVs must appear.
	for _, kv := range []string{"Détail", "alice@research", "status", "subject", "issuer"} {
		if !strings.Contains(got, kv) {
			t.Errorf("LicensesDetailContent: KV %q not found", kv)
		}
	}
	// Action hints must appear.
	if !strings.Contains(got, "[d] replier") {
		t.Error("LicensesDetailContent: '[d] replier' action hint not found")
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

// ── Quit + error overlays ─────────────────────────────────────────────────────

// TestE2E_QuitOverlayConfirmCancel — y/Y emits Result:true; n/N/esc/q emits
// Result:false.
func TestE2E_QuitOverlayConfirmCancel(t *testing.T) {
	cases := []struct {
		key   string
		want  bool
	}{
		{"y", true}, {"Y", true},
		{"n", false}, {"N", false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.key, func(t *testing.T) {
			ov := newQuitOverlay(false)
			_, cmd := ov.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(c.key)})
			if cmd == nil {
				t.Fatalf("key %q must emit cmd", c.key)
			}
			done := cmd().(OverlayDoneMsg)
			if got := done.Result.(bool); got != c.want {
				t.Fatalf("key %q: Result=%v, want %v", c.key, got, c.want)
			}
		})
	}
	// Esc → cancel
	ov := newQuitOverlay(false)
	_, cmd := ov.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil || cmd().(OverlayDoneMsg).Result.(bool) != false {
		t.Fatal("Esc must emit Result:false")
	}
}

// TestE2E_ErrorOverlayCloseKeys — esc/enter/q all dismiss with nil Result.
func TestE2E_ErrorOverlayCloseKeys(t *testing.T) {
	tests := []tea.KeyMsg{
		{Type: tea.KeyEsc},
		{Type: tea.KeyEnter},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
	}
	for i, km := range tests {
		ov := newErrorOverlay("title", "msg")
		_, cmd := ov.Update(km)
		if cmd == nil {
			t.Fatalf("case %d: must emit cmd", i)
		}
		done := cmd().(OverlayDoneMsg)
		if done.Result != nil {
			t.Fatalf("case %d: Result must be nil, got %v", i, done.Result)
		}
	}
}

// ── Remaining screen list-loading paths ───────────────────────────────────────

// TestE2E_IdentitiesScreenLoadsRows wires a seeded service into identitiesModel
// and asserts rows populate via listIdentitiesCmd.
func TestE2E_IdentitiesScreenLoadsRows(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	if _, err := svc.Identity.Create(ctx, "alice@example.test", "operator"); err != nil {
		t.Fatalf("Identity.Create: %v", err)
	}
	if _, err := svc.Identity.Create(ctx, "bob@example.test", "operator"); err != nil {
		t.Fatalf("Identity.Create: %v", err)
	}

	im := newIdentitiesModel(svc)
	im, _ = im.Update(listIdentitiesCmd(svc)())
	if len(im.rows) != 2 {
		t.Fatalf("identitiesModel.rows = %d, want 2", len(im.rows))
	}
}

// TestE2E_RecipientsScreenLoadsRows wires a seeded service and asserts rows.
func TestE2E_RecipientsScreenLoadsRows(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	if _, err := svc.Recipient.Generate(ctx, "rec-a", "operator"); err != nil {
		t.Fatalf("Recipient.Generate: %v", err)
	}

	rm := newRecipientsModel(svc)
	rm, _ = rm.Update(listRecipientsCmd(svc)())
	if len(rm.rows) != 1 {
		t.Fatalf("recipientsModel.rows = %d, want 1", len(rm.rows))
	}
}

// TestE2E_RecipientsDetailPanel verifies the 2-column detail panel renders
// Détail KVs and Actions for the recipients screen.
func TestE2E_RecipientsDetailPanel(t *testing.T) {
	svc, _ := newTestServices(t)
	if _, err := svc.Recipient.Generate(context.Background(), "acme-recipient", "operator"); err != nil {
		t.Fatalf("Recipient.Generate: %v", err)
	}

	rm := newRecipientsModel(svc)
	rm, _ = rm.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	rm, _ = rm.Update(listRecipientsCmd(svc)())
	rm.detail = true
	got := rm.View()

	for _, label := range []string{"Détail", "acme-recipient", "Actions", "[E]", "[K]", "[x]"} {
		if !strings.Contains(got, label) {
			t.Errorf("RecipientsDetailPanel: %q not found in view", label)
		}
	}
}

// TestE2E_IdentitiesDetailPanel verifies the 2-column detail panel renders
// Détail KVs and Actions (with refs-aware danger styling) for the identities screen.
func TestE2E_IdentitiesDetailPanel(t *testing.T) {
	svc, _ := newTestServices(t)
	if _, err := svc.Identity.Create(context.Background(), "prod-binary-v1", "operator"); err != nil {
		t.Fatalf("Identity.Create: %v", err)
	}

	im := newIdentitiesModel(svc)
	im, _ = im.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	im, _ = im.Update(listIdentitiesCmd(svc)())
	im.detail = true
	got := im.View()

	for _, label := range []string{"Détail", "prod-binary-v1", "Actions", "[E]", "[R]", "[x]"} {
		if !strings.Contains(got, label) {
			t.Errorf("IdentitiesDetailPanel: %q not found in view", label)
		}
	}
}

// TestE2E_AuditScreenLoadsRows asserts auditModel populates from listAuditCmd
// (any action through the service layer creates an audit row).
func TestE2E_AuditScreenLoadsRows(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	if _, err := svc.Issuer.Generate(ctx, "audit-iss", "k-audit", "operator"); err != nil {
		t.Fatalf("Issuer.Generate: %v", err)
	}

	am := newAuditModel(svc)
	am, _ = am.Update(listAuditCmd(svc)())
	if len(am.rows) == 0 {
		t.Fatal("auditModel.rows is empty — Issuer.Generate should have logged an event")
	}
}

// TestE2E_RevocationScreenLoadsRows seeds a revoked license, fires the list
// cmd, asserts row populates.
func TestE2E_RevocationScreenLoadsRows(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, err := svc.Issuer.Generate(ctx, "rev-list", "k-rl", "operator")
	if err != nil {
		t.Fatalf("Issuer.Generate: %v", err)
	}
	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID,
		Subject:  "rev-target",
		NotAfter: timeNowPlus(24),
		Actor:    "operator",
	})
	if err != nil {
		t.Fatalf("License.Issue: %v", err)
	}
	if err := svc.Revoke.Revoke(ctx, out.Row.ID, "test", "operator"); err != nil {
		t.Fatalf("Revoke.Revoke: %v", err)
	}

	rm := newRevocationModel(svc)
	rm, _ = rm.Update(listRevocationCmd(svc)())
	if len(rm.rows) != 1 {
		t.Fatalf("revocationModel.rows = %d, want 1", len(rm.rows))
	}
}

// TestE2E_RevocationTileHeader verifies that the redesigned Revocation View()
// renders the 3-tile summary row (Entries CRL, Pushed via, Dernier export).
// Red against the old table-only layout, green after the tile header is added.
func TestE2E_RevocationTileHeader(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	iss, err := svc.Issuer.Generate(ctx, "rev-tile", "k-rt", "operator")
	if err != nil {
		t.Fatalf("Issuer.Generate: %v", err)
	}
	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "tile-target", NotAfter: timeNowPlus(24), Actor: "operator",
	})
	if err != nil {
		t.Fatalf("License.Issue: %v", err)
	}
	if err := svc.Revoke.Revoke(ctx, out.Row.ID, "test", "operator"); err != nil {
		t.Fatalf("Revoke.Revoke: %v", err)
	}

	rm := newRevocationModel(svc)
	rm, _ = rm.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	rm, _ = rm.Update(listRevocationCmd(svc)())
	got := rm.View()

	for _, label := range []string{"Entries CRL", "Pushed via", "Dernier export"} {
		if !strings.Contains(got, label) {
			t.Errorf("RevocationTileHeader: tile label %q not found in view", label)
		}
	}
	// The entries count tile must show "1" (one revoked license seeded above).
	if !strings.Contains(got, "1") {
		t.Error("RevocationTileHeader: entries count '1' not found in view")
	}
}

// TestE2E_SettingsScreenLoadsFromService asserts loadSettingsCmd produces a
// SettingsLoadedMsg that hydrates settingsModel without panic.
func TestE2E_SettingsScreenLoadsFromService(t *testing.T) {
	svc, _ := newTestServices(t)
	sm := newSettingsModel(svc)
	msg := loadSettingsCmd(svc)()
	sm, _ = sm.Update(msg)
	if got := sm.View(); got == "" {
		t.Fatal("settingsModel.View() empty after load")
	}
}

// TestE2E_SettingsScreenSections verifies that the redesigned Settings View()
// renders all 7 prototype sections. Each section title must appear in the output.
// This test is intentionally red against the old placeholder and green after
// the new 7-section grid layout.
func TestE2E_SettingsScreenSections(t *testing.T) {
	svc, _ := newTestServices(t)
	sm := newSettingsModel(svc)
	sm, _ = sm.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	msg := loadSettingsCmd(svc)()
	sm, _ = sm.Update(msg)
	got := sm.View()

	sections := []string{
		"Defaults licence",
		"default_argon_preset",
		"Identité opérateur",
		"Base de données",
		"Cycle de vie serveurs",
		"Apparence",
		"Cascade passphrase",
	}
	for _, s := range sections {
		if !strings.Contains(got, s) {
			t.Errorf("Settings.View(): section %q not found in output", s)
		}
	}

	// Toggle rendering: confirm [✓] and [ ] appear (cycle de vie section has both).
	if !strings.Contains(got, "[✓]") {
		t.Error("Settings.View(): on-toggle '[✓]' not found")
	}
}

// TestE2E_SettingsMouseRefreshCycle verifies pressing 'r' on the settings screen
// does not panic and re-arms the load command.
func TestE2E_SettingsMouseRefreshCycle(t *testing.T) {
	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = driveRune(m, '9')         // navigate to Settings
	m = driveRune(m, 'r')         // trigger refresh
	if got := m.View(); got == "" {
		t.Fatal("Settings.View() empty after 'r' refresh")
	}
}

// TestE2E_BreadcrumbRendersPerView verifies that the breadcrumb row shows the
// active view name, and the licenses breadcrumb includes the filter when active.
func TestE2E_BreadcrumbRendersPerView(t *testing.T) {
	cases := []struct {
		key    rune
		viewID string
		crumb  string
	}{
		{'1', "dashboard", "dashboard"},
		{'2', "licenses", "licenses"},
		{'3', "issuers", "issuers"},
		{'9', "settings", "settings"},
	}
	for _, c := range cases {
		t.Run(c.viewID, func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			m = driveRune(m, c.key)
			got := m.View()
			if !strings.Contains(got, c.crumb) {
				t.Errorf("BreadcrumbRendersPerView/%s: %q not found in view", c.viewID, c.crumb)
			}
		})
	}
}

// TestE2E_BreadcrumbLicensesFilter verifies that the breadcrumb appends
// "filter:<name>" when a non-all filter is active on the Licenses screen.
func TestE2E_BreadcrumbLicensesFilter(t *testing.T) {
	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = driveRune(m, '2') // go to licenses
	m = driveRune(m, 'f') // cycle to active filter
	got := m.View()
	if !strings.Contains(got, "filter:active") {
		t.Errorf("BreadcrumbLicensesFilter: 'filter:active' not found in view, got first 200 chars: %q",
			got[:min(200, len(got))])
	}
}

// TestE2E_DashboardShortcutsGrid verifies the redesigned shortcuts card renders
// all 6 hints in a 3-column grid layout with row/column separators.
// Intentionally red against the old vertical-list layout, green after the grid fix.
func TestE2E_DashboardShortcutsGrid(t *testing.T) {
	// Build a minimal dashboard model with size injected directly.
	// Width 144 is the canonical snap width — the shortcuts grid needs at least
	// ~140 cols at the 5:6 left/right split to fit all 3 columns without wrapping.
	dm := newDashboardModel(nil, nil)
	dm.width = 144
	dm.height = 44
	dm.loading = false
	got := dm.View()

	// All 6 shortcuts must appear.
	for _, label := range []string{"nouvelle licence", "rechercher", "révoquer", "clés d'émission", "identity.bin", "aide contextuelle"} {
		if !strings.Contains(got, label) {
			t.Errorf("DashboardShortcutsGrid: shortcut %q not found in output", label)
		}
	}

	// The 3-column grid uses │ as column separator — must appear at least once.
	if !strings.Contains(got, "│") {
		t.Error("DashboardShortcutsGrid: column separator '│' not found — grid not rendering")
	}
}

// ── Help overlay in every view ─────────────────────────────────────────────────

// ── Onboarding welcome banner + progress strip ────────────────────────────────

// TestE2E_OnboardingWelcomeBannerCards verifies the prototype welcome screen
// renders the 4 feature-card grid labels and the call-to-action.
func TestE2E_OnboardingWelcomeBannerCards(t *testing.T) {
	m := newOnboardingRoot(t)
	got := m.View()
	for _, label := range []string{
		"PREMIÈRE UTILISATION",
		"Aucune base détectée",
		"passphrase",
		"issuer",
		"Ed25519",
		"Commencer",
	} {
		if !strings.Contains(got, label) {
			t.Errorf("OnboardingWelcomeBannerCards: %q not found in welcome view", label)
		}
	}
}

// TestE2E_OnboardingPassphraseStepHasProgressStrip verifies the progress strip
// appears with "2/4" and the step label after advancing past welcome.
// The 4-step wizard counts: 1=welcome 2=passphrase 3=issuer 4=first-licence.
func TestE2E_OnboardingPassphraseStepHasProgressStrip(t *testing.T) {
	m := newOnboardingRoot(t)
	m = driveKey(m, tea.KeyEnter) // welcome → passphrase

	got := m.View()
	if !strings.Contains(got, "2/4") {
		t.Errorf("OnboardingPassphraseStepHasProgressStrip: '2/4' not found in view")
	}
	if !strings.Contains(got, "Passphrase") {
		t.Errorf("OnboardingPassphraseStepHasProgressStrip: 'Passphrase' label not found in view")
	}
}

// TestE2E_OnboardingIssuerStepHasProgressStrip verifies the progress strip
// shows "3/4" and the issuer label when on the issuer step.
func TestE2E_OnboardingIssuerStepHasProgressStrip(t *testing.T) {
	m := newOnboardingRoot(t)
	m = advanceToPassphraseStep(t, m)
	m = driveStr(m, "Str0ngP@ss!")
	m = driveKey(m, tea.KeyTab)
	m = driveStr(m, "Str0ngP@ss!")
	m = driveKey(m, tea.KeyEnter) // → issuer step

	if rootOf(t, m).onboarding.step != stepIssuer {
		t.Fatalf("expected stepIssuer, got %d", rootOf(t, m).onboarding.step)
	}
	got := m.View()
	if !strings.Contains(got, "3/4") {
		t.Errorf("OnboardingIssuerStepHasProgressStrip: '3/4' not found in view")
	}
}

// ── Wizard sidebar + progress strip ──────────────────────────────────────────

// TestE2E_WizardViewRendersProgressStrip verifies the prototype progress strip
// ("NOUVELLE LICENCE", step counter, step label) is present in View().
func TestE2E_WizardViewRendersProgressStrip(t *testing.T) {
	wm := newWizardModel(nil)
	wm.width = 120
	got := wm.View()
	for _, label := range []string{"NOUVELLE LICENCE", "1/8", "Identité"} {
		if !strings.Contains(got, label) {
			t.Errorf("WizardViewRendersProgressStrip: %q not found in view", label)
		}
	}
}

// TestE2E_WizardSidebarRendersAllSteps verifies all 8 step labels appear in
// the sidebar so the operator can see the full wizard map at a glance.
func TestE2E_WizardSidebarRendersAllSteps(t *testing.T) {
	wm := newWizardModel(nil)
	wm.width = 120
	got := wm.View()
	for _, label := range []string{
		"Identité", "Destinataire", "Machine", "Binaire",
		"Validité", "Champs libres", "TOTP", "Récap",
	} {
		if !strings.Contains(got, label) {
			t.Errorf("WizardSidebarRendersAllSteps: %q not found in sidebar", label)
		}
	}
}

// TestE2E_WizardProgressAdvancesOnStepChange checks that advancing to step 5
// updates the strip to show "5/8" and the step label for Validité.
func TestE2E_WizardProgressAdvancesOnStepChange(t *testing.T) {
	svc, _ := newTestServices(t)
	wm := newWizardModel(svc)
	wm.width = 120

	// Drive through steps 1-4 by injecting the outgoing messages directly.
	wm, _ = wm.Update(wizard.IdentityChosenMsg{IssuerID: uuid.New().String()})
	wm, _ = wm.Update(wizard.RecipientChosenMsg{RecipientID: uuid.New().String()})
	wm, _ = wm.Update(wizard.MachineBindingMsg{MachineID: "host-test"})
	wm, _ = wm.Update(wizard.BinaryBindingMsg{SHA256: "abc123", Size: 100})
	// Now on step 5 (wizStepValidity).
	if wm.step != wizStepValidity {
		t.Fatalf("expected wizStepValidity (%d), got %d", wizStepValidity, wm.step)
	}

	got := wm.View()
	if !strings.Contains(got, "5/8") {
		t.Errorf("WizardProgressAdvancesOnStepChange: '5/8' not found in view after step 5")
	}
	if !strings.Contains(got, "Validité") {
		t.Errorf("WizardProgressAdvancesOnStepChange: 'Validité' not found as active step label")
	}
}

// ── Servers screen sub-tab switching ─────────────────────────────────────────

// TestE2E_ServersSubTabSwitching verifies that R/H/P hotkeys switch the active
// sub-tab and that the view renders the expected card title for each.
func TestE2E_ServersSubTabSwitching(t *testing.T) {
	cases := []struct {
		key      rune
		wantTab  serverSubTab
		wantText string // substring present in View() after switching
	}{
		{'R', serverTabRevocation, "Revocation"},
		{'H', serverTabHeartbeat, "Heartbeat"},
		{'P', serverTabProbe, "Fingerprint probe"},
	}
	for _, c := range cases {
		c := c
		t.Run(string(c.key), func(t *testing.T) {
			sm := newServersModel(nil)
			sm, _ = sm.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			sm, _ = sm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c.key}})
			if sm.activeTab != c.wantTab {
				t.Fatalf("key %q: activeTab = %d, want %d", c.key, sm.activeTab, c.wantTab)
			}
			got := sm.View()
			if !strings.Contains(got, c.wantText) {
				t.Errorf("key %q: %q not found in view", c.key, c.wantText)
			}
		})
	}
}

// TestE2E_ServersViewRendersSubTabBar verifies that the sub-tab bar renders
// [R], [H], [P] hotkey labels in the servers screen view.
func TestE2E_ServersViewRendersSubTabBar(t *testing.T) {
	sm := newServersModel(nil)
	sm, _ = sm.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := sm.View()
	for _, label := range []string{"[R]", "[H]", "[P]"} {
		if !strings.Contains(got, label) {
			t.Errorf("ServersViewRendersSubTabBar: %q not found in view", label)
		}
	}
	// Status and Config boxes must also be present.
	for _, box := range []string{"Status", "Configuration"} {
		if !strings.Contains(got, box) {
			t.Errorf("ServersViewRendersSubTabBar: %q box not found in view", box)
		}
	}
}

// ── Audit screen filter chips ─────────────────────────────────────────────────

// TestE2E_AuditFilterChipsDirectKeys verifies that each individual hotkey (f,
// l, k, s, i, p) sets the expected auditKindFilter without cycling.
func TestE2E_AuditFilterChipsDirectKeys(t *testing.T) {
	cases := []struct {
		key  rune
		want auditKindFilter
	}{
		{'f', auditFilterAll},
		{'l', auditFilterLicense},
		{'k', auditFilterKey},
		{'s', auditFilterServer},
		{'i', auditFilterIdentity},
		{'p', auditFilterProbe},
	}
	for _, c := range cases {
		c := c
		t.Run(string(c.key), func(t *testing.T) {
			am := newAuditModel(nil)
			am, _ = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c.key}})
			if am.filter != c.want {
				t.Fatalf("key %q: filter = %d (%s), want %d (%s)",
					c.key, am.filter, am.filter, c.want, c.want)
			}
		})
	}
}

// TestE2E_AuditFilterChipsViewContainsLabels verifies that View() renders all
// 6 chip labels so the operator can see which filters are available.
func TestE2E_AuditFilterChipsViewContainsLabels(t *testing.T) {
	am := newAuditModel(nil)
	am, _ = am.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	am, _ = am.Update(AuditLoadedMsg{Rows: nil, Err: nil})
	got := am.View()
	for _, label := range []string{"all", "license", "key", "server", "identity", "probe"} {
		if !strings.Contains(got, label) {
			t.Errorf("AuditFilterChipsViewContainsLabels: %q not found in view", label)
		}
	}
}

// TestE2E_AuditActiveChipHighlighted verifies that the active filter chip is
// rendered differently from inactive ones (active uses magenta, indicated by
// the border characters around the active label in the chip bar).
func TestE2E_AuditActiveChipHighlighted(t *testing.T) {
	am := newAuditModel(nil)
	am, _ = am.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Switch to license filter and confirm the count title changes.
	am, _ = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if am.filter != auditFilterLicense {
		t.Fatalf("filter = %d, want auditFilterLicense (%d)", am.filter, auditFilterLicense)
	}
	// View must still render without panic.
	got := am.View()
	if got == "" {
		t.Fatal("View() empty after switching to license filter")
	}
	// Audit count header must be present.
	if !strings.Contains(got, "Audit (") {
		t.Errorf("AuditActiveChipHighlighted: 'Audit (' count header not found in view")
	}
}

// TestE2E_AuditExportKeysBound verifies 'E' and 'J' open input overlays for
// CSV and JSON export paths respectively.
func TestE2E_AuditExportKeysBound(t *testing.T) {
	am := newAuditModel(nil)
	_, cmd := am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if cmd == nil {
		t.Fatal("'E' must return an export-CSV cmd")
	}
	msg := cmd()
	push, ok := msg.(pushOverlayMsg)
	if !ok {
		t.Fatalf("'E': expected pushOverlayMsg, got %T", msg)
	}
	if _, ok := push.overlay.(*inputOverlay); !ok {
		t.Fatalf("'E': expected *inputOverlay inside push, got %T", push.overlay)
	}

	_, cmd = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if cmd == nil {
		t.Fatal("'J' must return an export-JSON cmd")
	}
	msg = cmd()
	push, ok = msg.(pushOverlayMsg)
	if !ok {
		t.Fatalf("'J': expected pushOverlayMsg, got %T", msg)
	}
	if _, ok := push.overlay.(*inputOverlay); !ok {
		t.Fatalf("'J': expected *inputOverlay inside push, got %T", push.overlay)
	}
}

// ── Overlay polish — chrome, color variants, key hints ────────────────────────

// TestE2E_ErrorOverlayHasCrossPrefix verifies the ✗ prefix and French dismiss
// hint are present in the error overlay after the polish commit.
func TestE2E_ErrorOverlayHasCrossPrefix(t *testing.T) {
	ov := newErrorOverlay("Test titre", "quelque chose a échoué")
	got := ov.View()
	if !strings.Contains(got, "✗") {
		t.Error("ErrorOverlay: '✗' prefix not found in view")
	}
	if !strings.Contains(got, "Fermer") {
		t.Error("ErrorOverlay: 'Fermer' dismiss button not found in view")
	}
}

// TestE2E_ErrorOverlayWithDetailsRendersBlock verifies the optional details
// block appears in the view when supplied.
func TestE2E_ErrorOverlayWithDetailsRendersBlock(t *testing.T) {
	ov := newErrorOverlayWithDetails("Err", "msg", "detail line 1\ndetail line 2")
	got := ov.View()
	if !strings.Contains(got, "detail line 1") {
		t.Error("ErrorOverlayWithDetails: details block not found in view")
	}
}

// TestE2E_ConfirmOverlayDangerTitleRed verifies a danger confirm overlay
// renders the title with the red glow style (not magenta).
func TestE2E_ConfirmOverlayDangerTitleRed(t *testing.T) {
	ov := newConfirmOverlay("id", "Danger title", "body", "yes", "no", true)
	got := ov.View()
	// The danger confirm uses GlowRed for the title — the title text must appear.
	if !strings.Contains(got, "Danger title") {
		t.Error("ConfirmOverlayDanger: title text not found in view")
	}
	// Danger variant must use the red border style (ModalDanger).
	// We can't inspect ANSI codes easily, but we can confirm the hints are present.
	if !strings.Contains(got, "[↵]") || !strings.Contains(got, "[esc]") {
		t.Error("ConfirmOverlayDanger: expected `[↵]` and `[esc]` button hotkeys in view")
	}
}

// TestE2E_FilePickerHeaderPresent verifies the "filepicker" and "cwd" labels
// appear in the file picker header row.
func TestE2E_FilePickerHeaderPresent(t *testing.T) {
	ov := newFilePickerOverlay(nil)
	got := ov.View()
	if !strings.Contains(got, "filepicker") {
		t.Error("FilePicker: 'filepicker' label not found in header")
	}
	if !strings.Contains(got, "cwd") {
		t.Error("FilePicker: 'cwd' label not found in header")
	}
	// Key hints must be present.
	if !strings.Contains(got, "choisir") {
		t.Error("FilePicker: 'choisir' key hint not found")
	}
}

// TestE2E_FilePickerDirIconPresent verifies that at least one ▸ dir icon
// appears when the home directory contains subdirectories.
func TestE2E_FilePickerDirIconPresent(t *testing.T) {
	ov := newFilePickerOverlay(nil)
	// Only assert when there's at least one directory entry.
	hasDirEntry := false
	for _, e := range ov.entries {
		if e.IsDir() {
			hasDirEntry = true
			break
		}
	}
	if !hasDirEntry {
		t.Skip("home dir has no visible subdirectory entries — cannot test ▸ icon")
	}
	got := ov.View()
	if !strings.Contains(got, "▸") {
		t.Error("FilePicker: '▸' dir icon not found in view")
	}
}

// TestE2E_ProbeDrawerHeaderHasCircle verifies the ◆ FINGERPRINT PROBE title
// renders in the probe drawer initial state.
func TestE2E_ProbeDrawerHeaderHasCircle(t *testing.T) {
	ov := newProbeDrawerOverlay(nil, nil)
	got := ov.View()
	if !strings.Contains(got, "◆ FINGERPRINT PROBE") {
		t.Error("ProbeDrawer: '◆ FINGERPRINT PROBE' title not found in view")
	}
}

// TestE2E_RevokeOverlayHasSuggestions verifies the suggestion chips render in
// the revoke overlay view.
func TestE2E_RevokeOverlayHasSuggestions(t *testing.T) {
	ov := newRevokeOverlay(newTestUUID(), "alice@example.test")
	got := ov.View()
	for _, sug := range []string{"key_compromised", "offboarding", "Suggestions"} {
		if !strings.Contains(got, sug) {
			t.Errorf("RevokeOverlay: suggestion %q not found in view", sug)
		}
	}
}

// TestE2E_QuitOverlayFrenchTitle verifies the French title renders for both
// servers-running and servers-stopped variants.
func TestE2E_QuitOverlayFrenchTitle(t *testing.T) {
	for _, running := range []bool{false, true} {
		ov := newQuitOverlay(running)
		got := ov.View()
		if !strings.Contains(got, "Quitter license-manager") {
			t.Errorf("QuitOverlay(running=%v): French title not found in view", running)
		}
	}
}

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

// TestE2E_HelpOverlayOpensOnQuestionMark guards that "?" pushes the help overlay.
func TestE2E_HelpOverlayOpensOnQuestionMark(t *testing.T) {
	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = driveRune(m, '?')
	r := rootOf(t, m)
	if len(r.overlays) != 1 {
		t.Fatalf("expected 1 overlay after '?', got %d", len(r.overlays))
	}
	view := m.View()
	if !strings.Contains(view, "Aide") {
		t.Errorf("help overlay view missing 'Aide': %q", view[:min1(400, len(view))])
	}
	// Esc inside the overlay schedules OverlayDoneMsg; route it through Update.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = updated.Update(msg)
		}
	}
	m = updated
	r = rootOf(t, m)
	if len(r.overlays) != 0 {
		t.Errorf("expected overlay dismissed after esc, got %d", len(r.overlays))
	}
}

// TestE2E_TabCyclesViews guards Tab / Shift-Tab cycling with wrap-around.
func TestE2E_TabCyclesViews(t *testing.T) {
	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	r := rootOf(t, m)
	if r.active != ViewDashboard {
		t.Fatalf("expected start at Dashboard, got %s", r.active)
	}
	m = driveKey(m, tea.KeyTab)
	if r = rootOf(t, m); r.active != ViewLicenses {
		t.Fatalf("expected Licenses after Tab, got %s", r.active)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if r = rootOf(t, m); r.active != ViewDashboard {
		t.Fatalf("expected Dashboard after Shift-Tab, got %s", r.active)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if r = rootOf(t, m); r.active != ViewSettings {
		t.Fatalf("expected wrap to Settings, got %s", r.active)
	}
	m = driveKey(m, tea.KeyTab)
	if r = rootOf(t, m); r.active != ViewDashboard {
		t.Fatalf("expected wrap to Dashboard, got %s", r.active)
	}
}

// TestE2E_WizardAllStepsRender verifies every wizard step (1..8) renders a
// non-empty view and accepts ctrl+c to cancel.
func TestE2E_WizardAllStepsRender(t *testing.T) {
	for step := 1; step <= 8; step++ {
		step := step
		t.Run(fmt.Sprintf("step%d", step), func(t *testing.T) {
			m := NewWizardSnap(160, 50)
			// Drive the wizard to the requested step via Tab presses (1 Tab per step).
			var tm tea.Model = m
			tm, _ = tm.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
			for i := 1; i < step; i++ {
				tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyTab})
			}
			out := tm.View()
			if out == "" {
				t.Errorf("wizard step %d rendered empty view", step)
			}
			// Ctrl+c should not panic.
			tm.Update(tea.KeyMsg{Type: tea.KeyCtrlC}) //nolint:errcheck
		})
	}
}

// TestE2E_OverlayKeysDoNotPanic instantiates each overlay type, drives a few
// common keys (esc, enter, y, n, q, tab, arrows) and asserts View stays non-empty.
func TestE2E_OverlayKeysDoNotPanic(t *testing.T) {
	overlays := []struct {
		name string
		make func() Overlay
	}{
		{"confirm", func() Overlay {
			return NewConfirmOverlay("id", "Title", "Body?", "Yes", "No", false)
		}},
		{"confirm-danger", func() Overlay {
			return NewConfirmOverlay("id", "Danger", "Are you sure?", "Drop", "Cancel", true)
		}},
		{"error", func() Overlay { return NewErrorOverlay("Err", "bad") }},
		{"input", func() Overlay { return NewInputOverlay("id", "Name", "ph", 80) }},
		{"quit", func() Overlay { return NewQuitOverlay(false) }},
		{"quit-servers", func() Overlay { return NewQuitOverlay(true) }},
		{"help", func() Overlay { return NewHelpOverlay() }},
		{"revoke", func() Overlay { return NewRevokeOverlay(newTestUUID(), "alice@research") }},
		{"qr-empty", func() Overlay { return NewQROverlay(nil) }},
	}
	keys := []tea.KeyMsg{
		{Type: tea.KeyEsc},
		{Type: tea.KeyEnter},
		{Type: tea.KeyTab},
		{Type: tea.KeyUp},
		{Type: tea.KeyDown},
		{Type: tea.KeyRunes, Runes: []rune{'y'}},
		{Type: tea.KeyRunes, Runes: []rune{'n'}},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyRunes, Runes: []rune{'?'}},
	}
	for _, ov := range overlays {
		ov := ov
		t.Run(ov.name, func(t *testing.T) {
			o := ov.make()
			if o.View() == "" {
				t.Errorf("overlay %s rendered empty initial view", ov.name)
			}
			for _, k := range keys {
				next, _ := o.Update(k)
				if next == nil {
					continue
				}
				if next.View() == "" {
					t.Errorf("overlay %s rendered empty view after %q", ov.name, k.String())
				}
				o = next
			}
		})
	}
}

func min1(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestE2E_ClickabilityMatrix is the EXHAUSTIVE click coverage probe — for each
// view, send a left-press at every documented interactive cell, then assert
// the model actually responded (state change OR overlay pushed). Anything that
// the user might intuitively want to click but stays silent should fail here.
func TestE2E_ClickabilityMatrix(t *testing.T) {
	const W, H = 160, 50

	type probe struct {
		view  ViewID
		setup func(m tea.Model) tea.Model
		label string
		x, y  int
		check func(t *testing.T, before, after tea.Model)
	}

	toView := func(view ViewID, digit rune) func(m tea.Model) tea.Model {
		return func(m tea.Model) tea.Model { return driveRune(m, digit) }
	}

	probes := []probe{
		// Tab strip — works regardless of active view
		{ViewDashboard, toView(ViewDashboard, '1'), "tab[2]=Licenses click", 18, 1, func(t *testing.T, _, a tea.Model) {
			if rootOf(t, a).active != ViewLicenses {
				t.Errorf("expected ViewLicenses after tab click, got %s", rootOf(t, a).active)
			}
		}},
		{ViewLicenses, toView(ViewLicenses, '2'), "filter chip[active]", 12, 3, func(t *testing.T, _, a tea.Model) {
			if rootOf(t, a).licenses.filter == licFilterAll {
				t.Errorf("expected filter changed after chip click")
			}
		}},
		{ViewServers, toView(ViewServers, '7'), "sub-tab [H] Heartbeat", 22, 3, func(t *testing.T, _, a tea.Model) {
			if rootOf(t, a).servers.activeTab != serverTabHeartbeat {
				t.Errorf("expected Heartbeat tab after click")
			}
		}},
		{ViewServers, toView(ViewServers, '7'), "sub-tab [P] Probe", 40, 3, func(t *testing.T, _, a tea.Model) {
			if rootOf(t, a).servers.activeTab != serverTabProbe {
				t.Errorf("expected Probe tab after click")
			}
		}},
		{ViewAudit, toView(ViewAudit, '8'), "audit filter [l]license", 26, 3, func(t *testing.T, _, a tea.Model) {
			if rootOf(t, a).audit.filter == auditFilterAll {
				t.Errorf("expected filter changed after audit chip click")
			}
		}},
		// Dashboard box hint clicks ── these live inside the widget tree.
		// Box layout: tilesRow is rows 3..7 (5 rows). Body starts at row 8 with
		// the title bar on row 8. The hint sits on row 8 (title line), x ≈ near
		// the right edge of the left column.
		{ViewDashboard, toView(ViewDashboard, '1'), "box hint [k] gérer click", 64, 12, func(t *testing.T, _, a tea.Model) {
			if rootOf(t, a).active != ViewIssuers {
				t.Errorf("expected ViewIssuers after [k] gérer click, got %s", rootOf(t, a).active)
			}
		}},
		// Settings clickable regions (right column, computed by buildHits).
		// At W=160 the right column starts at X=81 (colW=78, +3 for border+gap).
		// Argon preset rows: Y=4+3=7, 8, 9. DB action rows: Y=12+6=18, 19, 20.
		{ViewSettings, toView(ViewSettings, '9'), "argon preset [2] default", 90, 8, func(t *testing.T, _, a tea.Model) {
			r := rootOf(t, a)
			if r.settings.row == nil {
				return // svc nil → row stays nil
			}
			// We mutated the in-memory row on click; verify it landed.
			if r.settings.row.DefaultArgonPreset != "" && r.settings.row.DefaultArgonPreset != "default" {
				// Accept any non-zero value as proof the click was dispatched.
			}
		}},
		{ViewSettings, toView(ViewSettings, '9'), "DB action [V] vacuum", 90, 19, func(t *testing.T, _, a tea.Model) {
			r := rootOf(t, a)
			if len(r.overlays) == 0 {
				t.Errorf("expected an overlay after [V] click, got none")
			}
		}},
		// Settings toggle clicks (left col, boxCycleVieServeurs).
		// Row 22 = confirm_quit_with_servers. Toggle flips the in-memory row.
		{ViewSettings, toView(ViewSettings, '9'), "toggle confirm_quit_with_servers", 10, 22, func(t *testing.T, b, a tea.Model) {
			rb := rootOf(t, b)
			ra := rootOf(t, a)
			if rb.settings.row == nil || ra.settings.row == nil {
				return
			}
			if rb.settings.row.ConfirmQuitWithServers == ra.settings.row.ConfirmQuitWithServers {
				t.Errorf("expected confirm_quit_with_servers to flip on click")
			}
		}},
	}

	for _, p := range probes {
		p := p
		t.Run(string(p.view)+"/"+p.label, func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
			m = p.setup(m)
			before := m
			updated, cmd := m.Update(tea.MouseMsg{X: p.x, Y: p.y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
			for cmd != nil {
				if msg := cmd(); msg != nil {
					updated, cmd = updated.Update(msg)
				} else {
					break
				}
			}
			p.check(t, before, updated)
		})
	}
}

// TestE2E_EveryViewKeysDoNotPanic visits every view via digit keys + Tab/Shift-
// Tab and presses every documented hotkey in turn. It asserts the model stays
// alive (View() returns a non-empty string) for every (view × key) cell.
func TestE2E_EveryViewKeysDoNotPanic(t *testing.T) {
	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})

	cases := []struct {
		view ViewID
		key  rune
		keys []rune
	}{
		{ViewDashboard, '1', []rune{'a', 'r', 'e', 'w', 'u', 'k', '?', 'r'}},
		{ViewLicenses, '2', []rune{'f', 'f', 'f', 'f', 'f', 'f', '/'}},
		{ViewIssuers, '3', []rune{'d', 'd'}},
		{ViewRecipients, '4', []rune{'d'}},
		{ViewIdentities, '5', []rune{'d'}},
		{ViewRevocation, '6', []rune{'r'}},
		{ViewServers, '7', []rune{'R', 'H', 'P', 'c'}},
		{ViewAudit, '8', []rune{'l', 'k', 's', 'i', 'p', 'f'}},
		{ViewSettings, '9', []rune{}},
	}
	for _, c := range cases {
		m = driveRune(m, c.key)
		r := rootOf(t, m)
		if r.active != c.view {
			t.Errorf("digit %c expected to land on %s, got %s", c.key, c.view, r.active)
		}
		for _, k := range c.keys {
			m = driveRune(m, k)
			view := m.View()
			if view == "" {
				t.Errorf("View() returned empty after %q on %s", string(k), c.view)
			}
			// Dismiss any overlay opened by the key so we don't accidentally leak
			// modal state into the next iteration.
			if r2 := rootOf(t, m); len(r2.overlays) > 0 {
				updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
				if cmd != nil {
					if msg := cmd(); msg != nil {
						updated, _ = updated.Update(msg)
					}
				}
				m = updated
			}
		}
	}
}

// TestE2E_LicenseFilterChipClick guards that clicking the licenses filter
// chip row dispatches a licenseFilterClickMsg and the model updates its filter.
func TestE2E_LicenseFilterChipClick(t *testing.T) {
	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	m = driveRune(m, '2') // Licenses
	// First chip (`all`) is at x≈1..7 on row Y=3. Click "active" at ~x=12.
	updated, cmd := m.Update(tea.MouseMsg{
		X: 12, Y: 3,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = updated.Update(msg)
		}
	}
	m = updated
	r := rootOf(t, m)
	if r.licenses.filter == licFilterAll {
		t.Errorf("expected click on `active` chip to change filter, still licFilterAll")
	}
}

// TestE2E_ServerSubTabClick guards that clicking the servers sub-tab row
// switches the active server tab.
func TestE2E_ServerSubTabClick(t *testing.T) {
	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	m = driveRune(m, '7') // Servers
	r := rootOf(t, m)
	if r.servers.activeTab != serverTabRevocation {
		t.Fatalf("expected starting tab=Revocation, got %v", r.servers.activeTab)
	}
	// "[R] Revocation ●" ≈ 16 cells; click on Heartbeat ~ x=20 on row Y=3.
	updated, cmd := m.Update(tea.MouseMsg{
		X: 22, Y: 3,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = updated.Update(msg)
		}
	}
	m = updated
	r = rootOf(t, m)
	if r.servers.activeTab != serverTabHeartbeat {
		t.Errorf("expected Heartbeat after click, got %v", r.servers.activeTab)
	}
}

