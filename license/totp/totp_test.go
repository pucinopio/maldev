package totp

import (
	"bytes"
	"encoding/base32"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewSecretBase32(t *testing.T) {
	s, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 32 { // 20 bytes → 32 base32 chars without padding
		t.Fatalf("len=%d, want 32", len(s))
	}
	if _, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s); err != nil {
		t.Fatalf("not valid base32: %v", err)
	}
}

// Reference vector from RFC 6238 Appendix B (SHA1, T=59).
// Secret = "12345678901234567890" → base32 "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ".
func TestCodeRFC6238Vector(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).
		EncodeToString([]byte("12345678901234567890"))
	got, err := Code(secret, time.Unix(59, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got != "287082" {
		t.Fatalf("got %s want 287082", got)
	}
}

func TestVerifySkewWindow(t *testing.T) {
	s, _ := NewSecret()
	now := time.Now()
	current, _ := Code(s, now)
	prev, _ := Code(s, now.Add(-30*time.Second))

	if !Verify(s, current, 0) {
		t.Fatal("current code rejected at skew=0")
	}
	if !Verify(s, prev, 1) {
		t.Fatal("prev-window code rejected at skew=1")
	}
	if Verify(s, prev, 0) {
		t.Fatal("prev-window code accepted at skew=0")
	}
	if Verify(s, "000000", 5) && current != "000000" {
		t.Fatal("garbage code accepted")
	}
}

func TestVerifyRejectsWrongLength(t *testing.T) {
	s, _ := NewSecret()
	if Verify(s, "12345", 1) {
		t.Fatal("5-digit code accepted")
	}
	if Verify(s, "1234567", 1) {
		t.Fatal("7-digit code accepted")
	}
}

func TestURIFormat(t *testing.T) {
	uri := URI("ABCD", "alice@example.com", "rshell")
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Fatalf("bad URI: %s", uri)
	}
	for _, want := range []string{"secret=ABCD", "issuer=rshell", "algorithm=SHA1", "digits=6", "period=30"} {
		if !strings.Contains(uri, want) {
			t.Fatalf("URI missing %q: %s", want, uri)
		}
	}
}

func TestQRImagePNG(t *testing.T) {
	s, _ := NewSecret()
	png, err := QRImagePNG(s, "alice", "rshell", 256)
	if err != nil {
		t.Fatal(err)
	}
	// PNG magic bytes.
	if !bytes.HasPrefix(png, []byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Fatal("output is not a PNG")
	}
	if len(png) < 100 {
		t.Fatalf("PNG too small: %d bytes", len(png))
	}
}

func TestQRImageASCII(t *testing.T) {
	s, _ := NewSecret()
	ascii, err := QRImageASCII(s, "alice", "rshell")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ascii, "██") {
		t.Fatalf("ASCII output has no QR cells: %q", ascii[:min(80, len(ascii))])
	}
}

func TestWriteQRImagePNG(t *testing.T) {
	s, _ := NewSecret()
	p := filepath.Join(t.TempDir(), "qr.png")
	if err := WriteQRImagePNG(p, s, "alice", "rshell", 128); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() < 100 {
		t.Fatalf("file too small: %d bytes", st.Size())
	}
}

func TestDecodeSecretTolerantOfSpaces(t *testing.T) {
	s, _ := NewSecret()
	spaced := s[:8] + " " + s[8:16] + " " + s[16:]
	if _, err := Code(spaced, time.Now()); err != nil {
		t.Fatalf("Code rejected spaced secret: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
