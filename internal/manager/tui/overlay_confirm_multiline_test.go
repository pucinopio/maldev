package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestConfirmOverlay_MultilineBodyClickHits is the regression guard for the
// operator-reported bug "les boutons du popup de suppression ne sont pas
// clicables". Before fix: the click handler hardcoded footer Y=7 which only
// matched a 1-line body. The new licence/issuer/revocation delete confirms
// use 2–4 line bodies, so the footer slid to Y=9–10 and every click missed.
//
// The fix routes the footer Y through View() which measures the rendered
// modal and exposes the actual button row.
func TestConfirmOverlay_MultilineBodyClickHits(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"1-line body (historical)", "ligne unique"},
		{"2-line body", "ligne 1\nligne 2"},
		{"4-line body (licence delete)",
			"Supprimer définitivement la licence \"alice\" ?\n" +
				"La ligne, sa révocation éventuelle et tout secret TOTP associé\n" +
				"seront effacés. L'audit conserve la trace de l'opération.\n" +
				"Le PEM exporté reste réimportable."},
		{"6-line body (safety margin)",
			"L1\nL2\nL3\nL4\nL5\nL6"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := newConfirmOverlay("test", "Title", tc.body, "OK", "Cancel", true)
			_ = o.View() // populate footerY
			if o.footerY == 0 {
				t.Fatal("View() did not populate footerY")
			}
			// Click the CONFIRM button (right half) at the computed footer Y.
			_, cmd := o.Update(tea.MouseMsg{
				Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
				X: 40, Y: o.footerY,
			})
			if cmd == nil {
				t.Fatalf("click at footer Y=%d produced no cmd", o.footerY)
			}
			done, ok := cmd().(OverlayDoneMsg)
			if !ok {
				t.Fatalf("cmd produced %T, want OverlayDoneMsg", cmd())
			}
			res, ok := done.Result.(ConfirmResultMsg)
			if !ok || !res.Confirm {
				t.Fatalf("expected Confirm=true, got %+v", done.Result)
			}
			// And the CANCEL button (left half) at the same Y.
			_, cmd = o.Update(tea.MouseMsg{
				Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
				X: 10, Y: o.footerY,
			})
			res = cmd().(OverlayDoneMsg).Result.(ConfirmResultMsg)
			if res.Confirm {
				t.Fatalf("expected Confirm=false from left half, got %+v", res)
			}
			// A click on a non-footer row must NOT dismiss.
			_, cmd = o.Update(tea.MouseMsg{
				Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
				X: 40, Y: o.footerY - 1,
			})
			if cmd != nil {
				t.Fatalf("click one row above footer should be no-op, got cmd")
			}
		})
	}
}
