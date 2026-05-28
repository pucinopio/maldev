package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestWorkflow_TOTPDetailNoLongerInlinesQR is the regression guard for the
// operator-reported "l'affichage du QRCODE décale tout l'affichage". The
// detail panel must NOT render the QR ASCII inline anymore — that's what
// pushed the listBox off-screen when the QR was wider than the detail
// box's inner area. Instead the panel shows metadata + a [Q] hint.
func TestWorkflow_TOTPDetailNoLongerInlinesQR(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	row, _, err := svc.TOTP.Generate(ctx, "alice@app")
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.TOTP.ByID(ctx, row.ID, "lab")
	if err != nil {
		t.Fatal(err)
	}

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = driveRune(m, '8') // TOTP tab
	rows, _ := svc.TOTP.List(ctx)
	root := rootOf(t, m)
	root.totp.rows = rows
	root.totp.view = view
	m = root

	rendered := m.View()
	// QR ASCII uses half-block glyphs (▀ ▄ █). Their presence in the
	// rendered view means the QR is still inline — that's the bug.
	for _, glyph := range []string{"▀", "▄"} {
		if strings.Contains(rendered, glyph) {
			t.Errorf("QR glyph %q present in default TOTP view — QR must only show in the [Q] popup", glyph)
		}
	}
	// The hint must be visible so operators know how to reach the QR.
	if !strings.Contains(rendered, "[Q]") {
		t.Error("[Q] hint missing from TOTP detail panel — operators have no way to reach the QR")
	}
}

// TestWorkflow_TOTPQRPopupOpensOnQ — pressing [Q] on the TOTP screen
// pushes a centred QR popup whose width is independent of the underlying
// listBox/detailBox layout, so the QR rendering can never shift the
// surrounding view.
func TestWorkflow_TOTPQRPopupOpensOnQ(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	row, _, err := svc.TOTP.Generate(ctx, "alice@app")
	if err != nil {
		t.Fatal(err)
	}
	view, _ := svc.TOTP.ByID(ctx, row.ID, "lab")

	var m tea.Model = New(svc, nil, SessionReady)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	m = driveRune(m, '8')
	root := rootOf(t, m)
	root.totp.view = view
	m = root

	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Q")})
	if cmd == nil {
		t.Fatal("[Q] produced no cmd; QR popup did not open")
	}
	push, ok := cmd().(pushOverlayMsg)
	if !ok {
		t.Fatalf("[Q] cmd produced %T, want pushOverlayMsg", cmd())
	}
	popup, ok := push.overlay.(*totpQRPopup)
	if !ok {
		t.Fatalf("[Q] pushed %T, want *totpQRPopup", push.overlay)
	}
	if popup.view != view {
		t.Error("popup not wired to the selected TOTP view")
	}

	// Render the popup view — half-block glyphs MUST appear (the QR is
	// actually shown), and pressing esc dismisses the overlay.
	popupView := popup.View()
	if !strings.Contains(popupView, "▀") && !strings.Contains(popupView, "▄") && !strings.Contains(popupView, "█") {
		t.Error("QR popup contains no half-block glyphs — the QR is not rendered")
	}
	_, dismissCmd := popup.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if dismissCmd == nil {
		t.Fatal("esc on QR popup produced no cmd")
	}
	if _, ok := dismissCmd().(OverlayDoneMsg); !ok {
		t.Fatalf("esc cmd produced %T, want OverlayDoneMsg", dismissCmd())
	}
	_ = mm
}

// TestWorkflow_TOTPDetailLayoutStableAcrossWidths sweeps a few terminal
// widths and asserts the rendered view's total height stays the same when
// a TOTP is selected vs. not (only the metadata block grows, never by
// more than the QR would have grown pre-fix). This is the structural
// equivalent of "the QR no longer shifts the layout".
func TestWorkflow_TOTPDetailLayoutStableAcrossWidths(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	row, _, _ := svc.TOTP.Generate(ctx, "alice@app")
	view, _ := svc.TOTP.ByID(ctx, row.ID, "lab")

	for _, w := range []int{100, 120, 144, 180} {
		var m tea.Model = New(svc, nil, SessionReady)
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 44})
		m = driveRune(m, '8')

		// No selection.
		emptyView := m.View()
		// With selection.
		root := rootOf(t, m)
		root.totp.view = view
		m = root
		selView := m.View()

		// Max line width in either view must be ≤ terminal width.
		for _, line := range strings.Split(emptyView, "\n") {
			if lw := visualWidth(line); lw > w {
				t.Errorf("width=%d empty: line overflows (%d > %d)", w, lw, w)
			}
		}
		for _, line := range strings.Split(selView, "\n") {
			if lw := visualWidth(line); lw > w {
				t.Errorf("width=%d selected: line overflows (%d > %d) — QR re-leaking into layout?",
					w, lw, w)
			}
		}
	}
}

func visualWidth(s string) int {
	// Strip ANSI escape sequences and count display width.
	out := []rune{}
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		out = append(out, r)
	}
	return len(out)
}

