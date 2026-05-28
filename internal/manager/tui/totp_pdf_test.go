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

// TestExportTOTPPDF_UTF8RoundTrip is the regression guard for the operator-
// reported "un caractère A avec accent qui apparait" in the TOTP PDF.
// Root cause: gofpdf's built-in fonts (Helvetica / Courier) render bytes
// in CP1252. Any UTF-8 multi-byte character (·, é, à, …) was interpreted
// by the PDF reader as two separate Latin-1 glyphs — the "Â·" mojibake
// the operator saw. Fix: route every Cell/MultiCell text through
// pdf.UnicodeTranslatorFromDescriptor("") which converts UTF-8 → CP1252
// bytes the fonts can render. This test renders a PDF with a UTF-8 input
// containing "é" and inspects the raw bytes for the corresponding UTF-8
// double-byte sequence (\xC3\xA9) which MUST be absent post-fix.
func TestExportTOTPPDF_UTF8RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.pdf")
	view := &service.TOTPSecretView{
		AccountLabel: "café@example.com",
		Secret:       "JBSWY3DPEHPK3PXP",
		OtpauthURI:   "otpauth://totp/lab:café@example.com?secret=JBSWY3DPEHPK3PXP",
	}
	if err := tui.ExportTOTPPDFForTest(view, path); err != nil {
		t.Fatalf("exportTOTPPDF: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// "é" is \xC3\xA9 in UTF-8; "·" is \xC2\xB7. Both used to leak through
	// to the PDF content stream pre-fix. They must not appear now.
	if contains2(data, 0xC3, 0xA9) {
		t.Error("PDF contains raw UTF-8 bytes for 'é' (\xC3\xA9) — translator skipped, will render as 'Ã©'")
	}
	if contains2(data, 0xC2, 0xB7) {
		t.Error("PDF contains raw UTF-8 bytes for '·' (\xC2\xB7) — translator skipped, will render as 'Â·'")
	}
}

// contains2 reports whether s contains the two-byte sequence [a, b].
func contains2(s []byte, a, b byte) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == a && s[i+1] == b {
			return true
		}
	}
	return false
}
