package tui

import (
	"strings"
	"testing"
	"time"
)

// t0 is the reference timestamp every seed builder uses; pinning it here
// makes the row payloads reproducible across test runs.
var t0 = time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)

// TestSeedLicensesMsg covers the licenses fixture: count, the long-PEM
// invariant on row 0 (so PEM-scroll tapes have content), and the standard
// row shape (subject + status + UUID set).
func TestSeedLicensesMsg(t *testing.T) {
	msg := SeedLicensesMsg(0, t0)
	if len(msg.Rows) != 0 {
		t.Errorf("n=0: rows=%d want 0", len(msg.Rows))
	}
	msg = SeedLicensesMsg(5, t0)
	if got := len(msg.Rows); got != 5 {
		t.Fatalf("n=5: rows=%d want 5", got)
	}
	if msg.Rows[0].Subject == "" {
		t.Error("row 0: subject empty")
	}
	if msg.Rows[0].LicenseUUID == "" {
		t.Error("row 0: LicenseUUID empty")
	}
	// Row 0 must carry the long PEM so the scroll-tape sees scroll motion.
	if !strings.HasPrefix(string(msg.Rows[0].Pem), "-----BEGIN MALDEV LICENSE") {
		t.Error("row 0: PEM missing BEGIN header")
	}
	if got := strings.Count(string(msg.Rows[0].Pem), "\n"); got < 100 {
		t.Errorf("row 0: PEM has %d lines, want ≥100", got)
	}
}

// TestSeedIssuersMsg checks the active-key invariant (row 0) + count + key
// uniqueness via distinct KeyIDs.
func TestSeedIssuersMsg(t *testing.T) {
	msg := SeedIssuersMsg(3, t0)
	if got := len(msg.Rows); got != 3 {
		t.Fatalf("rows=%d want 3", got)
	}
	if !msg.Rows[0].Active {
		t.Error("row 0 must be the active issuer")
	}
	for i := 1; i < len(msg.Rows); i++ {
		if msg.Rows[i].Active {
			t.Errorf("row %d: expected inactive (only row 0 is active)", i)
		}
	}
	// KeyIDs must be unique so the issuers screen renders distinguishable rows.
	seen := map[string]bool{}
	for _, r := range msg.Rows {
		if seen[r.KeyID] {
			t.Errorf("duplicate KeyID %q", r.KeyID)
		}
		seen[r.KeyID] = true
	}
}

// TestSeedRecipientsMsg verifies the X25519 public-key payload is 32 bytes
// per row + non-zero (deterministic but distinct per index).
func TestSeedRecipientsMsg(t *testing.T) {
	msg := SeedRecipientsMsg(4, t0)
	if got := len(msg.Rows); got != 4 {
		t.Fatalf("rows=%d want 4", got)
	}
	for i, r := range msg.Rows {
		if len(r.PublicKey) != 32 {
			t.Errorf("row %d: PublicKey len=%d want 32", i, len(r.PublicKey))
		}
	}
	// Row 0 and row 1 must have different bytes (deterministic per-index seed).
	if string(msg.Rows[0].PublicKey) == string(msg.Rows[1].PublicKey) {
		t.Error("rows 0 and 1 have identical PublicKey bytes — seed not per-index")
	}
}

// TestSeedIdentitiesMsg checks SHA256 hex length (64) and uniqueness.
func TestSeedIdentitiesMsg(t *testing.T) {
	msg := SeedIdentitiesMsg(3, t0)
	if got := len(msg.Rows); got != 3 {
		t.Fatalf("rows=%d want 3", got)
	}
	for i, r := range msg.Rows {
		if len(r.Sha256) != 64 {
			t.Errorf("row %d: Sha256 len=%d want 64 (hex of 32-byte digest)", i, len(r.Sha256))
		}
	}
}

// TestSeedRevocationMsg checks the revocation rows carry a non-empty
// Reason + RevokedBy (so the detail-panel rendering has values to display).
func TestSeedRevocationMsg(t *testing.T) {
	msg := SeedRevocationMsg(3, t0)
	if got := len(msg.Rows); got != 3 {
		t.Fatalf("rows=%d want 3", got)
	}
	for i, r := range msg.Rows {
		if r.Reason == "" {
			t.Errorf("row %d: Reason empty", i)
		}
		if r.RevokedBy == "" {
			t.Errorf("row %d: RevokedBy empty", i)
		}
	}
}

// TestSeedAuditMsg checks Kind rotation covers all common values (so the
// audit-filter chips have rows to filter for every category).
func TestSeedAuditMsg(t *testing.T) {
	msg := SeedAuditMsg(12, t0)
	if got := len(msg.Rows); got != 12 {
		t.Fatalf("rows=%d want 12", got)
	}
	hasLicense, hasIssuer, hasRecipient := false, false, false
	for _, r := range msg.Rows {
		switch {
		case strings.HasPrefix(r.Kind, "license."):
			hasLicense = true
		case strings.HasPrefix(r.Kind, "issuer."):
			hasIssuer = true
		case strings.HasPrefix(r.Kind, "recipient."):
			hasRecipient = true
		}
	}
	if !hasLicense || !hasIssuer || !hasRecipient {
		t.Errorf("Kind coverage: license=%v issuer=%v recipient=%v (all 3 must be present in n=12)",
			hasLicense, hasIssuer, hasRecipient)
	}
}

// TestSeedTOTPMsg checks the secret rows carry a non-empty account label
// (so the TOTP screen has something to render in the LABEL column).
func TestSeedTOTPMsg(t *testing.T) {
	msg := SeedTOTPMsg(3, t0)
	if got := len(msg.Rows); got != 3 {
		t.Fatalf("rows=%d want 3", got)
	}
	for i, r := range msg.Rows {
		if r.AccountLabel == "" {
			t.Errorf("row %d: AccountLabel empty", i)
		}
	}
}
