package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	licensekg "github.com/oioio-space/maldev/license"
	"github.com/oioio-space/maldev/license/totp"
)

func setupLicSvc(t *testing.T) (*LicenseService, *IssuerService, *IdentityService, *RecipientService, *TOTPService, context.Context) {
	t.Helper()
	ctx := context.Background()
	s := newTestStore(t)
	kek := newKEK(t)
	audit := NewAuditService(s)
	issuer := NewIssuerService(s, kek, audit)
	identity := NewIdentityService(s, audit)
	recipient := NewRecipientService(s, kek, audit)
	totpSvc := NewTOTPService(s, kek)
	lic := NewLicenseService(s, kek, audit, issuer, identity, recipient, totpSvc)
	return lic, issuer, identity, recipient, totpSvc, ctx
}

func totpComputeCode(secret string) (string, error) {
	return totp.Code(secret, time.Now())
}

func TestLicenseIssueMinimal(t *testing.T) {
	lic, issuer, _, _, _, ctx := setupLicSvc(t)
	iss, err := issuer.Generate(ctx, "lab", "k1", "op")
	if err != nil {
		t.Fatal(err)
	}
	out, err := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID,
		Subject:  "alice",
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "op",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.PEM) == 0 || out.Row.LicenseUUID == "" {
		t.Fatalf("bad output: %+v", out)
	}

	// Round-trip verify.
	pubBytes, err := issuer.ExportPublic(ctx, iss.ID)
	if err != nil {
		t.Fatal(err)
	}
	pub, kid, err := licensekg.ParsePublicKey(pubBytes)
	if err != nil {
		t.Fatal(err)
	}
	trusted := licensekg.Trusted{Keys: licensekg.SingleKey(kid, pub)}
	if _, err := licensekg.Verify(out.PEM, trusted); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
}

func TestLicenseIssueWithBindings(t *testing.T) {
	lic, issuer, _, _, _, ctx := setupLicSvc(t)
	iss, err := issuer.Generate(ctx, "lab", "k1", "op")
	if err != nil {
		t.Fatal(err)
	}

	out, err := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID,
		Subject:  "alice",
		Features: []string{"export", "api"},
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "op",
		Bindings: []BindingSpec{
			{Type: "machine", Values: []string{"machine-1", "machine-2"}},
			{Type: "password", Values: []string{"hunter2"}},
			{Type: "totp"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.TOTPs) != 1 {
		t.Fatalf("totps count=%d", len(out.TOTPs))
	}
	if out.TOTPs[0].Secret == "" || out.TOTPs[0].QRImageASCII == "" {
		t.Fatal("totp provisioning empty")
	}

	// Verify with all binding evidence.
	pubBytes, _ := issuer.ExportPublic(ctx, iss.ID)
	pub, kid, _ := licensekg.ParsePublicKey(pubBytes)
	trusted := licensekg.Trusted{Keys: licensekg.SingleKey(kid, pub)}
	code, err := totpComputeCode(out.TOTPs[0].Secret)
	if err != nil {
		t.Fatal(err)
	}
	v, err := licensekg.Verify(out.PEM, trusted,
		licensekg.WithMachineID([]byte("machine-1")),
		licensekg.WithPassword("hunter2"),
		licensekg.WithTOTPCode(code),
	)
	if err != nil {
		t.Fatalf("verify with bindings failed: %v", err)
	}
	if !v.HasFeature("export") {
		t.Fatal("export feature missing in verified license")
	}
}

func TestLicenseIssueSubjectRequired(t *testing.T) {
	lic, issuer, _, _, _, ctx := setupLicSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	_, err := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID,
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "op",
	})
	if err == nil {
		t.Fatal("expected error for missing subject")
	}
}

func TestLicenseListFilter(t *testing.T) {
	lic, issuer, _, _, _, ctx := setupLicSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	for _, sub := range []string{"alice", "bob", "carol"} {
		_, err := lic.Issue(ctx, IssueRequest{
			IssuerID: iss.ID, Subject: sub,
			NotAfter: time.Now().Add(24 * time.Hour),
			Actor:    "op",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	rows, err := lic.List(ctx, ListFilter{SubjectContain: "al"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Subject != "alice" {
		t.Fatalf("got %+v", rows)
	}
}

func TestLicenseListAll(t *testing.T) {
	lic, issuer, _, _, _, ctx := setupLicSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	for _, sub := range []string{"x1", "x2", "x3"} {
		_, err := lic.Issue(ctx, IssueRequest{
			IssuerID: iss.ID, Subject: sub,
			NotAfter: time.Now().Add(24 * time.Hour),
			Actor:    "op",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	rows, err := lic.List(ctx, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
}

func TestLicenseHashFile(t *testing.T) {
	lic, _, _, _, _, _ := setupLicSvc(t)
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := lic.HashFile(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	// sha256("hello world")
	want := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestLicenseHashFileProgress(t *testing.T) {
	lic, _, _, _, _, _ := setupLicSvc(t)
	p := filepath.Join(t.TempDir(), "f")
	data := make([]byte, 200*1024) // 200 KB — crosses the 64 KB read buffer
	for i := range data {
		data[i] = byte(i & 0xFF)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var calls int
	_, err := lic.HashFile(context.Background(), p, func(read, total int64) {
		calls++
		if total != int64(len(data)) {
			t.Errorf("total mismatch: got %d want %d", total, len(data))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls == 0 {
		t.Fatal("progress callback never called")
	}
}

func TestLicenseExportPEM(t *testing.T) {
	lic, issuer, _, _, _, ctx := setupLicSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	out, err := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID, Subject: "alice",
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "op",
	})
	if err != nil {
		t.Fatal(err)
	}
	pem, err := lic.ExportPEM(ctx, out.Row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pem) == 0 {
		t.Fatal("empty export")
	}
}

func TestLicenseExportBatch(t *testing.T) {
	lic, issuer, _, _, _, ctx := setupLicSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	out1, err := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID, Subject: "batch1",
		NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
	})
	if err != nil {
		t.Fatal(err)
	}
	out2, err := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID, Subject: "batch2",
		NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
	})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := lic.ExportBatch(ctx, []uuid.UUID{out1.Row.ID, out2.Row.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(archive) == 0 {
		t.Fatal("empty archive")
	}
}

func TestLicenseInspect(t *testing.T) {
	lic, issuer, _, _, _, ctx := setupLicSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	out, err := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID, Subject: "dave",
		Features: []string{"beta"},
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "op",
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := lic.Inspect(out.PEM)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Subject != "dave" {
		t.Fatalf("subject=%q", parsed.Subject)
	}
	if !parsed.HasFeature("beta") {
		t.Fatal("beta feature missing")
	}
}

func TestLicenseGetAndGetByUUID(t *testing.T) {
	lic, issuer, _, _, _, ctx := setupLicSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	out, err := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID, Subject: "eve",
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "op",
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err := lic.Get(ctx, out.Row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.Subject != "eve" {
		t.Fatalf("Get: subject=%q", row.Subject)
	}
	row2, err := lic.GetByUUID(ctx, out.Row.LicenseUUID)
	if err != nil {
		t.Fatal(err)
	}
	if row2.ID != row.ID {
		t.Fatalf("GetByUUID mismatch: %v vs %v", row2.ID, row.ID)
	}
}

// TestLicenseDelete covers the export→delete→re-import round-trip plus the
// cascade-clean of TOTPSecret + Revocation rows. Three sub-cases:
//   - delete an active licence and re-import its PEM without UUID conflict
//   - delete a licence whose binding produced a TOTPSecret row
//   - delete a revoked licence (Revocation row must be cascaded)
func TestLicenseDelete(t *testing.T) {
	t.Run("active round-trip", func(t *testing.T) {
		lic, issuer, _, _, _, ctx := setupLicSvc(t)
		iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
		out, err := lic.Issue(ctx, IssueRequest{
			IssuerID: iss.ID, Subject: "alice",
			NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
		})
		if err != nil {
			t.Fatal(err)
		}
		uuidStr := out.Row.LicenseUUID
		pem := out.PEM

		if err := lic.Delete(ctx, out.Row.ID, "op"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := lic.GetByUUID(ctx, uuidStr); err == nil {
			t.Fatal("row still queryable after Delete")
		}
		// Re-import the previously-exported PEM should now succeed.
		row, err := lic.Import(ctx, pem, "reimport", "op")
		if err != nil {
			t.Fatalf("re-import after Delete failed: %v", err)
		}
		if row.LicenseUUID != uuidStr {
			t.Fatalf("reimported UUID=%q want %q", row.LicenseUUID, uuidStr)
		}
	})

	t.Run("cascades totp secret", func(t *testing.T) {
		lic, issuer, _, _, _, ctx := setupLicSvc(t)
		iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
		out, err := lic.Issue(ctx, IssueRequest{
			IssuerID: iss.ID, Subject: "bob",
			NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
			Bindings: []BindingSpec{{Type: "totp"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		// Sanity: the issuance created one TOTPSecret row.
		n, _ := lic.store.Client.TOTPSecret.Query().Count(ctx)
		if n != 1 {
			t.Fatalf("totp row count before Delete=%d, want 1", n)
		}
		if err := lic.Delete(ctx, out.Row.ID, "op"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		n, _ = lic.store.Client.TOTPSecret.Query().Count(ctx)
		if n != 0 {
			t.Fatalf("totp row count after Delete=%d, want 0 (cascade)", n)
		}
	})

	t.Run("cascades revocation", func(t *testing.T) {
		lic, issuer, _, _, _, ctx := setupLicSvc(t)
		audit := NewAuditService(lic.store)
		revSvc := NewRevokeService(lic.store, audit, issuer)
		lic.SetRevoke(revSvc)

		iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
		out, err := lic.Issue(ctx, IssueRequest{
			IssuerID: iss.ID, Subject: "carol",
			NotAfter: time.Now().Add(24 * time.Hour), Actor: "op",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := revSvc.Revoke(ctx, out.Row.ID, "compromise", "op"); err != nil {
			t.Fatal(err)
		}
		if err := lic.Delete(ctx, out.Row.ID, "op"); err != nil {
			t.Fatalf("Delete revoked: %v", err)
		}
		// The revocation table should no longer reference the deleted licence.
		views, err := revSvc.ListRevoked(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range views {
			if v.LicenseID == out.Row.ID {
				t.Fatal("revocation row survived Delete")
			}
		}
	})

	t.Run("missing id errors", func(t *testing.T) {
		lic, _, _, _, _, ctx := setupLicSvc(t)
		if err := lic.Delete(ctx, uuid.New(), "op"); err == nil {
			t.Fatal("expected error deleting non-existent id")
		}
	})
}
