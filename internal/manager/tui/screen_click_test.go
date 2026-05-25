package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// TestClickMapping_ListScreens guards against off-by-N row-click regressions
// (like the user-reported Issuers bug fixed by 726eaf8). For each list
// screen, it renders the screen at 144×44, finds the actual Y of the table
// header row in the rendered output, and asserts that the model's
// `titleHints.y + 1` (== the screen's computed headerY) matches.
//
// If this fails, the screen's click handler will dispatch a row-select for
// the wrong row — or no row at all, depending on which direction it drifted.
func TestClickMapping_ListScreens(t *testing.T) {
	cases := []struct {
		name     string
		setup    func() (render string, gotHeaderY int)
		colMarker string // a substring that uniquely identifies the table header row
	}{
		{
			name: "issuers",
			setup: func() (string, int) {
				m := newIssuersModel(nil)
				return renderListScreen(t, ViewIssuers, &m), m.titleHints.y + 1
			},
			colMarker: "KEYID",
		},
		{
			name: "recipients",
			setup: func() (string, int) {
				m := newRecipientsModel(nil)
				return renderListScreen(t, ViewRecipients, &m), m.titleHints.y + 1
			},
			colMarker: "KEYID",
		},
		{
			name: "identities",
			setup: func() (string, int) {
				m := newIdentitiesModel(nil)
				return renderListScreen(t, ViewIdentities, &m), m.titleHints.y + 1
			},
			colMarker: "NAME",
		},
		{
			name: "totp",
			setup: func() (string, int) {
				m := newTOTPModel(nil)
				return renderListScreen(t, ViewTOTP, &m), m.titleHints.y + 1
			},
			colMarker: "LABEL",
		},
		{
			name: "revocation",
			setup: func() (string, int) {
				m := newRevocationModel(nil)
				return renderListScreen(t, ViewRevocation, &m), m.titleHints.y + 1
			},
			colMarker: "LICENSE",
		},
		{
			name: "licenses",
			setup: func() (string, int) {
				m := newLicensesModel(nil)
				return renderListScreen(t, ViewLicenses, &m), m.titleHints.y + 1
			},
			colMarker: "SUBJECT",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendered, computedHeaderY := tc.setup()
			actualHeaderY := findLineY(rendered, tc.colMarker)
			if actualHeaderY < 0 {
				t.Fatalf("%s: could not find table header marker %q in render", tc.name, tc.colMarker)
			}
			if computedHeaderY != actualHeaderY {
				t.Errorf("%s: click handler thinks header at Y=%d, render shows it at Y=%d (off-by-%d) — first data row click will be miscounted",
					tc.name, computedHeaderY, actualHeaderY, computedHeaderY-actualHeaderY)
			}
		})
	}
}

// renderListScreen wires the model into a rootModel, sends WindowSizeMsg,
// switches to the target view, and returns the full screen render. The
// caller passes a pointer to the typed sub-model so titleHints stays
// observable after Update.
func renderListScreen(t *testing.T, view ViewID, sub any) string {
	t.Helper()
	root := New(&service.Services{}, nil, SessionReady)
	root.active = view
	// Replace the matching sub-model with the caller-provided pointer's value
	// so titleHints is the SAME *titleHintRow we'll inspect later.
	switch v := sub.(type) {
	case *issuersModel:
		root.issuers = *v
	case *recipientsModel:
		root.recipients = *v
	case *identitiesModel:
		root.identities = *v
	case *totpModel:
		root.totp = *v
	case *revocationModel:
		root.revocation = *v
	case *licensesModel:
		root.licenses = *v
	default:
		t.Fatalf("unknown sub-model type %T", sub)
	}
	var m tea.Model = root
	m, _ = m.Update(tea.WindowSizeMsg{Width: 144, Height: 44})
	return m.View()
}

// findLineY returns the 0-based Y of the first line containing marker, or -1.
func findLineY(rendered, marker string) int {
	for i, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, marker) {
			return i
		}
	}
	return -1
}

// TestClickMapping_RowSelectDispatches calls OnClick on the expected first
// data-row Y for every list screen and confirms it produces a
// tableSelectRowMsg with row=0. Belt-and-braces guard: even if the Y math
// is right (TestClickMapping_ListScreens), this proves the dispatch chain
// is wired through.
func TestClickMapping_RowSelectDispatches(t *testing.T) {
	// Build the model + render so titleHints.y is populated, then synthesise
	// a click on the row just below the table header and verify the cmd.
	check := func(t *testing.T, name string, onClick func(x, y int) tea.Cmd, headerY int) {
		t.Helper()
		firstRowY := headerY + 1
		cmd := onClick(10, firstRowY)
		if cmd == nil {
			t.Errorf("%s: OnClick at first-row Y=%d returned nil — row click won't reach the model", name, firstRowY)
			return
		}
		msg := cmd()
		ts, ok := msg.(tableSelectRowMsg)
		if !ok {
			t.Errorf("%s: OnClick at Y=%d produced %T, want tableSelectRowMsg", name, firstRowY, msg)
			return
		}
		if ts.row != 0 {
			t.Errorf("%s: OnClick at Y=%d selected row %d, want 0", name, firstRowY, ts.row)
		}
	}

	// Each entry: build a model, push 1 fake row so tableRowCmd's bounds
	// check passes, render, then call OnClick.
	t.Run("issuers", func(t *testing.T) {
		m := newIssuersModel(nil)
		_ = renderListScreen(t, ViewIssuers, &m)
		m.rows = []*ent.Issuer{{}} // single fake row so tableRowCmd's bounds check passes
		check(t, "issuers", func(x, y int) tea.Cmd { return m.OnClick(x, y, 144) }, m.titleHints.y+1)
	})
	t.Run("recipients", func(t *testing.T) {
		m := newRecipientsModel(nil)
		_ = renderListScreen(t, ViewRecipients, &m)
		m.rows = []*ent.RecipientKey{{}}
		check(t, "recipients", func(x, y int) tea.Cmd { return m.OnClick(x, y, 144) }, m.titleHints.y+1)
	})
	t.Run("identities", func(t *testing.T) {
		m := newIdentitiesModel(nil)
		_ = renderListScreen(t, ViewIdentities, &m)
		m.rows = []*ent.Identity{{}}
		check(t, "identities", func(x, y int) tea.Cmd { return m.OnClick(x, y, 144) }, m.titleHints.y+1)
	})
	t.Run("totp", func(t *testing.T) {
		m := newTOTPModel(nil)
		_ = renderListScreen(t, ViewTOTP, &m)
		m.rows = []*ent.TOTPSecret{{}}
		check(t, "totp", func(x, y int) tea.Cmd { return m.OnClick(x, y, 144) }, m.titleHints.y+1)
	})
}
