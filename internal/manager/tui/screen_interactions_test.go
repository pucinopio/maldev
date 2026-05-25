package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
	"github.com/oioio-space/maldev/internal/manager/tui/widgets"
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
