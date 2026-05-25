package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	licenseent "github.com/oioio-space/maldev/internal/manager/store/ent/license"
)

// TestLicStatusPill_IsSingleLine guards the bug the user reported: the
// "active" pill rendered with a bordered Pill style (3 rows), which
// shifted the matching "status" label up by one row. The fix returns a
// flat one-line tag. If anyone reintroduces a bordered style, this test
// trips before it ships.
func TestLicStatusPill_IsSingleLine(t *testing.T) {
	cases := []struct {
		name  string
		input licenseent.Status
		want  string // substring (palette colour codes drop in no-TTY tests)
	}{
		{"active", licenseent.StatusActive, "● ACTIVE"},
		{"revoked", licenseent.StatusRevoked, "● REVOKED"},
		{"expired", licenseent.StatusExpired, "● EXPIRED"},
		{"superseded", licenseent.StatusSuperseded, "● SUPERSEDED"},
		{"expiring", licenseent.Status("expiring"), "● EXPIRING"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := licStatusPill(c.input)
			if h := lipgloss.Height(got); h != 1 {
				t.Errorf("%s pill height = %d, want 1 (kvRow alignment breaks otherwise)", c.name, h)
			}
			if strings.Contains(got, "\n") {
				t.Errorf("%s pill contains newline: %q", c.name, got)
			}
			if !strings.Contains(got, c.want) {
				t.Errorf("%s pill = %q, want substring %q", c.name, got, c.want)
			}
		})
	}
}
