package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// rowsAllSameWidth reports any rendered row whose visible width differs from
// the first non-empty row. Used to confirm the table grid does not drift on
// long subjects / audience / features.
func rowsAllSameWidth(t *testing.T, label, dump string) {
	t.Helper()
	lines := strings.Split(dump, "\n")
	var ref int
	for i, ln := range lines {
		w := lipgloss.Width(ln)
		if w == 0 {
			continue
		}
		if ref == 0 {
			ref = w
			continue
		}
		// The header/title/intro rows can legitimately be shorter — we only
		// gate when a row crosses 1.5× a typical short line, which is the
		// pattern a drifted table column produces.
		if w > ref+2 {
			t.Errorf("%s: line %d width=%d exceeds ref width %d — column drift\n%q", label, i, w, ref, ln)
		}
	}
}

// TestStress_LicensesRenderAt100Rows dumps the full View() with 100 licences
// holding subjects + audiences + features of varying lengths, then asserts
// every rendered line stays within the box width budget. The pre-2026-05
// stress test only counted rows and the table height — it never inspected
// the rendered text for column drift.
func TestStress_LicensesRenderAt100Rows(t *testing.T) {
	rows := make([]*ent.License, 100)
	now := time.Now()
	for i := range rows {
		rows[i] = &ent.License{
			ID:          uuid.New(),
			LicenseUUID: uuid.NewString(),
			// Subjects of varying length to stress the SUBJECT column truncation.
			Subject:    strings.Repeat("s", 1+(i%23)),
			IssuerName: "issuer-prod-2026Q2",
			// Long audience list to verify the 14-cell cap kicks in.
			Audience: []string{
				"aud-one-" + strings.Repeat("x", i%10),
				"aud-two-long-name",
			},
			Features:  []string{"feat-A", "feat-B", "feat-C-long-name"},
			Status:    "active",
			NotBefore: now.Add(-24 * time.Hour),
			NotAfter:  now.Add(time.Duration(i) * 24 * time.Hour),
		}
	}

	m := newLicensesModel(nil)
	m.width = 144
	m.hgt = 44
	m.rows = rows
	m.rebuildTable()
	dump := m.View()

	// Box outer width must equal m.width (the box uses BoxedWidth(m.width)
	// internally so the bordered frame fits in the terminal exactly).
	maxW := 0
	for _, ln := range strings.Split(dump, "\n") {
		if w := lipgloss.Width(ln); w > maxW {
			maxW = w
		}
	}
	if maxW > m.width {
		t.Errorf("widest line = %d > terminal width %d — overflow risk", maxW, m.width)
	}

	// Header row must contain every column title in order.
	hdr := strings.Index(dump, "STATUS")
	if hdr < 0 {
		t.Fatal("STATUS header missing from rendered view")
	}
	for _, col := range []string{"SUBJECT", "AUDIENCE", "KEYID", "EXPIRES", "FEATURES"} {
		if !strings.Contains(dump[hdr:], col) {
			t.Errorf("column %q missing from header row", col)
		}
	}

	// Spot-check: long subjects must be truncated (no "ssssssssssssssssssssssss"
	// substrings of length > column width).
	if strings.Contains(dump, strings.Repeat("s", 23)) {
		t.Error("untruncated subject (23 chars) present — SUBJECT column width=22 should clip")
	}

	rowsAllSameWidth(t, "licenses N=100", dump)
}

// TestLive_LicenseExportKey asserts [E] on a selected licence pushes an
// input overlay tagged OverlayIDLicenseExport (the route app.go uses to find
// handleLicenseInputResult).
func TestLive_LicenseExportKey(t *testing.T) {
	m := newLicensesModel(nil)
	m.rows = []*ent.License{{ID: uuid.New(), LicenseUUID: "u1", Subject: "s", Status: "active",
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}}
	m.width, m.hgt = 144, 44
	m.rebuildTable()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if cmd == nil {
		t.Fatal("[E] produced nil cmd")
	}
	msg := cmd()
	push, ok := msg.(pushOverlayMsg)
	if !ok {
		t.Fatalf("[E] produced %T, want pushOverlayMsg", msg)
	}
	in, ok := push.overlay.(*inputOverlay)
	if !ok {
		t.Fatalf("[E] overlay = %T, want *inputOverlay", push.overlay)
	}
	if in.id != OverlayIDLicenseExport {
		t.Errorf("overlay id = %q, want %q", in.id, OverlayIDLicenseExport)
	}
}

// TestStress_LicensesDumpFirst20RowsAt100 prints the first 22 lines of the
// rendered view to t.Logf so an operator can confirm visually that columns
// align. Visible only with `go test -v`; never asserts. The dump captures
// what the screen actually looks like at N=100.
func TestStress_LicensesDumpFirst20RowsAt100(t *testing.T) {
	rows := make([]*ent.License, 100)
	now := time.Now()
	for i := range rows {
		rows[i] = &ent.License{
			ID:          uuid.New(),
			LicenseUUID: uuid.NewString(),
			Subject:     "user-" + strings.Repeat("x", 1+(i%15)),
			IssuerName:  "issuer-prod-2026Q2",
			Audience:    []string{"aud-" + strings.Repeat("a", i%6)},
			Features:    []string{"f1", "f2"},
			Status:      "active",
			NotBefore:   now.Add(-24 * time.Hour),
			NotAfter:    now.Add(time.Duration(i) * 24 * time.Hour),
		}
	}
	m := newLicensesModel(nil)
	m.width = 144
	m.hgt = 44
	m.rows = rows
	m.rebuildTable()
	for i, ln := range strings.Split(m.View(), "\n") {
		if i > 22 {
			break
		}
		t.Logf("L%02d|%s|", i, ln)
	}
}

// TestStretchColumns_LicensesGrowsWithWidth confirms that SUBJECT and other
// weighted columns actually widen when the terminal grows — the regression
// guard for stretchColumns (vs. legacy stretchLastColumn which only widened
// the rightmost column).
func TestStretchColumns_LicensesGrowsWithWidth(t *testing.T) {
	m := newLicensesModel(nil)
	m.rows = []*ent.License{{
		ID: uuid.New(), LicenseUUID: uuid.NewString(),
		Subject: strings.Repeat("s", 80), IssuerName: "issuer-x",
		Audience: []string{"aud"}, Status: "active",
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour),
	}}

	// Snapshot widths into local arrays — Columns() returns the live slice
	// which the next rebuildTable will mutate in place.
	snap := func() []int {
		cs := m.table.Columns()
		ws := make([]int, len(cs))
		for i, c := range cs {
			ws[i] = c.Width
		}
		return ws
	}

	// 140 is above the minSum (99) + overhead (14) = 113, so we're in the
	// grow phase and weight-0 columns (EXPIRES) stay at their declared min.
	m.width, m.hgt = 140, 44
	m.rebuildTable()
	narrow := snap()

	m.width, m.hgt = 220, 44
	m.rebuildTable()
	wide := snap()
	t.Logf("narrow=%v wide=%v", narrow, wide)

	// SUBJECT (weight 3) must grow strictly.
	if wide[1] <= narrow[1] {
		t.Errorf("SUBJECT did not grow: narrow=%d wide=%d", narrow[1], wide[1])
	}
	// AUDIENCE (weight 2) must grow.
	if wide[2] <= narrow[2] {
		t.Errorf("AUDIENCE did not grow: narrow=%d wide=%d", narrow[2], wide[2])
	}
	// EXPIRES (weight 0) must stay equal.
	if wide[4] != narrow[4] {
		t.Errorf("EXPIRES drifted: narrow=%d wide=%d (expected fixed)", narrow[4], wide[4])
	}
}

// TestRevocation_DetailPanelRendersFields exercises the new R1 detail panel:
// asserts the panel appears when detail=true, that subject + uuid + revoked-
// at + revoked-by + reason are shown without truncation, and that the
// lazy-loaded licence context renders audience/features once detailLic is set.
func TestRevocation_DetailPanelRenders(t *testing.T) {
	m := newRevocationModel(nil)
	m.width, m.hgt = 200, 44
	licID := uuid.New()
	m.rows = []service.RevocationView{{
		LicenseID:   licID,
		LicenseUUID: "feedfacedeadbeef-cafe-babe-0000-000000000001",
		Subject:     "alice@example.test",
		KeyID:       "key-prod-2026Q2",
		Reason:      "key compromise — operator action 2026-05-27",
		RevokedAt:   time.Date(2026, 5, 27, 14, 32, 0, 0, time.UTC),
		RevokedBy:   "operator-jdoe",
	}}
	// Lazy-loaded licence behind the revocation.
	m.detailLic = &ent.License{
		ID:       licID,
		Audience: []string{"prod", "us-east-1"},
		Features: []string{"audit", "totp", "remote-revoke"},
		NotBefore: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:  time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	m.rebuildTable()
	out := m.View()

	for _, want := range []string{
		"alice@example.test",                            // full subject
		"feedfacedeadbeef-cafe-babe-0000-000000000001",  // full uuid
		"key-prod-2026Q2",                               // keyID
		"operator-jdoe",                                 // revokedBy (was missing before)
		"key compromise — operator action 2026-05-27",   // full reason
		"prod, us-east-1",                               // audience from lazy-loaded licence
		"audit, totp, remote-revoke",                    // features
	} {
		if !strings.Contains(out, want) {
			t.Errorf("detail panel missing %q in rendered view", want)
		}
	}
}

// TestStretchColumns_ShrinkBelowMins is the regression guard for the
// "fenêtre rétrécit et les dernières colonnes passent à la ligne" defect.
// When the terminal is narrower than the sum of declared minimum widths,
// stretchColumns must shrink every column proportionally so the row sum
// fits exactly inside (width - overhead). Otherwise bubbles/table wraps the
// trailing cells onto the next line.
func TestStretchColumns_ShrinkBelowMins(t *testing.T) {
	m := newLicensesModel(nil)
	m.rows = []*ent.License{{
		ID: uuid.New(), LicenseUUID: uuid.NewString(),
		Subject: "x", IssuerName: "iss", Status: "active",
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour),
	}}

	// Constructor mins for licenses: 11+22+16+18+12+20 = 99, overhead = 14
	// → minimum natural width is 113. Render at 80 to force the shrink path.
	m.width, m.hgt = 80, 44
	m.rebuildTable()

	overhead := 2*len(m.table.Columns()) + 2
	target := BoxedInner(m.width) - overhead

	sum := 0
	for _, c := range m.table.Columns() {
		if c.Width < 1 {
			t.Errorf("col %q ended up at width %d (< 1)", c.Title, c.Width)
		}
		sum += c.Width
	}
	if sum > target {
		t.Errorf("columns sum=%d exceeds target=%d (BoxedInner=%d) — overflow will wrap",
			sum, target, BoxedInner(m.width))
	}
}

// TestStretchColumns_FullCycle drives grow → shrink → re-grow on the same
// table.Model to confirm the algorithm is idempotent across history. The
// captured tableMins must NOT drift on subsequent calls.
func TestStretchColumns_FullCycle(t *testing.T) {
	m := newLicensesModel(nil)
	m.rows = []*ent.License{{
		ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "x",
		IssuerName: "iss", Status: "active",
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour),
	}}
	snap := func() []int {
		cs := m.table.Columns()
		ws := make([]int, len(cs))
		for i, c := range cs {
			ws[i] = c.Width
		}
		return ws
	}
	m.width, m.hgt = 160, 44
	m.rebuildTable()
	atRef := snap()

	for _, w := range []int{80, 220, 60, 200, 160} {
		m.width = w
		m.rebuildTable()
		overhead := 2*len(m.table.Columns()) + 2
		target := BoxedInner(w) - overhead
		sum := 0
		for _, c := range m.table.Columns() {
			sum += c.Width
		}
		if sum > target {
			t.Errorf("width=%d: sum=%d > target=%d (would overflow)", w, sum, target)
		}
	}
	// Final restoration: same width must yield same widths regardless of path.
	m.width = 160
	m.rebuildTable()
	atRet := snap()
	for i := range atRef {
		if atRef[i] != atRet[i] {
			t.Errorf("col %d drift after cycle: ref=%d ret=%d", i, atRef[i], atRet[i])
		}
	}
}

// TestStretchColumns_ShrinkBackToMins is the regression guard for the
// "widen then shrink" defect: after stretching to a wide width, going back
// to a narrow one must restore every column to its captured minimum so the
// row sum no longer overflows the new terminal width. Before the fix the
// columns kept their stretched widths and lines bled past the right border.
func TestStretchColumns_ShrinkBackToMins(t *testing.T) {
	m := newLicensesModel(nil)
	m.rows = []*ent.License{{
		ID: uuid.New(), LicenseUUID: uuid.NewString(),
		Subject: "x", IssuerName: "iss",
		Status: "active", NotBefore: time.Now(),
		NotAfter: time.Now().Add(time.Hour),
	}}

	snap := func() []int {
		cs := m.table.Columns()
		ws := make([]int, len(cs))
		for i, c := range cs {
			ws[i] = c.Width
		}
		return ws
	}

	// Capture baseline (no stretch — width too small) by rebuilding at 100.
	m.width, m.hgt = 100, 44
	m.rebuildTable()
	baseline := snap()

	// Stretch wide.
	m.width = 220
	m.rebuildTable()
	wide := snap()

	// Shrink back to the original narrow width — every column must equal
	// its baseline value again.
	m.width = 100
	m.rebuildTable()
	shrunk := snap()

	for i := range baseline {
		if shrunk[i] != baseline[i] {
			t.Errorf("col %d: shrunk=%d baseline=%d (wide=%d) — did not snap back to minimum",
				i, shrunk[i], baseline[i], wide[i])
		}
	}
}

// TestIssuers_ExportPrivKey_FullFlow walks the two-step [K] export flow:
//  1. [K] pushes a confirm overlay tagged OverlayIDIssuerExportPriv
//  2. Confirming it routes via app.go and pushes the path-input overlay
//     tagged OverlayIDIssuerExportPrivPath
//
// The pre-2026-05 bug left step 2 unimplemented — the confirm disappeared
// into a no-op and nothing ever asked for a path. We assert both halves.
func TestIssuers_ExportPrivKey_FullFlow(t *testing.T) {
	m := newIssuersModel(nil)
	m.rows = []*ent.Issuer{{ID: uuid.New(), Name: "prod", KeyID: "k1", Active: true, CreatedAt: time.Now()}}
	m.width, m.hgt = 144, 44
	m.rebuildTable()

	// Step 1 — [K] pushes confirm.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	if cmd == nil {
		t.Fatal("[K] produced nil cmd")
	}
	push, ok := cmd().(pushOverlayMsg)
	if !ok {
		t.Fatalf("[K] msg = %T, want pushOverlayMsg", cmd())
	}
	conf, ok := push.overlay.(*confirmOverlay)
	if !ok {
		t.Fatalf("[K] overlay = %T, want *confirmOverlay", push.overlay)
	}
	if conf.id != OverlayIDIssuerExportPriv {
		t.Errorf("confirm.id = %q, want %q", conf.id, OverlayIDIssuerExportPriv)
	}

	// Step 2 — simulate a Yes-confirm via the root dispatcher.
	r := New(nil, nil, SessionReady)
	r.active = ViewIssuers
	r.issuers = m
	updated := r.dispatchOverlayResult(ConfirmResultMsg{ID: OverlayIDIssuerExportPriv, Confirm: true})
	if updated.pendingCmd == nil {
		t.Fatal("ConfirmResultMsg(true) did not produce a pendingCmd — handler missing")
	}
	push2, ok := updated.pendingCmd().(pushOverlayMsg)
	if !ok {
		t.Fatalf("confirm follow-up msg = %T, want pushOverlayMsg", updated.pendingCmd())
	}
	in, ok := push2.overlay.(*inputOverlay)
	if !ok {
		t.Fatalf("step2 overlay = %T, want *inputOverlay", push2.overlay)
	}
	if in.id != OverlayIDIssuerExportPrivPath {
		t.Errorf("input.id = %q, want %q", in.id, OverlayIDIssuerExportPrivPath)
	}
}

// TestLicensesPEM_TabNavWhileViewing is the regression for the user complaint
// "le scroll de la clef PEM signé ne fonctionne pas et est peut etre en conflit
// avec la navigation du tableau". The new contract:
//   - ↑/↓ on the PEM tab navigates the licences table (cursor moves)
//   - the PEM viewport content auto-reloads from the new selection
//   - PgUp/PgDn scrolls the viewport content (no longer hijacks ↑/↓)
//   - clicking the [P] tab via licenseDetailTabClickMsg loads content too
func TestLicensesPEM_TabNavWhileViewing(t *testing.T) {
	row1 := &ent.License{ID: uuid.New(), LicenseUUID: uuid.NewString(),
		Subject: "alice", Pem: []byte("PEM-ALICE-1234"),
		Status: "active", NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
	row2 := &ent.License{ID: uuid.New(), LicenseUUID: uuid.NewString(),
		Subject: "bob", Pem: []byte("PEM-BOB-9876"),
		Status: "active", NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}

	m := newLicensesModel(nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{row1, row2}})

	// Open PEM tab via [P] — should load row1's PEM.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if m.detailTab != 2 {
		t.Fatalf("[P]: detailTab=%d want 2", m.detailTab)
	}
	if m.pemViewport.TotalLineCount() == 0 {
		t.Fatal("[P] did not load PEM content into viewport")
	}

	// On the PEM tab ↑/↓ navigate the table (the PEM viewport auto-reloads
	// for the new row — dual affordance documented in the keytable truth
	// table). j/k/pgup/pgdn/space/b/g/G scroll the viewport instead.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.table.Cursor() != 1 {
		t.Errorf("↓ on PEM tab: cursor=%d want 1 (table nav must still work)", m.table.Cursor())
	}
	cursorBefore := m.table.Cursor()
	out := m.renderDetailPEM(row2)
	if !strings.Contains(out, "PEM signé") {
		t.Errorf("renderDetailPEM did not produce expected header in:\n%s", out)
	}

	// PgDn should also scroll the viewport (not move the table cursor).
	yOffsetBefore := m.pemViewport.YOffset
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.table.Cursor() != cursorBefore {
		t.Errorf("PgDn moved table cursor (%d→%d) — should only scroll viewport",
			cursorBefore, m.table.Cursor())
	}
	// Content fits in viewport height (a 13-byte PEM never needs scroll) so
	// HalfViewDown is a no-op here. Just assert the call didn't crash and
	// table state is intact — the scroll mechanics themselves are exercised
	// by viewport's own tests in bubbles.
	_ = yOffsetBefore
}

// TestLicensesKeys_TableNavRegression isolates the user complaint "la
// navigation dans le tableau des licenses ne fonctionne plus". Drives ↑/↓
// in every meaningful screen context and asserts the table cursor moves
// when expected (and stays put when not). Used together with
// TestLicensesKeyDispatchTruthTable as the authoritative ground truth for
// what each key should do on the licenses screen.
func TestLicensesKeys_TableNavRegression(t *testing.T) {
	rows := []*ent.License{
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "alice",
			Pem: []byte("p1"), Status: "active", NotBefore: time.Now(),
			NotAfter: time.Now().Add(time.Hour)},
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "bob",
			Pem: []byte("p2"), Status: "active", NotBefore: time.Now(),
			NotAfter: time.Now().Add(time.Hour)},
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "carol",
			Pem: []byte("p3"), Status: "active", NotBefore: time.Now(),
			NotAfter: time.Now().Add(time.Hour)},
	}

	scenarios := []struct {
		name      string
		setup     func(m tea.Model) tea.Model
		key       tea.KeyMsg
		wantMove  bool // true = cursor MUST move; false = MUST stay
	}{
		{"detail closed + ↓", func(m tea.Model) tea.Model {
			r := rootOf(t, m)
			r.licenses.detail = false
			r.licenses.rebuildTable()
			return r
		}, tea.KeyMsg{Type: tea.KeyDown}, true},
		{"detail open Identity + ↓", func(m tea.Model) tea.Model {
			return drive(m, 'I')
		}, tea.KeyMsg{Type: tea.KeyDown}, true},
		{"detail open Bindings + ↓", func(m tea.Model) tea.Model {
			return drive(m, 'B')
		}, tea.KeyMsg{Type: tea.KeyDown}, true},
		{"detail open Chain + ↓", func(m tea.Model) tea.Model {
			return drive(m, 'C')
		}, tea.KeyMsg{Type: tea.KeyDown}, true},
		{"detail open Audit + ↓", func(m tea.Model) tea.Model {
			return drive(m, 'A')
		}, tea.KeyMsg{Type: tea.KeyDown}, true},
		// On PEM tab ↑/↓ STILL navigate the table (PEM auto-reloads to track
		// the new selection). j/k/space/b/pgup/pgdn/g/G scroll the viewport.
		// Truth table: screen_licenses_keytable_test.go.
		{"detail open PEM + ↓ navigates (PEM follows)", func(m tea.Model) tea.Model {
			return drive(m, 'P')
		}, tea.KeyMsg{Type: tea.KeyDown}, true},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
			m = drive(m, '2')
			m, _ = m.Update(LicensesLoadedMsg{Rows: rows})
			m = sc.setup(m)

			before := rootOf(t, m).licenses.table.Cursor()
			mm, _ := m.Update(sc.key)
			after := rootOf(t, mm).licenses.table.Cursor()

			if sc.wantMove && after == before {
				t.Errorf("table cursor did NOT move (stayed at %d) — key dispatch is wrong", before)
			}
			if !sc.wantMove && after != before {
				t.Errorf("table cursor moved (%d→%d) but should have stayed put — viewport scroll bled through", before, after)
			}
		})
	}
}

// TestLicensesPEM_ScrollViaRootModel drives the exact user flow through the
// rootModel: navigate to Licenses with '2', load rows, press 'P' to open the
// PEM tab, then send 'j'/'k' through rootModel.Update — which mirrors what
// bubbletea's runtime does for every keypress. If this test passes but the
// operator still can't scroll, the issue is somewhere in the terminal layer
// or in how the user observes the change.
func TestLicensesPEM_ScrollViaRootModel(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("-----BEGIN MALDEV LICENSE v2-----\n")
	for i := 0; i < 200; i++ {
		sb.WriteString(strings.Repeat("A", 64))
		sb.WriteString("\n")
	}
	sb.WriteString("-----END MALDEV LICENSE v2-----\n")
	pem := sb.String()
	row := &ent.License{ID: uuid.New(), LicenseUUID: uuid.NewString(),
		Subject: "alice", Pem: []byte(pem), Status: "active",
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}

	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = drive(m, '2') // ViewLicenses
	m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{row}})
	m = drive(m, 'P') // PEM tab

	rm := rootOf(t, m)
	if rm.licenses.detailTab != 2 {
		t.Fatalf("[P]: detailTab=%d (rootModel route), want 2", rm.licenses.detailTab)
	}
	if rm.licenses.pemViewport.TotalLineCount() < 100 {
		t.Fatalf("PEM not loaded via root path: total=%d", rm.licenses.pemViewport.TotalLineCount())
	}
	y0 := rm.licenses.pemViewport.YOffset

	// 'j' through rootModel.Update — must end up scrolling the viewport.
	m = drive(m, 'j')
	rm = rootOf(t, m)
	if rm.licenses.pemViewport.YOffset <= y0 {
		t.Errorf("'j' via rootModel did NOT scroll: YOffset %d→%d", y0, rm.licenses.pemViewport.YOffset)
	}

	// 'space' is a separator-free key — verify it routes too.
	m = drive(m, ' ')
	rm = rootOf(t, m)
	if rm.licenses.pemViewport.YOffset <= y0 {
		t.Errorf("space via rootModel did NOT scroll further: still at YOffset=%d", rm.licenses.pemViewport.YOffset)
	}

	// Critical: the rendered View() must actually CHANGE between before-scroll
	// and after-scroll. If YOffset moves but the lipgloss output is identical,
	// the operator perceives "no scroll" even though internal state changed —
	// e.g. when renderDetailPEM uses a value-receiver copy of the viewport.
	var mBefore tea.Model = New(nil, nil, SessionReady)
	mBefore, _ = mBefore.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	mBefore = drive(mBefore, '2')
	mBefore, _ = mBefore.Update(LicensesLoadedMsg{Rows: []*ent.License{row}})
	mBefore = drive(mBefore, 'P')
	viewBefore := mBefore.View()

	mAfter := mBefore
	for i := 0; i < 5; i++ {
		mAfter = drive(mAfter, 'j')
	}
	viewAfter := mAfter.View()

	if viewBefore == viewAfter {
		t.Errorf("after 5× 'j' the rendered View() is IDENTICAL — the viewport scroll is not reflected in the screen output")
	}
}

// TestLicensesPEM_PgDownActuallyScrolls drives a long PEM through the viewport
// and asserts PgDn moves YOffset. The earlier TestLicensesPEM_TabNavWhileViewing
// only checked "didn't crash" because the test PEM was short enough to fit
// in the viewport, where HalfViewDown is a no-op.
func TestLicensesPEM_PgDownActuallyScrolls(t *testing.T) {
	// 200 lines × 64 chars = way more than the viewport's ~17 line height.
	var sb strings.Builder
	sb.WriteString("-----BEGIN MALDEV LICENSE v2-----\n")
	for i := 0; i < 200; i++ {
		sb.WriteString(strings.Repeat("A", 64))
		sb.WriteString("\n")
	}
	sb.WriteString("-----END MALDEV LICENSE v2-----\n")
	pem := sb.String()

	row := &ent.License{ID: uuid.New(), LicenseUUID: uuid.NewString(),
		Subject: "alice", Pem: []byte(pem), Status: "active",
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}

	m := newLicensesModel(nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{row}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})

	if m.detailTab != 2 {
		t.Fatalf("[P]: detailTab=%d want 2", m.detailTab)
	}
	if m.pemViewport.TotalLineCount() < 100 {
		t.Fatalf("PEM not loaded as expected: TotalLineCount=%d", m.pemViewport.TotalLineCount())
	}
	if m.pemViewport.Height <= 0 {
		t.Fatalf("viewport height=%d after WindowSizeMsg — should be > 0", m.pemViewport.Height)
	}

	y0 := m.pemViewport.YOffset
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	y1 := m.pemViewport.YOffset
	if y1 <= y0 {
		t.Errorf("PgDn did not scroll: YOffset %d→%d (height=%d, total=%d)",
			y0, y1, m.pemViewport.Height, m.pemViewport.TotalLineCount())
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	y2 := m.pemViewport.YOffset
	if y2 >= y1 {
		t.Errorf("PgUp did not scroll back: YOffset %d→%d", y1, y2)
	}
}

// TestNoVerticalOverflow is the cross-screen audit asked by the user
// ("contrôle dans toute la TUI que rien ne déborde en bas"). For every list
// screen, drive it at a realistic terminal size with detail mode open and
// confirm the rendered View() height never exceeds the terminal height.
// Without the layout fix that follows, the licences PEM tab in particular
// over-shoots by ~7 lines on a 44-line terminal — the detail box bottom
// border and viewport tail render below the visible area, which is exactly
// why operators perceive "scroll doesn't work".
func TestNoVerticalOverflow(t *testing.T) {
	const W, H = 144, 44

	tests := []struct {
		name string
		view func() string
	}{
		{
			"licenses-detail-pem",
			func() string {
				row := &ent.License{ID: uuid.New(), LicenseUUID: uuid.NewString(),
					Subject: "alice", Pem: []byte(strings.Repeat("A", 4096)),
					Status: "active", NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
				m := newLicensesModel(nil)
				m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
				m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{row}})
				m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
				return m.View()
			},
		},
		{
			"licenses-detail-identity",
			func() string {
				row := &ent.License{ID: uuid.New(), LicenseUUID: uuid.NewString(),
					Subject: "alice", Status: "active",
					NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
				m := newLicensesModel(nil)
				m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
				m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{row}})
				return m.View()
			},
		},
		{
			"issuers-detail",
			func() string {
				m := newIssuersModel(nil)
				m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
				m, _ = m.Update(IssuersLoadedMsg{Rows: []*ent.Issuer{
					{ID: uuid.New(), Name: "prod", KeyID: "k1", Active: true, CreatedAt: time.Now()},
				}})
				return m.View()
			},
		},
		{
			"revocation-detail",
			func() string {
				licID := uuid.New()
				m := newRevocationModel(nil)
				m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
				m, _ = m.Update(RevocationLoadedMsg{Rows: []service.RevocationView{{
					LicenseID: licID, LicenseUUID: "uuid-1", Subject: "alice", KeyID: "k1",
					Reason: "compromise", RevokedAt: time.Now(), RevokedBy: "op",
				}}})
				return m.View()
			},
		},
		{
			"recipients-detail",
			func() string {
				m := newRecipientsModel(nil)
				m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
				m, _ = m.Update(RecipientsLoadedMsg{Rows: []*ent.RecipientKey{
					{ID: uuid.New(), Name: "alice", PublicKey: []byte("k"),
						CreatedAt: time.Now()},
				}})
				return m.View()
			},
		},
		{
			"identities-detail",
			func() string {
				m := newIdentitiesModel(nil)
				m, _ = m.Update(tea.WindowSizeMsg{Width: W, Height: H})
				m, _ = m.Update(IdentitiesLoadedMsg{Rows: []*ent.Identity{
					{ID: uuid.New(), Name: "id-1", Sha256: strings.Repeat("a", 64),
						CreatedAt: time.Now()},
				}})
				return m.View()
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			out := c.view()
			h := lipgloss.Height(out)
			// The status bar lives one line below the screen content (chrome
			// reserves a final row). Anything strictly over H means the
			// detail/table extends past the bottom of the terminal.
			if h > H {
				t.Errorf("rendered height = %d > terminal H = %d — overflow:\n--- last 20 lines ---\n%s",
					h, H, lastLines(out, 20))
			}
		})
	}
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// TestLicensesPEM_DumpFullStack renders the licences view (chrome wrapped)
// at 44x144 with the PEM tab open and dumps the entire frame so the operator
// can confirm visually that the detail box closes within the screen height
// and that scrolling moves visible content (not hidden content).
func TestLicensesPEM_DumpFullStack(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("-----BEGIN MALDEV LICENSE v2-----\n")
	for i := 0; i < 60; i++ {
		sb.WriteString(strings.Repeat("A", 64))
		sb.WriteString("\n")
	}
	sb.WriteString("-----END MALDEV LICENSE v2-----\n")
	pem := sb.String()
	row := &ent.License{ID: uuid.New(), LicenseUUID: uuid.NewString(),
		Subject: "alice", Pem: []byte(pem), Status: "active",
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}

	root := New(nil, nil, SessionReady)
	var m tea.Model = root
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = drive(m, '2')                                                // ViewLicenses
	m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{row}})
	m = drive(m, 'P')                                                // open PEM tab

	for i, ln := range strings.Split(m.View(), "\n") {
		t.Logf("S%02d|%s|", i, ln)
	}
}

func drive(m tea.Model, r rune) tea.Model {
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return mm
}

// TestNoOverflow_FullMatrix exhaustively audits the entire TUI for vertical
// overflow by driving rootModel.View() (the path the operator actually sees)
// across a matrix of terminal sizes × every list-screen detail tab.
// Pre-fix: detail boxes were clamped off the bottom in many combinations.
// The assertion is "rendered height ≤ H and bottom row of content is not a
// stray box border / empty pad" so the operator never sees a "cut off" frame.
func TestNoOverflow_FullMatrix(t *testing.T) {
	type screenCase struct {
		name  string
		setup func(m tea.Model) tea.Model
	}

	licRow := &ent.License{
		ID: uuid.New(), LicenseUUID: uuid.NewString(),
		Subject: "alice@example.test", Pem: []byte(strings.Repeat("A", 4096)),
		IssuerName: "iss", Audience: []string{"prod"}, Features: []string{"audit"},
		Status: "active", NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour),
		BindingsMeta:   map[string]any{"machine": "fp-deadbeef"},
		IdentitySha256: strings.Repeat("a", 64),
		BinarySha256:   strings.Repeat("b", 64),
		PayloadKind:    "sealed",
	}
	issRow := &ent.Issuer{ID: uuid.New(), Name: "prod", KeyID: "k1", Active: true, CreatedAt: time.Now()}
	recRow := &ent.RecipientKey{ID: uuid.New(), Name: "alice", PublicKey: []byte("k"), CreatedAt: time.Now()}
	idRow := &ent.Identity{ID: uuid.New(), Name: "id-1", Sha256: strings.Repeat("a", 64), CreatedAt: time.Now()}
	revLicID := uuid.New()
	revRow := service.RevocationView{LicenseID: revLicID, LicenseUUID: "uuid-1",
		Subject: "alice", KeyID: "k1", Reason: "compromise",
		RevokedAt: time.Now(), RevokedBy: "op"}

	screens := []screenCase{
		{"licenses (no tab open)", func(m tea.Model) tea.Model {
			m = drive(m, '2')
			m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{licRow}})
			return m
		}},
		{"licenses [I]dent tab", func(m tea.Model) tea.Model {
			m = drive(m, '2')
			m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{licRow}})
			return drive(m, 'I')
		}},
		{"licenses [B]ind tab", func(m tea.Model) tea.Model {
			m = drive(m, '2')
			m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{licRow}})
			return drive(m, 'B')
		}},
		{"licenses [P]EM tab", func(m tea.Model) tea.Model {
			m = drive(m, '2')
			m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{licRow}})
			return drive(m, 'P')
		}},
		{"issuers detail", func(m tea.Model) tea.Model {
			m = drive(m, '3')
			m, _ = m.Update(IssuersLoadedMsg{Rows: []*ent.Issuer{issRow}})
			return m
		}},
		{"recipients detail", func(m tea.Model) tea.Model {
			m = drive(m, '4')
			m, _ = m.Update(RecipientsLoadedMsg{Rows: []*ent.RecipientKey{recRow}})
			return m
		}},
		{"identities detail", func(m tea.Model) tea.Model {
			m = drive(m, '5')
			m, _ = m.Update(IdentitiesLoadedMsg{Rows: []*ent.Identity{idRow}})
			return m
		}},
		{"revocation detail", func(m tea.Model) tea.Model {
			m = drive(m, '6')
			m, _ = m.Update(RevocationLoadedMsg{Rows: []service.RevocationView{revRow}})
			return m
		}},
	}

	sizes := []struct{ w, h int }{
		{80, 24}, {100, 30}, {144, 44}, {200, 60}, {120, 35},
	}

	for _, sc := range screens {
		for _, sz := range sizes {
			t.Run(fmt.Sprintf("%s@%dx%d", sc.name, sz.w, sz.h), func(t *testing.T) {
				var m tea.Model = New(nil, nil, SessionReady)
				m, _ = m.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
				m = sc.setup(m)
				out := m.View()
				lines := strings.Split(out, "\n")
				if len(lines) > sz.h {
					t.Errorf("rendered %d lines > terminal H=%d (overflow by %d)",
						len(lines), sz.h, len(lines)-sz.h)
				}
				// Every box that opens (┌) must close (└) before the status
				// bar. Count occurrences in the visible region. Mismatch means
				// a box border was clipped off the bottom of the terminal —
				// exactly what the operator perceives as "the bottom of the
				// detail box is cut off".
				opens := strings.Count(out, "┌")
				closes := strings.Count(out, "└")
				if opens != closes {
					t.Errorf("box ┌ != ┘ (opens=%d closes=%d) — a frame is unclosed:\nFULL OUTPUT (%d lines):\n%s",
						opens, closes, len(lines), out)
				}
			})
		}
	}
}

// TestChrome_BreadcrumbDoesNotWrap guards that a long breadcrumb (which can
// happen when an active screen contributes a CrumbExtra such as a selected
// row's subject) is rendered on EXACTLY 1 line, not 2 via lipgloss soft-wrap.
// A wrapped breadcrumb eats an extra row, shifts everything down by 1 and
// makes the detail box's bottom border disappear off the screen.
func TestChrome_BreadcrumbDoesNotWrap(t *testing.T) {
	for _, w := range []int{80, 100, 120, 144} {
		t.Run(fmt.Sprintf("w=%d", w), func(t *testing.T) {
			out := renderBreadcrumb(ViewLicenses, licFilterActive,
				[]string{"liste (123)", "alice.long.user@example.com.research.tenant"},
				w)
			h := lipgloss.Height(out)
			if h != 1 {
				t.Errorf("breadcrumb at width=%d wrapped to %d lines:\n%s", w, h, out)
			}
		})
	}
}

// TestLicensesPEM_AllScrollBindings drives every advertised scroll key over a
// long PEM and asserts each one actually moves the viewport. Pre-fix only
// PgUp/PgDn were bound; on Windows Terminal those are often intercepted for
// the buffer scrollback, so the operator perceived "no scroll". j/k + space/b
// + g/G are TUI-standard and immune to terminal capture.
func TestLicensesPEM_AllScrollBindings(t *testing.T) {
	// 200 lines of body — easily exceeds the viewport's height budget.
	var sb strings.Builder
	sb.WriteString("-----BEGIN MALDEV LICENSE v2-----\n")
	for i := 0; i < 200; i++ {
		sb.WriteString(strings.Repeat("A", 64))
		sb.WriteString("\n")
	}
	sb.WriteString("-----END MALDEV LICENSE v2-----\n")
	pem := sb.String()

	mk := func() licensesModel {
		row := &ent.License{ID: uuid.New(), LicenseUUID: uuid.NewString(),
			Subject: "alice", Pem: []byte(pem), Status: "active",
			NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
		m := newLicensesModel(nil)
		m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
		m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{row}})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
		return m
	}

	type keyCase struct {
		name  string
		press func() tea.KeyMsg
		// wantUp  true  → expects YOffset to DECREASE (page up / line up / g)
		// wantUp  false → expects YOffset to INCREASE (page down / line down / G)
		wantUp bool
		// seed: if non-zero, scroll the viewport down by this many lines first
		// so an "up" key has somewhere to scroll back to.
		seed int
	}

	cases := []keyCase{
		// Down direction first — viewport starts at top.
		{"space", func() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}} }, false, 0},
		{"j", func() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}} }, false, 0},
		{"pgdown", func() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyPgDown} }, false, 0},
		{"G (end)", func() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}} }, false, 0},
		// Up direction — needs seeding.
		{"b", func() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}} }, true, 30},
		{"k", func() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}} }, true, 30},
		{"pgup", func() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyPgUp} }, true, 30},
		{"g (home)", func() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}} }, true, 30},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := mk()
			// Seed the viewport position if the case wants an "up" delta.
			for i := 0; i < c.seed; i++ {
				m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
			}
			before := m.pemViewport.YOffset
			m, _ = m.Update(c.press())
			after := m.pemViewport.YOffset
			if c.wantUp && after >= before {
				t.Errorf("%s: YOffset did not decrease (%d→%d)", c.name, before, after)
			}
			if !c.wantUp && after <= before {
				t.Errorf("%s: YOffset did not increase (%d→%d)", c.name, before, after)
			}
		})
	}
}

// TestLicensesPEM_ClickTabLoadsContent guards that the licenseDetailTabClickMsg
// case 2 path now loads PEM content. Pre-fix the click would set detailTab=2
// without loading; only the [P] keyboard shortcut populated the viewport.
func TestLicensesPEM_ClickTabLoadsContent(t *testing.T) {
	row := &ent.License{ID: uuid.New(), LicenseUUID: uuid.NewString(),
		Subject: "alice", Pem: []byte("PEM-CONTENT-XYZ"),
		Status: "active", NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
	m := newLicensesModel(nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m, _ = m.Update(LicensesLoadedMsg{Rows: []*ent.License{row}})

	if m.pemViewport.TotalLineCount() != 0 {
		t.Fatal("viewport content unexpectedly preloaded before clicking [P] tab")
	}
	m, _ = m.Update(licenseDetailTabClickMsg{tab: 2})
	if m.detailTab != 2 {
		t.Fatalf("detailTab=%d after click, want 2", m.detailTab)
	}
	if m.pemViewport.TotalLineCount() == 0 {
		t.Fatal("clicking [P] tab did NOT load PEM content into viewport (regression)")
	}
}

// TestAutoFit_ShortAudienceFreesSpaceForSubject is the regression guard for
// the user complaint "il y a des colonnes avec du texte strippé tandis qu'il
// y a la place pour le texte contenu". Two scenarios are compared:
//
//   - short  : every AUDIENCE cell is the 4-char "prod"
//   - normal : AUDIENCE cells span up to the declared min (16 chars)
//
// After the migration to setAutoFitRows, the SUBJECT column in the "short"
// scenario must be STRICTLY WIDER than in "normal" because the saved
// AUDIENCE cells are redistributed by the weight vector. The pre-migration
// behaviour gave the same widths in both scenarios.
func TestAutoFit_ShortAudienceFreesSpaceForSubject(t *testing.T) {
	mk := func(audience string) *licensesModel {
		m := newLicensesModel(nil)
		m.width, m.hgt = 144, 44
		m.rows = []*ent.License{{
			ID: uuid.New(), LicenseUUID: uuid.NewString(),
			Subject:    "alice@example.test",
			IssuerName: "iss",
			Audience:   []string{audience},
			Status:     "active",
			NotBefore:  time.Now(),
			NotAfter:   time.Now().Add(time.Hour),
		}}
		m.rebuildTable()
		return &m
	}
	short := mk("prod")
	normal := mk(strings.Repeat("x", 30))

	shortSubject := short.table.Columns()[1].Width
	normalSubject := normal.table.Columns()[1].Width
	if shortSubject <= normalSubject {
		t.Errorf("auto-fit not reclaiming AUDIENCE slack: SUBJECT short=%d normal=%d",
			shortSubject, normalSubject)
	}
}

// TestAutoFit_DumpAt200LicenseShortFields dumps a licenses view where every
// data column is short, so the operator can see SUBJECT/AUDIENCE/FEATURES
// stretched into a wide window with the unused space reclaimed.
func TestAutoFit_DumpAt200LicenseShortFields(t *testing.T) {
	m := newLicensesModel(nil)
	m.width, m.hgt = 200, 44
	m.rows = []*ent.License{
		{ID: uuid.New(), LicenseUUID: uuid.NewString(),
			Subject: "alice", IssuerName: "iss-A",
			Audience: []string{"prod"}, Features: []string{"audit"},
			Status:    "active",
			NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)},
		{ID: uuid.New(), LicenseUUID: uuid.NewString(),
			Subject: "bob@longer.subject.with.spaces", IssuerName: "iss-Beta-2026Q4",
			Audience: []string{"us-east-1", "us-west-2"}, Features: []string{"totp", "remote-revoke"},
			Status:    "active",
			NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)},
	}
	m.rebuildTable()
	for i, ln := range strings.Split(m.View(), "\n") {
		if i > 10 {
			break
		}
		t.Logf("F%02d|%s|", i, ln)
	}
}

// TestLicenses_DumpAt80 dumps the licences view at a very narrow width
// (80 cols < minSum) so the operator can confirm rows no longer wrap.
func TestLicenses_DumpAt80(t *testing.T) {
	m := newLicensesModel(nil)
	m.rows = []*ent.License{{
		ID: uuid.New(), LicenseUUID: uuid.NewString(),
		Subject: "alice@example.test", IssuerName: "issuer-prod-2026Q2",
		Audience: []string{"prod"}, Features: []string{"audit", "totp"},
		Status: "active", NotBefore: time.Now(),
		NotAfter: time.Now().Add(24 * time.Hour),
	}}
	m.width, m.hgt = 80, 44
	m.rebuildTable()
	for i, ln := range strings.Split(m.View(), "\n") {
		if i > 12 {
			break
		}
		t.Logf("N%02d|%s|", i, ln)
	}
}

// TestIssuers_DumpDetailAlignment renders the issuer detail panel so we can
// confirm that the [a]/[e]/[E]/[K]/[x] action-hint column starts at the same
// X for every line. HintKey has Padding(0,1) — GlowRed (used for [x]) does
// not, which used to shift [x] left by 1 cell.
func TestIssuers_DumpDetailAlignment(t *testing.T) {
	m := newIssuersModel(nil)
	m.width, m.hgt = 144, 44
	m.rows = []*ent.Issuer{{ID: uuid.New(), Name: "prod-2026", KeyID: "key-42",
		Active: true, CreatedAt: time.Now()}}
	m.rebuildTable()
	for i, ln := range strings.Split(m.View(), "\n") {
		t.Logf("D%02d|%s|", i, ln)
	}
}

// TestRevocation_DumpDetailAt200 dumps the revocation view at width=200 so
// the operator can confirm visually that the detail panel + the widened
// columns work together. Visible only with `go test -v`.
func TestRevocation_DumpDetailAt200(t *testing.T) {
	m := newRevocationModel(nil)
	m.width, m.hgt = 200, 44
	licID := uuid.New()
	m.rows = []service.RevocationView{{
		LicenseID:   licID,
		LicenseUUID: "feedfacedeadbeef-cafe-babe-0000-000000000001",
		Subject:     "alice@example.test",
		KeyID:       "key-prod-2026Q2",
		Reason:      "key compromise — operator action 2026-05-27",
		RevokedAt:   time.Date(2026, 5, 27, 14, 32, 0, 0, time.UTC),
		RevokedBy:   "operator-jdoe",
	}}
	m.detailLic = &ent.License{
		ID:        licID,
		Audience:  []string{"prod", "us-east-1"},
		Features:  []string{"audit", "totp", "remote-revoke"},
		NotBefore: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:  time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	m.rebuildTable()
	for i, ln := range strings.Split(m.View(), "\n") {
		t.Logf("R%02d|%s|", i, ln)
	}
}

// TestStress_IssuersRenderAt100Rows mirrors the visual check for issuers.
func TestStress_IssuersRenderAt100Rows(t *testing.T) {
	rows := make([]*ent.Issuer, 100)
	now := time.Now()
	for i := range rows {
		rows[i] = &ent.Issuer{
			ID:        uuid.New(),
			Name:      "issuer-" + strings.Repeat("z", 1+(i%25)),
			KeyID:     "key-" + uuid.NewString()[:8],
			Active:    i == 0,
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		}
	}
	m := newIssuersModel(nil)
	m.width = 144
	m.hgt = 44
	m.rows = rows
	m.rebuildTable()
	dump := m.View()

	maxW := 0
	for _, ln := range strings.Split(dump, "\n") {
		if w := lipgloss.Width(ln); w > maxW {
			maxW = w
		}
	}
	if maxW > m.width {
		t.Errorf("widest line = %d > terminal width %d", maxW, m.width)
	}
	rowsAllSameWidth(t, "issuers N=100", dump)
}
