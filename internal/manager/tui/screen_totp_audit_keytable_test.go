package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// ═══════════════════════════════════════════════════════════════════════════
// TRUTH TABLES — TOTP + audit screens.
// ═══════════════════════════════════════════════════════════════════════════

// ── TOTP ───────────────────────────────────────────────────────────────────

func TestTOTPKeyDispatchTruthTable(t *testing.T) {
	rows := []*ent.TOTPSecret{
		{ID: uuid.New(), AccountLabel: "alice@app", CreatedAt: time.Now()},
		{ID: uuid.New(), AccountLabel: "bob@app", CreatedAt: time.Now()},
	}
	cases := []struct {
		name   string
		key    tea.KeyMsg
		expect string // "overlay-push" | "cursor-move"
	}{
		{"n opens new-TOTP input overlay", runRune('n'), "overlay-push"},
		{"x opens delete confirm overlay", runRune('x'), "overlay-push"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
			m = drive(m, '8') // ViewTOTP
			m, _ = m.Update(TOTPLoadedMsg{Rows: rows})

			r0 := rootOf(t, m)
			ov0 := len(r0.overlays)

			mm, cmd := m.Update(c.key)
			if cmd != nil {
				if msg := cmd(); msg != nil {
					mm, _ = mm.Update(msg)
				}
			}
			r1 := rootOf(t, mm)
			if c.expect == "overlay-push" && len(r1.overlays) <= ov0 {
				t.Errorf("no overlay pushed (was %d, now %d)", ov0, len(r1.overlays))
			}
		})
	}
}

// ── AUDIT ──────────────────────────────────────────────────────────────────

func TestAuditKeyDispatchTruthTable(t *testing.T) {
	rows := []*ent.AuditEvent{
		{ID: uuid.New(), Kind: "license.issue", Actor: "op",
			TargetKind: "License", TargetID: "lic-1", CreatedAt: time.Now()},
		{ID: uuid.New(), Kind: "issuer.generate", Actor: "op",
			TargetKind: "Issuer", TargetID: "iss-1", CreatedAt: time.Now()},
	}
	cases := []struct {
		name   string
		setup  func(m tea.Model) tea.Model
		key    tea.KeyMsg
		expect string // "overlay-push" | "filter-change" | "detail-open" | "detail-close"
	}{
		{"E opens CSV export overlay", nil, runRune('E'), "overlay-push"},
		{"J opens JSON export overlay", nil, runRune('J'), "overlay-push"},
		{"l filters to license events", nil, runRune('l'), "filter-change"},
		{"k filters to key events", nil, runRune('k'), "filter-change"},
		{"i filters to identity events", nil, runRune('i'), "filter-change"},
		{"p filters to probe events", nil, runRune('p'), "filter-change"},
		{"s filters to server events", nil, runRune('s'), "filter-change"},
		{"d opens detail panel", nil, runRune('d'), "detail-open"},
		{"esc in detail closes it", func(m tea.Model) tea.Model {
			return drive(m, 'd')
		}, tea.KeyMsg{Type: tea.KeyEsc}, "detail-close"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
			m = drive(m, '9') // ViewAudit
			m, _ = m.Update(AuditLoadedMsg{Rows: rows})
			if c.setup != nil {
				m = c.setup(m)
			}

			r0 := rootOf(t, m)
			ov0 := len(r0.overlays)
			filter0 := r0.audit.filter
			detail0 := r0.audit.detail

			mm, cmd := m.Update(c.key)
			if cmd != nil {
				if msg := cmd(); msg != nil {
					mm, _ = mm.Update(msg)
				}
			}
			r1 := rootOf(t, mm)
			switch c.expect {
			case "overlay-push":
				if len(r1.overlays) <= ov0 {
					t.Errorf("no overlay pushed (was %d, now %d)", ov0, len(r1.overlays))
				}
			case "filter-change":
				if r1.audit.filter == filter0 {
					t.Errorf("filter unchanged (still %v)", filter0)
				}
			case "detail-open":
				if !r1.audit.detail || detail0 {
					t.Errorf("detail did not open: was %v, now %v", detail0, r1.audit.detail)
				}
			case "detail-close":
				if r1.audit.detail || !detail0 {
					t.Errorf("detail did not close: was %v, now %v", detail0, r1.audit.detail)
				}
			}
		})
	}
}
