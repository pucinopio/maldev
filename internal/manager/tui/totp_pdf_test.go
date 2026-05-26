package tui_test

// totp_pdf_test.go — guard tests for exportTOTPPDF (D-S44).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/tui"
)

// fixtureTOTPView returns a minimal TOTPSecretView for PDF rendering tests.
// QRImagePNG is nil here (not all test environments have a QR library wired);
// exportTOTPPDF handles nil QRImagePNG gracefully by skipping the image block.
func fixtureTOTPView() *service.TOTPSecretView {
	return &service.TOTPSecretView{
		Secret:       "JBSWY3DPEHPK3PXP",
		AccountLabel: "alice@example.com",
		OtpauthURI:   "otpauth://totp/alice%40example.com?secret=JBSWY3DPEHPK3PXP&issuer=license-manager",
		QRImageASCII: "█▀▀▀█ test █▀▀▀█",
		QRImagePNG:   nil, // omitted — avoids QR-library dep in unit tests
	}
}

// TestExportTOTPPDF_WritesPDFFile verifies that exportTOTPPDF writes a file
// that starts with the PDF magic bytes and is larger than 1 KB.
func TestExportTOTPPDF_WritesPDFFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "totp.pdf")

	if err := tui.ExportTOTPPDFForTest(fixtureTOTPView(), path); err != nil {
		t.Fatalf("exportTOTPPDF returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("PDF file not found at %s: %v", path, err)
	}
	if len(data) < 1024 {
		t.Errorf("PDF too small: %d bytes, want > 1024", len(data))
	}
	if !strings.HasPrefix(string(data[:5]), "%PDF-") {
		t.Errorf("file does not start with %%PDF-, got: %q", string(data[:8]))
	}
}

// TestExportTOTPPDF_EnsuresExtension verifies that ensureExtension(".pdf") is
// applied before the write so files without the extension still get it.
func TestExportTOTPPDF_EnsuresExtension(t *testing.T) {
	dir := t.TempDir()
	// Deliberately omit the .pdf suffix — the handler must append it.
	rawPath := filepath.Join(dir, "totp_secret")

	// Simulate what handleTOTPInputResult does: apply ensureExtension then call exportTOTPPDF.
	path := tui.EnsureExtensionForTest(rawPath, ".pdf")
	if err := tui.ExportTOTPPDFForTest(fixtureTOTPView(), path); err != nil {
		t.Fatalf("exportTOTPPDF returned error: %v", err)
	}

	if _, err := os.Stat(rawPath + ".pdf"); err != nil {
		t.Errorf("expected file at %s.pdf, stat failed: %v", rawPath, err)
	}
}

// TestLive_TOTPExportPDF_PKeyPushesOverlay verifies that pressing [P] on the
// TOTP screen (when a view is loaded) pushes an input overlay with the
// OverlayIDTOTPExportPDF ID.
func TestLive_TOTPExportPDF_PKeyPushesOverlay(t *testing.T) {
	m := tui.NewTOTPModelForTest(nil)

	// Inject a non-nil view so the [P] guard passes.
	m = tui.SetTOTPViewForTest(m, fixtureTOTPView())

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if cmd == nil {
		t.Fatal("[P] key with non-nil view must return a non-nil cmd")
	}
	push, ok := tui.AsPushOverlay(cmd())
	if !ok {
		t.Fatalf("cmd() did not return a pushOverlayMsg")
	}
	view := push.Overlay.View()
	// The overlay title contains "PDF" to distinguish from the PNG overlay.
	if !strings.Contains(view, "PDF") {
		t.Errorf("[P] overlay view should contain 'PDF', got:\n%s", view)
	}
}

// TestLive_TOTPExportPDF_PKeyNoopWhenNoView verifies that [P] is a no-op
// (returns nil cmd) when no TOTP secret is selected.
func TestLive_TOTPExportPDF_PKeyNoopWhenNoView(t *testing.T) {
	m := tui.NewTOTPModelForTest(nil)
	// view is nil by default.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if cmd != nil {
		t.Error("[P] with nil view must return nil cmd")
	}
}
