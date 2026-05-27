package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// ═══════════════════════════════════════════════════════════════════════════
// ISSUERS SCREEN — KEY DISPATCH TRUTH TABLE
// ═══════════════════════════════════════════════════════════════════════════
//
// One row per (context, key) combination. Same format as the licenses table.
// Effects use the issuersSnap struct below.
// ═══════════════════════════════════════════════════════════════════════════

type issuersSnap struct {
	cursor   int
	detail   bool
	overlays int
}

func snapIssuers(t *testing.T, m tea.Model) issuersSnap {
	t.Helper()
	r := rootOf(t, m)
	return issuersSnap{
		cursor:   r.issuers.table.Cursor(),
		detail:   r.issuers.detail,
		overlays: len(r.overlays),
	}
}

func TestIssuersKeyDispatchTruthTable(t *testing.T) {
	rows := []*ent.Issuer{
		{ID: uuid.New(), Name: "prod-A", KeyID: "k-A", Active: true, CreatedAt: time.Now()},
		{ID: uuid.New(), Name: "prod-B", KeyID: "k-B", Active: false, CreatedAt: time.Now()},
		{ID: uuid.New(), Name: "prod-C", KeyID: "k-C", Active: false, CreatedAt: time.Now()},
	}

	type row struct {
		name    string
		setup   func(m tea.Model) tea.Model
		key     tea.KeyMsg
		assert  func(t *testing.T, before, after issuersSnap)
	}

	cases := []row{
		{"↓ navigates table", nil, tea.KeyMsg{Type: tea.KeyDown}, func(t *testing.T, b, a issuersSnap) {
			if b.cursor == a.cursor {
				t.Errorf("cursor did not move")
			}
		}},
		{"↑ navigates table (seed first)", func(m tea.Model) tea.Model {
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
			return m
		}, tea.KeyMsg{Type: tea.KeyUp}, func(t *testing.T, b, a issuersSnap) {
			if b.cursor == a.cursor {
				t.Errorf("cursor did not move up")
			}
		}},
		{"j navigates table", nil, runRune('j'), func(t *testing.T, b, a issuersSnap) {
			if b.cursor == a.cursor {
				t.Errorf("j cursor did not move")
			}
		}},
		{"d toggles detail", nil, runRune('d'), func(t *testing.T, b, a issuersSnap) {
			if b.detail == a.detail {
				t.Errorf("detail flag did not toggle")
			}
		}},
		{"n opens new-issuer input overlay", nil, runRune('n'), func(t *testing.T, b, a issuersSnap) {
			if a.overlays <= b.overlays {
				t.Errorf("no overlay pushed")
			}
		}},
		{"i opens import filepicker", nil, runRune('i'), func(t *testing.T, b, a issuersSnap) {
			if a.overlays <= b.overlays {
				t.Errorf("no overlay pushed")
			}
		}},
		{"E opens export-public input overlay", nil, runRune('E'), func(t *testing.T, b, a issuersSnap) {
			if a.overlays <= b.overlays {
				t.Errorf("no overlay pushed")
			}
		}},
		{"K opens export-private confirm overlay", nil, runRune('K'), func(t *testing.T, b, a issuersSnap) {
			if a.overlays <= b.overlays {
				t.Errorf("no overlay pushed")
			}
		}},
		{"x opens retire confirm overlay", nil, runRune('x'), func(t *testing.T, b, a issuersSnap) {
			if a.overlays <= b.overlays {
				t.Errorf("no overlay pushed")
			}
		}},
		{"e opens rename input overlay", nil, runRune('e'), func(t *testing.T, b, a issuersSnap) {
			if a.overlays <= b.overlays {
				t.Errorf("no overlay pushed")
			}
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
			m = drive(m, '3') // ViewIssuers
			m, _ = m.Update(IssuersLoadedMsg{Rows: rows})
			if c.setup != nil {
				m = c.setup(m)
			}

			before := snapIssuers(t, m)
			mm, cmd := m.Update(c.key)
			if cmd != nil {
				if msg := cmd(); msg != nil {
					mm, _ = mm.Update(msg)
				}
			}
			after := snapIssuers(t, mm)
			c.assert(t, before, after)
		})
	}
}
