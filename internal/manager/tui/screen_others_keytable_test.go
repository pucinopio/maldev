package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// ═══════════════════════════════════════════════════════════════════════════
// TRUTH TABLES — recipients / identities / revocation / TOTP / audit
// One file per screen would scale but adds boilerplate; group the smaller
// screens here. Each test runs a setup → key → effect assertion just like
// the licenses + issuers tables.
// ═══════════════════════════════════════════════════════════════════════════

// ── RECIPIENTS ─────────────────────────────────────────────────────────────

func TestRecipientsKeyDispatchTruthTable(t *testing.T) {
	rows := []*ent.RecipientKey{
		{ID: uuid.New(), Name: "acme", PublicKey: []byte("k1"), CreatedAt: time.Now()},
		{ID: uuid.New(), Name: "bob", PublicKey: []byte("k2"), CreatedAt: time.Now()},
	}
	cases := []struct {
		name   string
		setup  func(m tea.Model) tea.Model
		key    tea.KeyMsg
		expect string // "table-move" | "detail-toggle" | "overlay-push"
	}{
		{"↓ navigates table", nil, tea.KeyMsg{Type: tea.KeyDown}, "table-move"},
		{"j navigates table", nil, runRune('j'), "table-move"},
		{"d toggles detail", nil, runRune('d'), "detail-toggle"},
		{"n opens new-recipient overlay", nil, runRune('n'), "overlay-push"},
		{"e opens rename overlay", nil, runRune('e'), "overlay-push"},
		{"E opens export-pub overlay", nil, runRune('E'), "overlay-push"},
		{"x opens delete confirm", nil, runRune('x'), "overlay-push"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
			m = drive(m, '4') // ViewRecipients
			m, _ = m.Update(RecipientsLoadedMsg{Rows: rows})
			if c.setup != nil {
				m = c.setup(m)
			}
			r0 := rootOf(t, m)
			cursor0 := r0.recipients.table.Cursor()
			detail0 := r0.recipients.detail
			ov0 := len(r0.overlays)

			mm, cmd := m.Update(c.key)
			if cmd != nil {
				if msg := cmd(); msg != nil {
					mm, _ = mm.Update(msg)
				}
			}
			r1 := rootOf(t, mm)
			switch c.expect {
			case "table-move":
				if r1.recipients.table.Cursor() == cursor0 {
					t.Errorf("cursor stayed at %d", cursor0)
				}
			case "detail-toggle":
				if r1.recipients.detail == detail0 {
					t.Errorf("detail did not toggle")
				}
			case "overlay-push":
				if len(r1.overlays) <= ov0 {
					t.Errorf("no overlay pushed (was %d, now %d)", ov0, len(r1.overlays))
				}
			}
		})
	}
}

// ── IDENTITIES ─────────────────────────────────────────────────────────────

func TestIdentitiesKeyDispatchTruthTable(t *testing.T) {
	rows := []*ent.Identity{
		{ID: uuid.New(), Name: "id-A", Sha256: "aaaaaaaa", CreatedAt: time.Now()},
		{ID: uuid.New(), Name: "id-B", Sha256: "bbbbbbbb", CreatedAt: time.Now()},
	}
	cases := []struct {
		name   string
		setup  func(m tea.Model) tea.Model
		key    tea.KeyMsg
		expect string
	}{
		{"↓ navigates table", nil, tea.KeyMsg{Type: tea.KeyDown}, "table-move"},
		{"d toggles detail", nil, runRune('d'), "detail-toggle"},
		{"n opens new-identity overlay", nil, runRune('n'), "overlay-push"},
		{"e opens rename overlay", nil, runRune('e'), "overlay-push"},
		{"E opens export-bin overlay", nil, runRune('E'), "overlay-push"},
		{"R opens regen confirm overlay", nil, runRune('R'), "overlay-push"},
		{"x opens delete confirm", nil, runRune('x'), "overlay-push"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
			m = drive(m, '5') // ViewIdentities
			m, _ = m.Update(IdentitiesLoadedMsg{Rows: rows})
			if c.setup != nil {
				m = c.setup(m)
			}
			r0 := rootOf(t, m)
			cursor0 := r0.identities.table.Cursor()
			detail0 := r0.identities.detail
			ov0 := len(r0.overlays)

			mm, cmd := m.Update(c.key)
			if cmd != nil {
				if msg := cmd(); msg != nil {
					mm, _ = mm.Update(msg)
				}
			}
			r1 := rootOf(t, mm)
			switch c.expect {
			case "table-move":
				if r1.identities.table.Cursor() == cursor0 {
					t.Errorf("cursor stayed at %d", cursor0)
				}
			case "detail-toggle":
				if r1.identities.detail == detail0 {
					t.Errorf("detail did not toggle")
				}
			case "overlay-push":
				if len(r1.overlays) <= ov0 {
					t.Errorf("no overlay pushed (was %d, now %d)", ov0, len(r1.overlays))
				}
			}
		})
	}
}

// ── REVOCATION ─────────────────────────────────────────────────────────────

func TestRevocationKeyDispatchTruthTable(t *testing.T) {
	rows := []service.RevocationView{
		{LicenseID: uuid.New(), LicenseUUID: "u1", Subject: "alice", KeyID: "k1",
			Reason: "compromise", RevokedAt: time.Now(), RevokedBy: "op"},
		{LicenseID: uuid.New(), LicenseUUID: "u2", Subject: "bob", KeyID: "k2",
			Reason: "expired", RevokedAt: time.Now(), RevokedBy: "op"},
	}
	cases := []struct {
		name   string
		setup  func(m tea.Model) tea.Model
		key    tea.KeyMsg
		expect string
	}{
		{"↓ navigates table", nil, tea.KeyMsg{Type: tea.KeyDown}, "table-move"},
		{"d toggles detail", nil, runRune('d'), "detail-toggle"},
		{"x opens remove confirm overlay", nil, runRune('x'), "overlay-push"},
		{"E opens export-CRL input overlay", nil, runRune('E'), "overlay-push"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m tea.Model = New(nil, nil, SessionReady)
			m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
			m = drive(m, '6') // ViewRevocation
			m, _ = m.Update(RevocationLoadedMsg{Rows: rows})
			if c.setup != nil {
				m = c.setup(m)
			}
			r0 := rootOf(t, m)
			cursor0 := r0.revocation.table.Cursor()
			detail0 := r0.revocation.detail
			ov0 := len(r0.overlays)

			mm, cmd := m.Update(c.key)
			if cmd != nil {
				if msg := cmd(); msg != nil {
					mm, _ = mm.Update(msg)
				}
			}
			r1 := rootOf(t, mm)
			switch c.expect {
			case "table-move":
				if r1.revocation.table.Cursor() == cursor0 {
					t.Errorf("cursor stayed at %d", cursor0)
				}
			case "detail-toggle":
				if r1.revocation.detail == detail0 {
					t.Errorf("detail did not toggle")
				}
			case "overlay-push":
				if len(r1.overlays) <= ov0 {
					t.Errorf("no overlay pushed (was %d, now %d)", ov0, len(r1.overlays))
				}
			}
		})
	}
}
