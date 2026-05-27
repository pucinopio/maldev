package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	licenseent "github.com/oioio-space/maldev/internal/manager/store/ent/license"
)

// ─────────────────────────────────────────────────────────────────────────────
// Seed*Msg builders — deterministic synthetic row sets for visual demo tapes
// (cmd/tui-gif), screenshot tools (cmd/tui-snap), and any test that needs a
// populated screen without spinning up a real service+DB.
//
// `n` is the row count; `t0` is the reference timestamp every row derives
// its dates from so the same (n, t0) always produces the same Msg payload.
// That determinism matters for GIF reproducibility — the encoder writes
// byte-identical output across runs, so diffs flag real visual changes.
// ─────────────────────────────────────────────────────────────────────────────

// SeedLicensesMsg returns a LicensesLoadedMsg with n synthetic licences.
// The first row carries a deterministic ~17 KB PEM so PEM-scroll tapes
// have something to actually scroll; subsequent rows have shorter PEMs.
func SeedLicensesMsg(n int, t0 time.Time) LicensesLoadedMsg {
	rows := make([]*ent.License, n)
	subjects := []string{
		"alice@research.example",
		"bob@research.example",
		"carol@prod.example",
		"dave@dev.example",
		"eve@audit.example",
		"frank@qa.example",
		"grace@stage.example",
		"henry@infra.example",
	}
	statuses := []string{"active", "active", "active", "expiring", "expired", "revoked", "active", "active"}
	for i := 0; i < n; i++ {
		var pem strings.Builder
		pem.WriteString("-----BEGIN MALDEV LICENSE v2-----\n")
		// First row gets ~200 lines; later rows get ~10. Keeps the demo GIF
		// small while still letting the PEM-scroll tape produce observable
		// motion when the cursor lands on row 0.
		lines := 10
		if i == 0 {
			lines = 200
		}
		for j := 0; j < lines; j++ {
			pem.WriteString(strings.Repeat("A", 64))
			pem.WriteString("\n")
		}
		pem.WriteString("-----END MALDEV LICENSE v2-----\n")

		rows[i] = &ent.License{
			ID:          uuid.New(),
			LicenseUUID: fmt.Sprintf("lic-%08d-%s", i, uuid.NewString()[:8]),
			Subject:     subjects[i%len(subjects)],
			IssuerName:  fmt.Sprintf("issuer-prod-2026Q%d", 1+(i%4)),
			Audience:    []string{"prod", fmt.Sprintf("region-%d", i%3)},
			Features:    []string{"audit", "totp"},
			Status:      licenseent.Status(statuses[i%len(statuses)]),
			NotBefore:   t0.Add(-time.Duration(i) * 24 * time.Hour),
			NotAfter:    t0.Add(time.Duration(30+i) * 24 * time.Hour),
			Pem:         []byte(pem.String()),
		}
	}
	return LicensesLoadedMsg{Rows: rows}
}

// SeedIssuersMsg returns an IssuersLoadedMsg with n synthetic Ed25519
// issuer keys; row 0 is the active key.
func SeedIssuersMsg(n int, t0 time.Time) IssuersLoadedMsg {
	rows := make([]*ent.Issuer, n)
	for i := 0; i < n; i++ {
		rows[i] = &ent.Issuer{
			ID:        uuid.New(),
			Name:      fmt.Sprintf("issuer-prod-2026Q%d", 1+(i%4)),
			KeyID:     fmt.Sprintf("key-%08x", 0xDEADBEEF+i),
			Active:    i == 0,
			CreatedAt: t0.Add(-time.Duration(i*7) * 24 * time.Hour),
		}
	}
	return IssuersLoadedMsg{Rows: rows}
}

// SeedRecipientsMsg returns a RecipientsLoadedMsg with n synthetic X25519
// recipient keys.
func SeedRecipientsMsg(n int, t0 time.Time) RecipientsLoadedMsg {
	rows := make([]*ent.RecipientKey, n)
	names := []string{"acme-corp", "globex", "initech", "umbrella", "tyrell"}
	for i := 0; i < n; i++ {
		key := make([]byte, 32)
		for j := range key {
			key[j] = byte((i*31 + j) & 0xff)
		}
		rows[i] = &ent.RecipientKey{
			ID:        uuid.New(),
			Name:      names[i%len(names)],
			PublicKey: key,
			CreatedAt: t0.Add(-time.Duration(i*3) * 24 * time.Hour),
		}
	}
	return RecipientsLoadedMsg{Rows: rows}
}

// SeedIdentitiesMsg returns an IdentitiesLoadedMsg with n synthetic
// identity.bin pins.
func SeedIdentitiesMsg(n int, t0 time.Time) IdentitiesLoadedMsg {
	rows := make([]*ent.Identity, n)
	for i := 0; i < n; i++ {
		rows[i] = &ent.Identity{
			ID:        uuid.New(),
			Name:      fmt.Sprintf("prod-binary-v%d", 1+i),
			Sha256:    fmt.Sprintf("%064x", i+1),
			CreatedAt: t0.Add(-time.Duration(i*2) * 24 * time.Hour),
		}
	}
	return IdentitiesLoadedMsg{Rows: rows}
}

// SeedRevocationMsg returns a RevocationLoadedMsg with n synthetic CRL
// entries.
func SeedRevocationMsg(n int, t0 time.Time) RevocationLoadedMsg {
	rows := make([]service.RevocationView, n)
	reasons := []string{
		"key compromise",
		"superseded — re-issued",
		"affiliation changed",
		"cessation of operation",
		"privilege withdrawn",
	}
	for i := 0; i < n; i++ {
		rows[i] = service.RevocationView{
			LicenseID:   uuid.New(),
			LicenseUUID: fmt.Sprintf("lic-revoked-%08x", i),
			Subject:     fmt.Sprintf("user-%d@example", i),
			KeyID:       fmt.Sprintf("key-%08x", 0xDEADBEEF+(i%3)),
			Reason:      reasons[i%len(reasons)],
			RevokedAt:   t0.Add(-time.Duration(i) * time.Hour),
			RevokedBy:   "operator",
		}
	}
	return RevocationLoadedMsg{Rows: rows}
}

// SeedAuditMsg returns an AuditLoadedMsg with n synthetic audit events
// rotated through the most common Kind values.
func SeedAuditMsg(n int, t0 time.Time) AuditLoadedMsg {
	rows := make([]*ent.AuditEvent, n)
	kinds := []string{
		"license.issue", "license.revoke", "license.reissue",
		"issuer.generate", "issuer.import", "issuer.set_active",
		"recipient.generate", "identity.create", "identity.regenerate",
		"server.start", "server.stop", "probe.enrolled",
	}
	for i := 0; i < n; i++ {
		rows[i] = &ent.AuditEvent{
			ID:         uuid.New(),
			Kind:       kinds[i%len(kinds)],
			Actor:      "operator",
			TargetKind: "License",
			TargetID:   fmt.Sprintf("lic-%08x", i),
			CreatedAt:  t0.Add(-time.Duration(i) * time.Minute),
			Payload:    map[string]any{"note": fmt.Sprintf("synthetic event #%d", i)},
		}
	}
	return AuditLoadedMsg{Rows: rows}
}

// SeedTOTPMsg returns a TOTPLoadedMsg with n synthetic TOTP secrets.
func SeedTOTPMsg(n int, t0 time.Time) TOTPLoadedMsg {
	rows := make([]*ent.TOTPSecret, n)
	for i := 0; i < n; i++ {
		rows[i] = &ent.TOTPSecret{
			ID:           uuid.New(),
			AccountLabel: fmt.Sprintf("user-%d@app", i),
			CreatedAt:    t0.Add(-time.Duration(i*2) * time.Hour),
		}
	}
	return TOTPLoadedMsg{Rows: rows}
}
