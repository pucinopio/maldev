package service

import (
	"context"
	"testing"
	"time"

	licensekg "github.com/oioio-space/maldev/license"
	"github.com/oioio-space/maldev/license/revoke"
)

func setupRevokeSvc(t *testing.T) (*RevokeService, *IssuerService, *LicenseService, context.Context) {
	t.Helper()
	lic, issuer, _, _, _, ctx := setupLicSvc(t)
	rev := NewRevokeService(lic.store, lic.audit, issuer)
	return rev, issuer, lic, ctx
}

func TestRevokeBasic(t *testing.T) {
	rev, issuer, lic, ctx := setupRevokeSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	out, _ := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID, Subject: "alice",
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "op",
	})
	if err := rev.Revoke(ctx, out.Row.ID, "key compromised", "op"); err != nil {
		t.Fatal(err)
	}
	rows, err := rev.ListRevoked(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Reason != "key compromised" {
		t.Fatalf("got %+v", rows)
	}
}

func TestRevokeRequiresReason(t *testing.T) {
	rev, issuer, lic, ctx := setupRevokeSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	out, _ := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID, Subject: "alice",
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "op",
	})
	if err := rev.Revoke(ctx, out.Row.ID, "", "op"); err == nil {
		t.Fatal("empty reason accepted")
	}
}

func TestRevokeIdempotent(t *testing.T) {
	rev, issuer, lic, ctx := setupRevokeSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	out, _ := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID, Subject: "alice",
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "op",
	})
	if err := rev.Revoke(ctx, out.Row.ID, "x", "op"); err != nil {
		t.Fatal(err)
	}
	if err := rev.Revoke(ctx, out.Row.ID, "y", "op"); err != nil {
		t.Fatal("second revoke should be idempotent, got err")
	}
}

func TestUnrevoke(t *testing.T) {
	rev, issuer, lic, ctx := setupRevokeSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	out, _ := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID, Subject: "alice",
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "op",
	})
	_ = rev.Revoke(ctx, out.Row.ID, "x", "op")
	if err := rev.Unrevoke(ctx, out.Row.ID, "op"); err != nil {
		t.Fatal(err)
	}
	rows, _ := rev.ListRevoked(ctx)
	if len(rows) != 0 {
		t.Fatalf("after unrevoke, list has %d", len(rows))
	}
}

func TestPublishSignedListCache(t *testing.T) {
	rev, issuer, lic, ctx := setupRevokeSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	_ = issuer.SetActive(ctx, iss.ID, "op")
	out, _ := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID, Subject: "bob",
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "op",
	})

	// First call populates the cache.
	pem1, err := rev.PublishSignedList(ctx, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Second call (no mutation) must return the cached copy — same bytes.
	pem2, err := rev.PublishSignedList(ctx, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if string(pem1) != string(pem2) {
		t.Fatal("expected cache hit: PEMs differ on repeated call with no mutation")
	}

	// Revoke invalidates; next call must re-sign (different sequence → different PEM).
	if err := rev.Revoke(ctx, out.Row.ID, "cache-test", "op"); err != nil {
		t.Fatal(err)
	}
	pem3, err := rev.PublishSignedList(ctx, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if string(pem3) == string(pem1) {
		t.Fatal("expected cache miss after Revoke: PEM unchanged")
	}

	// Unrevoke also invalidates.
	if err := rev.Unrevoke(ctx, out.Row.ID, "op"); err != nil {
		t.Fatal(err)
	}
	pem4, err := rev.PublishSignedList(ctx, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if string(pem4) == string(pem3) {
		t.Fatal("expected cache miss after Unrevoke: PEM unchanged")
	}
}

func TestPublishSignedListRoundTrip(t *testing.T) {
	rev, issuer, lic, ctx := setupRevokeSvc(t)
	iss, _ := issuer.Generate(ctx, "lab", "k1", "op")
	_ = issuer.SetActive(ctx, iss.ID, "op")
	out, _ := lic.Issue(ctx, IssueRequest{
		IssuerID: iss.ID, Subject: "alice",
		NotAfter: time.Now().Add(24 * time.Hour),
		Actor:    "op",
	})
	_ = rev.Revoke(ctx, out.Row.ID, "leak", "op")

	signed, err := rev.PublishSignedList(ctx, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	pubBytes, _ := issuer.ExportPublic(ctx, iss.ID)
	pub, kid, _ := licensekg.ParsePublicKey(pubBytes)
	parsed, err := revoke.VerifyBytes(signed, pub, kid)
	if err != nil {
		t.Fatal(err)
	}
	licUUID := out.Row.LicenseUUID
	found := false
	for _, id := range parsed.Revoked {
		if id == licUUID {
			found = true
		}
	}
	if !found {
		t.Fatal("revoked licence not in published list")
	}
	if len(parsed.Entries) != 1 || parsed.Entries[0].Reason != "leak" {
		t.Fatalf("entries=%+v", parsed.Entries)
	}
}
