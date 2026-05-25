package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/license/totp"
)

// TestTOTPQRFitsInMinDetailW guards the user-reported bug: minDetailW=36
// was below the QR's 53-cell width, so lipgloss soft-wrapped each QR row
// and the half-block grid rendered as garbage.
//
// We render a real QR for a 16-byte secret (the production size), then
// check every row fits inside minDetailW=58's content area (=58 - 4 for
// BoxStyle border+padding).
func TestTOTPQRFitsInMinDetailW(t *testing.T) {
	const minDetailW = 58
	const innerW = minDetailW - 4 // BoxStyle: 2 border + 2 padding

	secret := strings.Repeat("A", 26) // a typical 26-char base32 secret
	ascii, err := totp.QRImageASCIICompact(secret, "alice@example.com", "maldev")
	if err != nil {
		t.Fatalf("QRImageASCIICompact: %v", err)
	}
	if ascii == "" {
		t.Fatal("QR ASCII empty")
	}

	for i, row := range strings.Split(ascii, "\n") {
		if w := lipgloss.Width(row); w > innerW {
			t.Errorf("QR row %d width = %d, > innerW %d — minDetailW must grow", i, w, innerW)
		}
	}
}
