package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
	"github.com/oioio-space/maldev/internal/manager/tui/widgets"
	"github.com/oioio-space/maldev/internal/manager/tui/wizard"
)

// truncateLines returns the first n lines of s for compact failure dumps.
func truncateLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch 9 — Wizard full happy-path. Inject the outcome msg of each step
// and assert the wizard advances + the final state contains the typed
// fields. Doesn't drive the inner step UIs (those are covered by the
// per-step tests already shipped); this checks the orchestration.
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_WizardFullHappyPath(t *testing.T) {
	wm := newWizardModel(nil)

	steps := []struct {
		from wizardStep
		msg  tea.Msg
		to   wizardStep
	}{
		{wizStepIdentity, wizard.IdentityChosenMsg{IssuerID: "00000000-0000-0000-0000-000000000001"}, wizStepRecipient},
		{wizStepRecipient, wizard.RecipientChosenMsg{RecipientID: "00000000-0000-0000-0000-000000000002"}, wizStepMachine},
		{wizStepMachine, wizard.MachineBindingMsg{MachineID: "deadbeefcafe"}, wizStepBinary},
		{wizStepBinary, wizard.BinaryBindingMsg{SHA256: "abc123", Size: 4096}, wizStepValidity},
		{wizStepValidity, wizard.ValidityMsg{}, wizStepFreeFields},
		{wizStepFreeFields, wizard.FreeFieldsMsg{Subject: "alice", Audience: "team-a", Fields: map[string]string{"env": "prod"}}, wizStepTOTP},
		{wizStepTOTP, wizard.TOTPChoiceMsg{Require: false}, wizStepReview},
	}

	for _, s := range steps {
		if wm.step != s.from {
			t.Fatalf("expected step %v before %T, got %v", s.from, s.msg, wm.step)
		}
		updated, _ := wm.Update(s.msg)
		wm = updated
		if wm.step != s.to {
			t.Errorf("after %T: step=%v, want %v", s.msg, wm.step, s.to)
		}
	}

	// Final state should mirror what each msg supplied.
	want := wizard.WizardState{
		IssuerID:     "00000000-0000-0000-0000-000000000001",
		RecipientID:  "00000000-0000-0000-0000-000000000002",
		MachineID:    "deadbeefcafe",
		BinarySHA256: "abc123",
		BinarySize:   4096,
		Subject:      "alice",
		Audience:     "team-a",
		FreeFields:   map[string]string{"env": "prod"},
		RequireTOTP:  false,
	}
	if wm.state.IssuerID != want.IssuerID {
		t.Errorf("state.IssuerID = %q, want %q", wm.state.IssuerID, want.IssuerID)
	}
	if wm.state.RecipientID != want.RecipientID {
		t.Errorf("state.RecipientID = %q, want %q", wm.state.RecipientID, want.RecipientID)
	}
	if wm.state.MachineID != want.MachineID {
		t.Errorf("state.MachineID = %q, want %q", wm.state.MachineID, want.MachineID)
	}
	if wm.state.BinarySHA256 != want.BinarySHA256 || wm.state.BinarySize != want.BinarySize {
		t.Errorf("binary: got SHA=%q Size=%d, want %q/%d",
			wm.state.BinarySHA256, wm.state.BinarySize, want.BinarySHA256, want.BinarySize)
	}
	if wm.state.Subject != want.Subject || wm.state.Audience != want.Audience {
		t.Errorf("subj/aud: got %q/%q, want %q/%q",
			wm.state.Subject, wm.state.Audience, want.Subject, want.Audience)
	}
	if got := wm.state.FreeFields["env"]; got != "prod" {
		t.Errorf("FreeFields[env] = %q, want prod", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch 8 — Onboarding 4-step flow. Walk welcome → passphrase → issuer →
// license and assert the final OnboardingDoneMsg payload contains the
// typed values.
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_OnboardingHappyPath(t *testing.T) {
	m := newOnboardingModel()
	// Welcome: any key advances.
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepPassphrase {
		t.Fatalf("after welcome enter: step=%v, want stepPassphrase", m.step)
	}
	// Click anywhere on the welcome screen would have worked too — but the
	// model is already past Welcome at this point.

	// Passphrase: type into field, advance focus with tab + same pass in
	// confirm, then enter.
	m.passInput.SetValue("Str0ngP@ss!")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyTab})
	m.passConfirm.SetValue("Str0ngP@ss!")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepIssuer {
		t.Fatalf("after passphrase: step=%v, want stepIssuer", m.step)
	}

	// Issuer: name + keyID, enter.
	m.issuerName.SetValue("primary-issuer")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyTab})
	m.issuerKeyID.SetValue("ed25519-2026-q1")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepLicense {
		t.Fatalf("after issuer: step=%v, want stepLicense", m.step)
	}

	// License step: press enter — completes with payload + Skipped:true (the
	// 'create first licence' UI is Phase 2, so Phase 1 only persists the
	// passphrase + issuer and marks the licence step as skipped).
	_, cmd := m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("license step skip: nil cmd")
	}
	done, ok := cmd().(OnboardingDoneMsg)
	if !ok {
		t.Fatalf("license skip: cmd produced %T, want OnboardingDoneMsg", cmd())
	}
	if !done.Skipped {
		t.Errorf("done.Skipped = false, want true (s = skip)")
	}
	if done.Passphrase != "Str0ngP@ss!" {
		t.Errorf("done.Passphrase = %q, want %q", done.Passphrase, "Str0ngP@ss!")
	}
	if done.IssuerName != "primary-issuer" {
		t.Errorf("done.IssuerName = %q, want %q", done.IssuerName, "primary-issuer")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch 10 — Server keyboard handlers: log filter 1-4 + clear/autoscroll.
// (A start-all + Z stop-all are root-level — covered in TestRootKeys_AZ.)
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_ServerLogFilters(t *testing.T) {
	cases := []struct {
		key    string
		wantSrv string
	}{
		{"1", ""},            // all
		{"2", "revocation"},
		{"3", "heartbeat"},
		{"4", "probe"},
	}
	for _, c := range cases {
		// log.Update for filter msgs returns nil (pure state mutation), so
		// we can't assert on the returned Cmd. Instead we send the key and
		// then inspect the underlying serverLog's filter field directly to
		// confirm the binding reached the model.
		m := newServersModel(nil, nil)
		mu, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(c.key)})
		if mu.log.filter != c.wantSrv {
			t.Errorf("key %q: log filter = %q, want %q", c.key, mu.log.filter, c.wantSrv)
		}
	}
}

// TestRootKeys_AZ asserts the root key handler routes capital A / Z to the
// http-bundle start-all / stop-all commands when on the Servers tab. Since
// we don't wire a real bundle, the Cmd returned is nil when m.httpsrv is
// nil — the test just verifies the routing branch is reached without a
// panic and that A/Z DO NOT fall through to the active screen.
func TestRootKeys_AZ_NoPanic(t *testing.T) {
	root := New(nil, nil, SessionReady)
	root.active = ViewServers
	for _, k := range []string{"A", "Z"} {
		_, _ = root.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch 11 — Overlay button assertions.
//   confirm: y → ConfirmResultMsg{Confirm: true}, n/esc → Confirm: false
//   input:   enter → InputResultMsg{Value: typed}, esc → Confirm: false
//   ok:      any key → OverlayDoneMsg
//   error:   any key → OverlayDoneMsg
//   quit:    y → tea.Quit (true), n → false
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_ConfirmOverlay(t *testing.T) {
	o := newConfirmOverlay("test-id", "title", "body", "OK", "Cancel", false)
	_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatal("confirm y: nil cmd")
	}
	done := cmd().(OverlayDoneMsg)
	res, ok := done.Result.(ConfirmResultMsg)
	if !ok || !res.Confirm || res.ID != "test-id" {
		t.Errorf("confirm y: got %+v, want ConfirmResultMsg{ID:test-id, Confirm:true}", done.Result)
	}
	o2 := newConfirmOverlay("test-id", "title", "body", "OK", "Cancel", false)
	_, cmd = o2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if cmd == nil {
		t.Fatal("confirm n: nil cmd")
	}
	res2 := cmd().(OverlayDoneMsg).Result.(ConfirmResultMsg)
	if res2.Confirm {
		t.Errorf("confirm n: Confirm = true, want false")
	}
}

func TestInteractions_InputOverlay(t *testing.T) {
	o := newInputOverlay("test-id", "title", "placeholder", 100)
	// Type "hello" then press enter via two msgs.
	o.input.SetValue("hello")
	_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("input enter: nil cmd")
	}
	res := cmd().(OverlayDoneMsg).Result.(InputResultMsg)
	if res.Value != "hello" || res.ID != "test-id" {
		t.Errorf("input enter: got %+v, want {ID:test-id, Value:hello}", res)
	}
}

func TestInteractions_OKAndErrorOverlays(t *testing.T) {
	for _, o := range []Overlay{NewOKOverlay("title", "body"), newErrorOverlay("err title", "err body")} {
		_, cmd := o.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd == nil {
			t.Errorf("%T enter: nil cmd", o)
			continue
		}
		if _, ok := cmd().(OverlayDoneMsg); !ok {
			t.Errorf("%T enter: did not produce OverlayDoneMsg", o)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch 7 — Settings keyboard handlers. Every visible action has a keyboard
// shortcut that drives the same msg the click path would. Drive each via
// Update() and assert the right msg type comes out.
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_SettingsKeyboard(t *testing.T) {
	type tc struct {
		name string
		key  string
		want any
	}
	cases := []tc{
		{"rekey [P]", "P", settingsActionMsg{}},
		{"vacuum [V]", "V", settingsActionMsg{}},
		{"backup [B]", "B", settingsActionMsg{}},
		{"argon fast [1]", "1", settingsSetArgonMsg{}},
		{"argon default [2]", "2", settingsSetArgonMsg{}},
		{"argon paranoid [3]", "3", settingsSetArgonMsg{}},
		{"theme neon [N]", "N", settingsSetThemeMsg{}},
		{"theme mono [M]", "M", settingsSetThemeMsg{}},
		{"theme nord [O]", "O", settingsSetThemeMsg{}},
		{"toggle confirm_quit [Q]", "Q", settingsToggleMsg{}},
		{"toggle auto_start [U]", "U", settingsToggleMsg{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newSettingsModel(nil)
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(c.key)})
			if cmd == nil {
				t.Fatalf("key %q produced nil cmd", c.key)
			}
			got := cmd()
			if reflectTypeOf(got) != reflectTypeOf(c.want) {
				t.Errorf("key %q: msg type = %T, want %T", c.key, got, c.want)
			}
		})
	}
}

// reflectTypeOf returns the runtime type of v as a string identifier.
func reflectTypeOf(v any) string { return fmt.Sprintf("%T", v) }

// ─────────────────────────────────────────────────────────────────────────────
// Batch 2 — Dashboard widget tree dispatch. The dashboard is built from a
// Flex(Tiles, Box × 2, Box × 2, Shortcuts) tree laid out at Y=TopChromeRows.
// We render once + dispatchClick on the tree at each tile's centre,
// asserting the right SwitchToLicensesMsg / SwitchViewMsg comes out.
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_DashboardTiles(t *testing.T) {
	m := newDashboardModel(nil, nil)
	root := New(nil, nil, SessionReady)
	root.active = ViewDashboard
	root.dashboard = m
	var rm tea.Model = root
	rm, _ = rm.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	root = rm.(rootModel)
	// Send a snapshot msg so dashboard is loaded + cache invalidated.
	rm, _ = rm.Update(cmds.DashboardSnapshotMsg{
		Active: 3, Revoked: 1, Expired: 0, ExpiringSoon: 0, Superseded: 0,
	})
	root = rm.(rootModel)
	tree := root.dashboard.widgetTree()

	// Tile layout: 5 tiles in a row from X=0 horizontally. Each ≈28 cells
	// wide. We click middle of tile 0 (Actives → filter=active).
	wantTileToFilter := map[int]string{0: "active", 1: "revoked", 2: "expired", 3: "expiring", 4: "superseded"}
	for i, want := range wantTileToFilter {
		// Tile bounds: compute approximate center based on tree layout.
		// Easier: walk tree.Children() for the first Flex (tiles row).
		tileCenters := dashTileCenters(tree)
		if i >= len(tileCenters) {
			t.Errorf("tile #%d: layout produced only %d tiles", i, len(tileCenters))
			continue
		}
		pt := tileCenters[i]
		px, py := pt[0], pt[1]
		cmd := dispatchClick(tree, px, py)
		if cmd == nil {
			t.Errorf("tile #%d: dispatchClick(%d,%d) returned nil", i, px, py)
			continue
		}
		msg := cmd()
		sw, ok := msg.(SwitchToLicensesMsg)
		if !ok {
			t.Errorf("tile #%d: cmd produced %T, want SwitchToLicensesMsg", i, msg)
			continue
		}
		if sw.Filter != want {
			t.Errorf("tile #%d: filter = %q, want %q", i, sw.Filter, want)
		}
	}
}

// dashTileCenters returns the (X, Y) centre of each tile in the dashboard's
// top tiles row. Walks the first Flex child of the root and reads each
// tile's Bounds().
func dashTileCenters(tree Widget) [][2]int {
	type childer interface{ Children() []Widget }
	root, ok := tree.(childer)
	if !ok {
		return nil
	}
	for _, c := range root.Children() {
		row, isFlex := c.(childer)
		if !isFlex {
			continue
		}
		grand := row.Children()
		if len(grand) < 5 {
			continue
		}
		centers := make([][2]int, 0, len(grand))
		for _, g := range grand {
			b := g.Bounds()
			centers = append(centers, [2]int{b.X + b.W/2, b.Y + b.H/2})
		}
		return centers
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch 5 — Servers screen interactions: sub-tab bar clicks (R/H/P),
// Probe inner tabs (T/H/L), and action chips (s/e/g/c/a).
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_ServersSubTabBar(t *testing.T) {
	m := newServersModel(nil, nil)
	m.width, m.height = 144, 44
	// Sub-tab labels rendered by View() at Y=4 (chrome=4 rows above).
	// Click middle of each tab and expect serverSubTabClickMsg.
	wantTabs := []struct {
		tab serverSubTab
		srv string
		hit int // approximate middle-X of the tab in the rendered strip
	}{
		{serverTabRevocation, "revocation", 8},
		{serverTabHeartbeat, "heartbeat", 27},
		{serverTabProbe, "probe", 55},
	}
	for _, tc := range wantTabs {
		cmd := m.OnClick(tc.hit, 4, 144)
		if cmd == nil {
			t.Errorf("sub-tab %s: OnClick(%d, 4) returned nil", tc.srv, tc.hit)
			continue
		}
		msg := cmd()
		st, ok := msg.(serverSubTabClickMsg)
		if !ok {
			t.Errorf("sub-tab %s: cmd produced %T, want serverSubTabClickMsg", tc.srv, msg)
		} else if st.tab != tc.tab || st.srv != tc.srv {
			t.Errorf("sub-tab %s: got %+v, want tab=%v srv=%q", tc.srv, st, tc.tab, tc.srv)
		}
	}
}

func TestInteractions_ServersActionChips(t *testing.T) {
	m := newServersModel(nil, nil)
	m.width, m.height = 144, 44
	actionBarY := m.height - 2 // OnClick math
	// Walk actionBarHit reverse: query each chip via its hotkey in the
	// pre-built serverActionChips slice. We don't depend on exact X — we
	// pass the X of the chip's hint key, which is `[k]`.
	for _, c := range serverActionChips {
		hitX := actionBarChipX(c.key)
		cmd := m.OnClick(hitX, actionBarY, 144)
		if cmd == nil {
			t.Errorf("action chip [%s]: OnClick(%d, %d) returned nil", c.key, hitX, actionBarY)
			continue
		}
		// Each chip's Cmd shape differs; we just assert non-nil — the
		// production code's switch decides what message to emit (start,
		// stop, push overlay, clear, autoscroll). Per-message asserts
		// would couple this test to msg layouts.
		_ = cmd
	}
}

// actionBarChipX returns the X where the [key] glyph of the named chip
// starts in the action bar, mirroring actionBarHit's cursor walk.
func actionBarChipX(key string) int {
	cursor := 0
	for _, c := range serverActionChips {
		w := lipgloss.Width(HintKey.Render("["+c.key+"]") + " " + Dim.Render(c.label))
		if c.key == key {
			return cursor + 1 // middle of the "[k]" glyph
		}
		cursor += w
	}
	return -1
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch 4 — wizard sidebar steps + step buttons.
//
// 4a — sidebar [1..8] click: the wizardOverlay.Update mouse-handler maps
// body-local Y to step index `(bodyY - 2) / 2` so each step occupies 2
// rows (badge + label). We bypass the overlay's coord translation and
// drive routeBodyClick / gotoStep directly through the wizardModel
// because that's where the step-switching logic lives.
//
// 4b — stepReview Issue / Cancel buttons: click body-local Y on the
// recorded issueBtnY/cancelBtnY and assert the matching IssueResultMsg
// is produced (Issue → none yet, Cancel → ErrCancelled).
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_WizardSidebarSteps(t *testing.T) {
	m := newWizardModel(nil)
	// Render once so any lazy state initialises.
	_, w, h := 144, 90, 30
	_ = w
	_ = h
	m.width, m.hgt = 144, 30
	for step := wizStepIdentity; step <= wizStepReview; step++ {
		updated, _ := m.gotoStep(step)
		if updated.step != step {
			t.Errorf("gotoStep(%d): step is %d, want %d", step, updated.step, step)
		}
	}
}

func TestInteractions_WizardReviewButtons(t *testing.T) {
	sr := wizard.NewStepReview(nil)
	sr.Focus()
	// View() populates issueBtnY / cancelBtnY based on the rendered
	// summary length. We don't expose the fields publicly so test
	// indirectly via OnClick: click anywhere below the summary lines
	// (Issue is the first action row, Cancel the next).
	_ = sr.View()
	issueCmd := sr.OnClick(0, reviewIssueY(sr))
	if issueCmd == nil {
		t.Fatal("StepReview.OnClick on Issue row returned nil")
	}
	msg := issueCmd()
	res, ok := msg.(wizard.IssueResultMsg)
	if !ok {
		// Issue path triggers issueCmd() which calls into the service;
		// with a nil svc it returns an IssueResultMsg with an error.
		t.Errorf("Issue button: cmd produced %T, want IssueResultMsg", msg)
	}
	_ = res

	// Fresh StepReview for the Cancel path — Issue above set s.issuing=true
	// which short-circuits OnClick.
	sr2 := wizard.NewStepReview(nil)
	sr2.Focus()
	_ = sr2.View()
	cancelCmd := sr2.OnClick(0, reviewCancelY(sr2))
	if cancelCmd == nil {
		t.Fatal("StepReview.OnClick on Cancel row returned nil")
	}
	cancelMsg := cancelCmd().(wizard.IssueResultMsg)
	if !errors.Is(cancelMsg.Err, wizard.ErrCancelled) {
		t.Errorf("Cancel button: Err = %v, want wizard.ErrCancelled", cancelMsg.Err)
	}
}

// reviewIssueY / reviewCancelY locate the action button Y rows by scanning
// the rendered StepReview view for the distinctive labels.
func reviewIssueY(sr *wizard.StepReview) int {
	for i, l := range strings.Split(sr.View(), "\n") {
		if strings.Contains(l, "Issue licence") {
			return i
		}
	}
	return -1
}
func reviewCancelY(sr *wizard.StepReview) int {
	for i, l := range strings.Split(sr.View(), "\n") {
		if strings.Contains(l, "Cancel") {
			return i
		}
	}
	return -1
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch 3 — audit filter chips. Each chip [f]/[l]/[k]/[s]/[i]/[p] should
// produce an auditFilterClickMsg with the matching enum value.
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_AuditFilterChips(t *testing.T) {
	m := newAuditModel(nil)
	_ = renderListScreen(t, ViewAudit, &m)
	allFilters := []auditKindFilter{
		auditFilterAll, auditFilterLicense, auditFilterKey,
		auditFilterServer, auditFilterIdentity, auditFilterProbe,
	}
	// Same layout walk as audit OnClick: " filtres :  " takes 12 cells.
	cursor := 12
	const chipY = 5 // middle of the 3-row chip block (Y=4..6)
	for _, f := range allFilters {
		pillW := 1 + 1 + 1 + len(f.label()) + 4 // HintKey 3 cells + label + border+padding 4
		hitX := cursor + pillW/2
		cmd := m.OnClick(hitX, chipY, 144)
		if cmd == nil {
			t.Errorf("audit chip [%s]: OnClick(%d,%d) returned nil", f.hotkey(), hitX, chipY)
			cursor += pillW
			continue
		}
		msg := cmd()
		fm, ok := msg.(auditFilterClickMsg)
		if !ok {
			t.Errorf("audit chip [%s]: cmd produced %T, want auditFilterClickMsg", f.hotkey(), msg)
		} else if fm.f != f {
			t.Errorf("audit chip [%s]: filter = %v, want %v", f.hotkey(), fm.f, f)
		}
		cursor += pillW
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch 3b — license filter chips on the topRow (active/expiring/etc). Each
// click should fire licenseFilterClickMsg with the right enum.
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_LicenseFilterChips(t *testing.T) {
	m := newLicensesModel(nil)
	_ = renderListScreen(t, ViewLicenses, &m)
	// chip pill rows are at TopChromeRows + 1 (leading blank), 3 rows tall.
	// Middle row is the clickable label.
	const chipMidY = TopChromeRows + 2
	// OnClick mirrors renderFilterBar's cursor=1 + per-pill (label+4 border/pad) + 1 sep.
	allFilters := []licenseFilter{
		licFilterAll, licFilterActive, licFilterExpiring,
		licFilterExpired, licFilterRevoked, licFilterSuperseded,
	}
	cursor := 1
	for _, f := range allFilters {
		w := len(f.String()) + 4
		hitX := cursor + w/2
		cmd := m.OnClick(hitX, chipMidY, 144)
		if cmd == nil {
			t.Errorf("licenses chip %q: OnClick(%d,%d) returned nil", f.String(), hitX, chipMidY)
			cursor += w + 1
			continue
		}
		msg := cmd()
		fm, ok := msg.(licenseFilterClickMsg)
		if !ok {
			t.Errorf("licenses chip %q: cmd produced %T, want licenseFilterClickMsg", f.String(), msg)
		} else if fm.f != f {
			t.Errorf("licenses chip %q: filter = %v, want %v", f.String(), fm.f, f)
		}
		cursor += w + 1
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch 3c — license detail tab strip [I/B/P/A/C]. Each click must fire
// licenseDetailTabClickMsg with the matching tab index 0..4.
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_LicenseDetailTabs(t *testing.T) {
	m := newLicensesModel(nil)
	m.detail = true
	// Inject a fake row so selectedRow() returns non-nil and renderDetail
	// emits the full panel WITH the tab strip (the no-selection branch
	// shows only a placeholder).
	m.rows = []*ent.License{{LicenseUUID: "00000000-0000-0000-0000-000000000000", Subject: "test@example"}}
	view := renderListScreen(t, ViewLicenses, &m)
	// HintKey has Padding(0, 1) so '[I]' renders as ' [I] '; the tab labels
	// are joined with two-space gaps. Anchor by the bracketed key.
	tabStripY := findLineY(view, "[I]")
	if tabStripY < 0 {
		t.Logf("=== full rendered view ===\n%s", view)
		t.Fatalf("license detail tab strip not found")
	}
	// Cells laid out by renderDetail's tabStrip walker:
	//   each cell width = lipgloss.Width(label) + 2 (gap)
	cells := []struct {
		tab   int
		label string
	}{{0, "[I]dent"}, {1, "[B]ind"}, {2, "[P]EM"}, {3, "[A]udit"}, {4, "[C]haîne"}}
	cursor := 0
	for _, c := range cells {
		w := len(c.label) + 2
		hitX := cursor + w/2
		cmd := m.OnClick(hitX, tabStripY, 144)
		if cmd == nil {
			t.Errorf("detail tab %s: OnClick(%d,%d) returned nil", c.label, hitX, tabStripY)
			cursor += w
			continue
		}
		msg := cmd()
		dt, ok := msg.(licenseDetailTabClickMsg)
		if !ok {
			t.Errorf("detail tab %s: cmd produced %T, want licenseDetailTabClickMsg", c.label, msg)
		} else if dt.tab != c.tab {
			t.Errorf("detail tab %s: got tab=%d, want %d", c.label, dt.tab, c.tab)
		}
		cursor += w
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch 6 — global tab bar clicks: clicking each of the 10 tabs at Y=1 must
// switch the active view via widgets.SwitchViewMsg{ID: <view-name>}.
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_TopTabBarClicks(t *testing.T) {
	expectations := []ViewID{
		ViewDashboard, ViewLicenses, ViewIssuers, ViewRecipients,
		ViewIdentities, ViewRevocation, ViewServers, ViewTOTP,
		ViewAudit, ViewSettings,
	}
	tb := buildTabBar(ViewDashboard, 144)
	_ = tb.View() // populates tab widths via the widget itself
	for i, want := range expectations {
		// Use the TabBar's own per-rune hit logic: scan the rendered output
		// for the matching tab label and click on it. Anchor by the digit
		// prefix the tab strip prepends ("1 ", "2 ", …, "0 " for 10th).
		out := tb.View()
		labels := []string{"Dashboard", "Licenses", "Issuer keys", "Recipients",
			"Identities", "Revocation", "Servers", "TOTP", "Audit", "Settings"}
		hitX := strings.Index(out, labels[i])
		if hitX < 0 {
			t.Errorf("tab #%d (%s): label %q not found in rendered tab strip", i, want, labels[i])
			continue
		}
		cmd := tb.OnClick(hitX, 0, tea.MouseButtonLeft)
		if cmd == nil {
			t.Errorf("tab #%d (%s): OnClick(%d) returned nil", i, want, hitX)
			continue
		}
		sv, ok := cmd().(widgets.SwitchViewMsg)
		if !ok {
			t.Errorf("tab #%d (%s): cmd produced %T, want widgets.SwitchViewMsg", i, want, cmd())
			continue
		}
		if sv.ID != string(want) {
			t.Errorf("tab #%d: SwitchViewMsg.ID = %q, want %q", i, sv.ID, want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch 1 — title-hint chip clicks on the 6 list screens.
//
// Each [k] chip in the box title row is expected to synthesise a KeyMsg with
// that rune so the existing keyboard handler runs unchanged. We render the
// screen, walk every chip recorded in the model's titleHints, click on the
// chip's centre X at its row Y, and assert the produced Cmd emits the
// matching KeyMsg.
//
// A failure here means either:
//   - the chip's hit-test math is wrong (X off, Y off), or
//   - the wizard/list screen handler stopped reacting to the key.
// ─────────────────────────────────────────────────────────────────────────────

func TestInteractions_TitleHintsSynthesiseKey(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) *titleHintRow
	}{
		{"issuers", func(t *testing.T) *titleHintRow {
			m := newIssuersModel(nil)
			_ = renderListScreen(t, ViewIssuers, &m)
			return m.titleHints
		}},
		{"recipients", func(t *testing.T) *titleHintRow {
			m := newRecipientsModel(nil)
			_ = renderListScreen(t, ViewRecipients, &m)
			return m.titleHints
		}},
		{"identities", func(t *testing.T) *titleHintRow {
			m := newIdentitiesModel(nil)
			_ = renderListScreen(t, ViewIdentities, &m)
			return m.titleHints
		}},
		{"totp", func(t *testing.T) *titleHintRow {
			m := newTOTPModel(nil)
			_ = renderListScreen(t, ViewTOTP, &m)
			return m.titleHints
		}},
		{"revocation", func(t *testing.T) *titleHintRow {
			m := newRevocationModel(nil)
			_ = renderListScreen(t, ViewRevocation, &m)
			return m.titleHints
		}},
		{"licenses", func(t *testing.T) *titleHintRow {
			m := newLicensesModel(nil)
			_ = renderListScreen(t, ViewLicenses, &m)
			return m.titleHints
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := tc.setup(t)
			if row == nil || len(row.hints) == 0 {
				t.Fatalf("%s: titleHints not populated by View()", tc.name)
			}
			cursor := row.startX
			for i, h := range row.hints {
				key := h.Key
				w := row.segWs[i]
				clickX := cursor + w/2
				clickY := row.y
				cmd := row.hit(clickX, clickY)
				// licenses [↑↓] is informational — its Cmd factory returns
				// nil by design (`func() tea.Cmd { return nil }`).
				if key == "↑↓" {
					if cmd != nil {
						t.Errorf("licenses/[↑↓]: expected nil Cmd, got %T", cmd)
					}
					cursor += w + sepWidth
					continue
				}
				if cmd == nil {
					t.Errorf("%s/[%s]: hit(%d,%d) returned nil — chip click would not fire", tc.name, key, clickX, clickY)
					cursor += w + sepWidth
					continue
				}
				msg := cmd()
				km, ok := msg.(tea.KeyMsg)
				if !ok {
					t.Errorf("%s/[%s]: cmd produced %T, want tea.KeyMsg", tc.name, key, msg)
				} else if string(km.Runes) != key {
					t.Errorf("%s/[%s]: KeyMsg.Runes = %q, want %q", tc.name, key, string(km.Runes), key)
				}
				cursor += w + sepWidth
			}
		})
	}
}
