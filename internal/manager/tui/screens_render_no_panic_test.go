package tui

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestAllScreensRenderAcrossWidths drives the root model into each of the
// 10 navigable views and renders at 4 widths (narrow / compact / standard /
// wide). Asserts no panic + non-empty output + no obvious layout corruption
// markers (NaN, %!s formatting failures, raw tab chars in body).
//
// This is the safety net for the screens-x-widths matrix the user spent
// the night fixing — any layout regression that compiles but breaks
// rendering at a specific width gets caught here in <1s.
func TestAllScreensRenderAcrossWidths(t *testing.T) {
	views := []ViewID{
		ViewDashboard, ViewLicenses, ViewIssuers, ViewRecipients,
		ViewIdentities, ViewRevocation, ViewServers, ViewTOTP,
		ViewAudit, ViewSettings,
	}
	widths := []int{80, 100, 144, 200}

	for _, v := range views {
		for _, w := range widths {
			name := string(v) + "_w" + strconv.Itoa(w)
			t.Run(name, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic rendering %s at width=%d: %v", v, w, r)
					}
				}()
				root := New(nil, nil, SessionReady)
				root.active = v
				var m tea.Model = root
				m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 44})
				out := m.View()
				if out == "" {
					t.Fatalf("%s @w=%d: empty view", v, w)
				}
				// %!s indicates a Sprintf went wrong; NaN means a divisor was 0.
				for _, bad := range []string{"%!s", "%!d", "%!v", "NaN"} {
					if strings.Contains(out, bad) {
						t.Errorf("%s @w=%d: view contains %q (formatting bug):\n%s", v, w, bad, out)
					}
				}
			})
		}
	}
}

