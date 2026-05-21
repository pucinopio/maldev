package tui

// Workflow E2E tests — drive rootModel exactly as the bubbletea runtime does
// (WindowSize → key messages → assert state) without requiring a TTY.
//
// Package: tui (white-box) so we can inspect unexported fields directly.
// Tests are intentionally written BEFORE the corresponding bug-fixes so each
// test is red against the unfixed code and green after the fix.

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store"
)

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
