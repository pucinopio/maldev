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
// VHS-EQUIVALENT E2E TESTS — these mirror the recordings under vhs/*.tape.
// VHS produces a GIF for visual demos; this file produces an exit code for
// CI. When a tape is added or a binding changes, update BOTH so the visual
// demo and the regression guard stay in sync.
// ═══════════════════════════════════════════════════════════════════════════

// makePEM returns a deterministic multi-line PEM that's long enough for
// every scroll affordance to produce an observable YOffset delta.
func makePEM(t *testing.T) []byte {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("-----BEGIN MALDEV LICENSE v2-----\n")
	for i := 0; i < 200; i++ {
		sb.WriteString(strings.Repeat("A", 64))
		sb.WriteString("\n")
	}
	sb.WriteString("-----END MALDEV LICENSE v2-----\n")
	return []byte(sb.String())
}

// TestVHS_LicensesNavAndPEMScroll mirrors vhs/licenses-nav-and-pem-scroll.tape.
// Documents AND verifies every keystroke recorded in the tape:
//
//	 2        → go to Licences tab
//	 Down ×3  → cursor moves to row 3
//	 P        → open PEM detail tab
//	 j ×3     → scroll viewport down (3 lines each)
//	 Space ×2 → half-page scroll twice
//	 G        → go to bottom of PEM
//	 g        → go back to top of PEM
//	 Up ×3    → cursor moves back to row 0 (PEM auto-reloads)
func TestVHS_LicensesNavAndPEMScroll(t *testing.T) {
	rows := []*ent.License{
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "alice",
			Pem: makePEM(t), Status: "active", NotBefore: time.Now(),
			NotAfter: time.Now().Add(time.Hour)},
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "bob",
			Pem: makePEM(t), Status: "active", NotBefore: time.Now(),
			NotAfter: time.Now().Add(time.Hour)},
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "carol",
			Pem: makePEM(t), Status: "active", NotBefore: time.Now(),
			NotAfter: time.Now().Add(time.Hour)},
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "dave",
			Pem: makePEM(t), Status: "active", NotBefore: time.Now(),
			NotAfter: time.Now().Add(time.Hour)},
	}

	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})

	// Step 1 — Type "2" : navigate to Licences.
	m = drive(m, '2')
	m, _ = m.Update(LicensesLoadedMsg{Rows: rows})
	if r := rootOf(t, m); r.active != ViewLicenses {
		t.Fatalf("after '2': active=%s, want ViewLicenses", r.active)
	}

	// Step 2 — Down × 3.
	for i := 0; i < 3; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if r := rootOf(t, m); r.licenses.table.Cursor() != 3 {
		t.Errorf("after 3× Down: cursor=%d, want 3", r.licenses.table.Cursor())
	}

	// Step 3 — Type "P" : open PEM detail tab.
	m = drive(m, 'P')
	if r := rootOf(t, m); r.licenses.detailTab != 2 {
		t.Fatalf("after 'P': detailTab=%d, want 2", r.licenses.detailTab)
	}
	pemBefore := rootOf(t, m).licenses.pemViewport.YOffset

	// Step 4 — j × 3 (each scrolls by 3 lines so total +9).
	for i := 0; i < 3; i++ {
		m = drive(m, 'j')
	}
	pemAfterJ := rootOf(t, m).licenses.pemViewport.YOffset
	if pemAfterJ <= pemBefore {
		t.Errorf("3× 'j' did not scroll PEM: YOffset %d→%d", pemBefore, pemAfterJ)
	}

	// Step 5 — Space × 2 (half-page down).
	for i := 0; i < 2; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	}
	pemAfterSpace := rootOf(t, m).licenses.pemViewport.YOffset
	if pemAfterSpace <= pemAfterJ {
		t.Errorf("2× Space did not scroll: YOffset %d→%d", pemAfterJ, pemAfterSpace)
	}

	// Step 6 — G : go to bottom.
	m = drive(m, 'G')
	r := rootOf(t, m)
	wantBottom := r.licenses.pemViewport.TotalLineCount() - r.licenses.pemViewport.Height
	if got := r.licenses.pemViewport.YOffset; got != wantBottom {
		t.Errorf("after 'G': YOffset=%d, want %d (bottom)", got, wantBottom)
	}

	// Step 7 — g : back to top.
	m = drive(m, 'g')
	if got := rootOf(t, m).licenses.pemViewport.YOffset; got != 0 {
		t.Errorf("after 'g': YOffset=%d, want 0 (top)", got)
	}

	// Step 8 — Up × 3 : cursor back to row 0 (table nav still works on PEM tab).
	for i := 0; i < 3; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	if r := rootOf(t, m); r.licenses.table.Cursor() != 0 {
		t.Errorf("after 3× Up on PEM tab: cursor=%d, want 0 (table nav must work)", r.licenses.table.Cursor())
	}
	if r := rootOf(t, m); r.licenses.detailTab != 2 {
		t.Errorf("Up navigation must NOT leave PEM tab: detailTab=%d", r.licenses.detailTab)
	}
}

// TestVHS_LicensesImportExport mirrors vhs/licenses-import-export.tape:
//
//	 2        → goto Licences
//	 i        → push filepicker overlay
//	 Esc      → close picker
//	 Down     → select row 1
//	 E        → push export-path input overlay
//
// The actual file write needs a real service, so we stop at the overlay-push
// assertion — same depth as what the tape records visually.
func TestVHS_LicensesImportExport(t *testing.T) {
	rows := []*ent.License{
		{ID: uuid.New(), LicenseUUID: uuid.NewString(), Subject: "alice",
			Pem: []byte("PEM-A"), Status: "active", NotBefore: time.Now(),
			NotAfter: time.Now().Add(time.Hour)},
	}

	var m tea.Model = New(nil, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = drive(m, '2')
	m, _ = m.Update(LicensesLoadedMsg{Rows: rows})

	// Step — i : trigger import. The keypress returns a Cmd that pushes a
	// filepicker overlay; execute that Cmd to land it in m.overlays.
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			mm, _ = mm.Update(msg)
		}
	}
	if r := rootOf(t, mm); len(r.overlays) == 0 {
		t.Fatalf("after 'i': no overlay pushed")
	} else if _, ok := r.overlays[len(r.overlays)-1].(*filePickerOverlay); !ok {
		t.Errorf("after 'i': top overlay = %T, want *filePickerOverlay", r.overlays[len(r.overlays)-1])
	}

	// Step — Esc : close the filepicker overlay. The picker returns a
	// Cmd that emits OverlayDoneMsg{Result: nil}; we drain it so the
	// overlay is actually popped before we send the next key.
	mm, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			mm, _ = mm.Update(msg)
		}
	}

	// Step — E : push the export-path input overlay.
	m = mm
	mm, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			mm, _ = mm.Update(msg)
		}
	}
	if r := rootOf(t, mm); len(r.overlays) == 0 {
		t.Fatalf("after 'E': no overlay pushed")
	} else {
		top := r.overlays[len(r.overlays)-1]
		if in, ok := top.(*inputOverlay); !ok {
			t.Errorf("after 'E': top overlay = %T, want *inputOverlay", top)
		} else if in.id != OverlayIDLicenseExport {
			t.Errorf("after 'E': overlay id=%q, want %q", in.id, OverlayIDLicenseExport)
		}
	}
}
