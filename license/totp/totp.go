// Package totp implements RFC 6238 time-based one-time passwords (TOTP) with
// helpers for QR-code provisioning (PNG and ASCII).
//
// Designed as a second-factor binding inside a maldev license: the issuer
// generates a secret, ships it to the operator via QR code (one-time visible),
// and requires the current 6-digit code at Verify time.
//
// Security note: the secret is stored in clear inside the license binding.
// Anyone with the license file CAN extract the secret and compute codes —
// this is a speed bump, not strong 2FA. Combine with BindPassword + binary
// pinning for layered defence. See docs/license/concepts.md.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	// SecretLength is the recommended TOTP secret length in bytes (RFC 6238
	// section 5.1). 20 bytes = 160 bits = SHA1 block-aligned.
	SecretLength = 20

	// Period is the time window length in seconds. 30 is universal across
	// Google Authenticator, Authy, 1Password, Yubico Authenticator, etc.
	Period = 30

	// Digits is the code length. RFC 6238 supports 6, 7, or 8; 6 is the
	// universal authenticator default.
	Digits = 6
)

// NewSecret generates a fresh base32-encoded TOTP secret suitable for sharing
// with an authenticator app. Uses crypto/rand.
func NewSecret() (string, error) {
	b := make([]byte, SecretLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("totp: read random: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// Code computes the TOTP code for secret at time t. The secret must be
// base32-encoded (the format NewSecret returns).
func Code(secret string, t time.Time) (string, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}
	counter := uint64(t.Unix() / Period)
	return hotp(key, counter), nil
}

// Verify checks that code matches the expected TOTP for secret at the current
// time, tolerating ±skewSteps periods to absorb clock drift. skewSteps=1 is
// the recommended default (allows the previous and next 30-second windows).
func Verify(secret, code string, skewSteps int) bool {
	if skewSteps < 0 {
		skewSteps = 0
	}
	key, err := decodeSecret(secret)
	if err != nil {
		return false
	}
	if len(code) != Digits {
		return false
	}
	now := time.Now().Unix() / Period
	for step := -skewSteps; step <= skewSteps; step++ {
		counter := uint64(int64(now) + int64(step))
		if constantTimeEqual(code, hotp(key, counter)) {
			return true
		}
	}
	return false
}

// URI returns the otpauth:// URI used by authenticator apps. account is the
// user-visible label (typically an email), issuer is the org name shown in
// the authenticator.
func URI(secret, account, issuer string) string {
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", fmt.Sprintf("%d", Digits))
	v.Set("period", fmt.Sprintf("%d", Period))
	label := url.PathEscape(issuer + ":" + account)
	return "otpauth://totp/" + label + "?" + v.Encode()
}

// QRImagePNG returns a PNG byte slice of the otpauth URI QR code, suitable
// for writing to a file or embedding in a one-time provisioning page.
// size is the desired image size in pixels (e.g. 256).
func QRImagePNG(secret, account, issuer string, size int) ([]byte, error) {
	if size <= 0 {
		size = 256
	}
	return qrcode.Encode(URI(secret, account, issuer), qrcode.Medium, size)
}

// QRImageASCII returns an ASCII representation of the QR code, suitable for
// printing in a terminal. Two characters per cell so the result reads as a
// roughly-square QR when displayed in a fixed-width font.
func QRImageASCII(secret, account, issuer string) (string, error) {
	q, err := qrcode.New(URI(secret, account, issuer), qrcode.Medium)
	if err != nil {
		return "", err
	}
	bm := q.Bitmap()
	var b strings.Builder
	for _, row := range bm {
		for _, on := range row {
			if on {
				b.WriteString("██")
			} else {
				b.WriteString("  ")
			}
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// QRImageASCIICompact returns a half-height QR rendering using Unicode
// half-block characters (▀ ▄ █  ). Each line encodes TWO QR rows so the
// result is roughly square in a fixed-width terminal cell (which is twice as
// tall as wide). Use this in TUIs where vertical space is tight; pair with
// QRImageASCII when you need the wider double-cell form for screenshots.
func QRImageASCIICompact(secret, account, issuer string) (string, error) {
	q, err := qrcode.New(URI(secret, account, issuer), qrcode.Medium)
	if err != nil {
		return "", err
	}
	bm := q.Bitmap()
	var b strings.Builder
	for y := 0; y < len(bm); y += 2 {
		for x := 0; x < len(bm[y]); x++ {
			top := bm[y][x]
			var bot bool
			if y+1 < len(bm) {
				bot = bm[y+1][x]
			}
			switch {
			case top && bot:
				b.WriteString("█")
			case top:
				b.WriteString("▀")
			case bot:
				b.WriteString("▄")
			default:
				b.WriteString(" ")
			}
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// WriteQRImagePNG is a convenience wrapper that writes the PNG to path.
func WriteQRImagePNG(path, secret, account, issuer string, size int) error {
	png, err := QRImagePNG(secret, account, issuer, size)
	if err != nil {
		return err
	}
	return writeFile(path, png)
}

// hotp computes a single HMAC-SHA1-based one-time password per RFC 4226.
func hotp(key []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	// Dynamic truncation per RFC 4226 section 5.3.
	off := sum[len(sum)-1] & 0x0f
	val := (uint32(sum[off])&0x7f)<<24 |
		uint32(sum[off+1])<<16 |
		uint32(sum[off+2])<<8 |
		uint32(sum[off+3])
	mod := uint32(1)
	for i := 0; i < Digits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", Digits, val%mod)
}

func decodeSecret(secret string) ([]byte, error) {
	if secret == "" {
		return nil, errors.New("totp: empty secret")
	}
	// Authenticator apps tolerate spaces in pasted secrets — match that.
	secret = strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		key, err = base32.StdEncoding.DecodeString(secret)
		if err != nil {
			return nil, fmt.Errorf("totp: invalid base32 secret: %w", err)
		}
	}
	return key, nil
}

// constantTimeEqual compares two equal-length ASCII strings without leaking
// timing information through the early-exit pattern of native string ==.
func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
