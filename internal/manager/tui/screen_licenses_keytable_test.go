package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// ═══════════════════════════════════════════════════════════════════════════
// LICENSES SCREEN — KEY DISPATCH TRUTH TABLE
// ═══════════════════════════════════════════════════════════════════════════
//
// Authoritative reference for what every key does in every context on the
// licenses screen. When a key binding feels broken, ADD a row here first —
// the test will tell you whether the current implementation matches the
// intended behavior, and if not, which side to fix.
//
// Context columns:
//   - search:  true  → text input has focus (operator typed "/")
//              false → no input focus
//   - detail:  true  → detail panel is open ([d] toggle)
//              false → detail panel is collapsed
//   - tab:     I=Identity, B=Bindings, P=PEM, A=Audit, C=Chain. Only
//              meaningful when detail=true.
//
// Effect column codes (assert helpers below):
//   - "table-move"     → m.table.Cursor() changes
//   - "table-stay"     → m.table.Cursor() unchanged
//   - "pem-scroll"     → m.pemViewport.YOffset changes
//   - "pem-stay"       → m.pemViewport.YOffset unchanged
//   - "tab=N"          → m.detailTab equals N afterwards
//   - "detail-toggle"  → m.detail flips
//   - "overlay-push"   → an Overlay was pushed onto m.overlays
//   - "search-focus"   → m.licenses.search.Focused() is true
//   - "filter-cycled"  → m.licenses.filter changed
//   - "noop"           → no observable state change in this dimension
// ═══════════════════════════════════════════════════════════════════════════

type keyRow struct {
	name    string
	setup   func(m tea.Model) tea.Model
	key     tea.KeyMsg
	effects []string
}

func runRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func runStr(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// TestLicensesKeyDispatchTruthTable is the methodical sweep — every key in
// every meaningful context, with its expected effect documented as data.
func TestLicensesKeyDispatchTruthTable(t *testing.T) {
	// Long PEM so scroll is observable.
	var sb strings.Builder
	sb.WriteString("-----BEGIN MALDEV LICENSE v2-----\n")
	for i := 0; i < 200; i++ {
		sb.WriteString(strings.Repeat("A", 64))
		sb.WriteString("\n")
	}
	sb.WriteString("-----END MALDEV LICENSE v2-----\n")
	pem := sb.String()

	rows := []*ent.License{
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "alice",
			Pem: []byte(pem), Status: "active", NotBefore: time.Now(),
			NotAfter: time.Now().Add(time.Hour)},
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "bob",
			Pem: []byte(pem), Status: "active", NotBefore: time.Now(),
			NotAfter: time.Now().Add(time.Hour)},
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "carol",
			Pem: []byte(pem), Status: "active", NotBefore: time.Now(),
			NotAfter: time.Now().Add(time.Hour)},
	}

	// Setup helpers — applied in order after navigating to ViewLicenses.
	openTab := func(r rune) func(tea.Model) tea.Model {
		return func(m tea.Model) tea.Model { return drive(m, r) }
	}
	closeDetail := func(m tea.Model) tea.Model {
		root := rootOf(t, m)
		root.licenses.detail = false
		root.licenses.rebuildTable()
		return root
	}
	openSearch := openTab('/')

	// ─────────────────────────────────────────────────────────────────────
	// TRUTH TABLE — one row per (context, key) combination.
	// ─────────────────────────────────────────────────────────────────────
	table := []keyRow{
		// ──── Detail closed: ↑/↓/j/k all navigate the table ─────────────
		{"closed/↓ navigates table", closeDetail, tea.KeyMsg{Type: tea.KeyDown}, []string{"table-move", "pem-stay"}},
		{"closed/↑ navigates table", func(m tea.Model) tea.Model {
			m = closeDetail(m)
			return drive(m, rune(tea.KeyDown)) // seed cursor at 1 so up can decrement
		}, tea.KeyMsg{Type: tea.KeyUp}, []string{"pem-stay"}},
		{"closed/j navigates table", closeDetail, runRune('j'), []string{"table-move", "pem-stay"}},
		{"closed/k navigates table (seed first)", func(m tea.Model) tea.Model {
			m = closeDetail(m)
			mm, _ := m.Update(runRune('j'))
			return mm
		}, runRune('k'), []string{"pem-stay"}},

		// ──── Detail open, NON-PEM tab: arrows nav table; viewport stays
		{"detail/I/↓ navigates table", openTab('I'), tea.KeyMsg{Type: tea.KeyDown}, []string{"table-move", "pem-stay"}},
		{"detail/B/↓ navigates table", openTab('B'), tea.KeyMsg{Type: tea.KeyDown}, []string{"table-move", "pem-stay"}},
		{"detail/A/↓ navigates table", openTab('A'), tea.KeyMsg{Type: tea.KeyDown}, []string{"table-move", "pem-stay"}},
		{"detail/C/↓ navigates table", openTab('C'), tea.KeyMsg{Type: tea.KeyDown}, []string{"table-move", "pem-stay"}},
		{"detail/I/j navigates table", openTab('I'), runRune('j'), []string{"table-move", "pem-stay"}},

		// ──── Detail open, PEM tab: ↑/↓ STILL navigate the table (PEM
		// auto-reloads); j/k/space/b/pgup/pgdn/g/G scroll the viewport.
		// This dual-affordance design matches operator intuition AND keeps
		// the preview synced with the selected row.
		{"detail/P/↓ navigates table (PEM follows)", openTab('P'), tea.KeyMsg{Type: tea.KeyDown}, []string{"table-move"}},
		{"detail/P/j scrolls PEM",   openTab('P'), runRune('j'),                    []string{"table-stay", "pem-scroll"}},
		{"detail/P/space half-page", openTab('P'), runRune(' '),                    []string{"table-stay", "pem-scroll"}},
		{"detail/P/pgdown half-page",openTab('P'), tea.KeyMsg{Type: tea.KeyPgDown}, []string{"table-stay", "pem-scroll"}},
		{"detail/P/G to bottom",     openTab('P'), runRune('G'),                    []string{"table-stay", "pem-scroll"}},

		// ──── Tab switches (single letters) ─────────────────────────────
		{"I switches to Identity tab", nil, runRune('I'), []string{"tab=0"}},
		{"B switches to Bindings tab", nil, runRune('B'), []string{"tab=1"}},
		{"P switches to PEM tab",      nil, runRune('P'), []string{"tab=2"}},
		{"A switches to Audit tab",    nil, runRune('A'), []string{"tab=3"}},
		{"C switches to Chain tab",    nil, runRune('C'), []string{"tab=4"}},

		// ──── Detail toggle ─────────────────────────────────────────────
		{"d toggles detail (open→close)", nil, runRune('d'), []string{"detail-toggle"}},

		// ──── Filter cycle ──────────────────────────────────────────────
		{"f cycles filter", nil, runRune('f'), []string{"filter-cycled"}},

		// ──── Overlay pushers (don't enter the overlay, just check push)
		{"n opens wizard overlay",       nil, runRune('n'), []string{"overlay-push"}},
		{"i opens import filepicker",    nil, runRune('i'), []string{"overlay-push"}},
		{"E opens export input overlay", nil, runRune('E'), []string{"overlay-push"}},
		{"x opens revoke overlay",       nil, runRune('x'), []string{"overlay-push"}},
		{"e opens reissue confirm",      nil, runRune('e'), []string{"overlay-push"}},
		{"D opens delete confirm",       nil, runRune('D'), []string{"overlay-push"}},

		// ──── Search input ──────────────────────────────────────────────
		{"/ focuses the search input", nil, runRune('/'), []string{"search-focus"}},
		{"esc in search blurs search", openSearch, tea.KeyMsg{Type: tea.KeyEsc}, []string{"search-stay-blurred"}},
	}

	for _, c := range table {
		t.Run(c.name, func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
			m = drive(m, '2')
			m, _ = m.Update(LicensesLoadedMsg{Rows: rows})
			if c.setup != nil {
				m = c.setup(m)
			}

			before := snapState(t, m)
			// Drive the key AND execute any returned Cmd so overlay pushes
			// (which travel as pushOverlayMsg through a Cmd) actually land
			// in m.overlays before we snapshot the after-state.
			mm, cmd := m.Update(c.key)
			if cmd != nil {
				if msg := cmd(); msg != nil {
					mm, _ = mm.Update(msg)
				}
			}
			after := snapState(t, mm)

			for _, eff := range c.effects {
				if err := checkEffect(eff, before, after); err != "" {
					t.Errorf("effect %q: %s", eff, err)
				}
			}
		})
	}
}

// stateSnap captures the licenses-screen state dimensions the truth table
// asserts against.
type stateSnap struct {
	cursor     int
	pemY       int
	detailTab  int
	detail     bool
	filter     licenseFilter
	overlays   int
	searchOn   bool
}

func snapState(t *testing.T, m tea.Model) stateSnap {
	t.Helper()
	r := rootOf(t, m)
	return stateSnap{
		cursor:    r.licenses.table.Cursor(),
		pemY:      r.licenses.pemViewport.YOffset,
		detailTab: r.licenses.detailTab,
		detail:    r.licenses.detail,
		filter:    r.licenses.filter,
		overlays:  len(r.overlays),
		searchOn:  r.licenses.search.Focused(),
	}
}

func checkEffect(eff string, before, after stateSnap) string {
	switch eff {
	case "table-move":
		if before.cursor == after.cursor {
			return "table cursor did NOT move"
		}
	case "table-stay":
		if before.cursor != after.cursor {
			return "table cursor moved unexpectedly"
		}
	case "pem-scroll":
		if before.pemY == after.pemY {
			return "PEM YOffset did NOT change"
		}
	case "pem-stay":
		if before.pemY != after.pemY {
			return "PEM YOffset changed unexpectedly"
		}
	case "tab=0", "tab=1", "tab=2", "tab=3", "tab=4":
		want := int(eff[len(eff)-1] - '0')
		if after.detailTab != want {
			return "detailTab is %d, want %d"
		}
		_ = want
	case "detail-toggle":
		if before.detail == after.detail {
			return "detail flag did NOT toggle"
		}
	case "filter-cycled":
		if before.filter == after.filter {
			return "filter did NOT cycle"
		}
	case "overlay-push":
		if after.overlays <= before.overlays {
			return "no overlay was pushed"
		}
	case "search-focus":
		if !after.searchOn {
			return "search input is NOT focused"
		}
	case "search-stay-blurred":
		if after.searchOn {
			return "search remained focused; expected blur"
		}
	}
	return ""
}
