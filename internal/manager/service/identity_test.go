package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestIdentityCreateAndExport(t *testing.T) {
	s := newTestStore(t)
	audit := NewAuditService(s)
	svc := NewIdentityService(s, audit)
	ctx := context.Background()

	id, err := svc.Create(ctx, "rshell-v1", "operator")
	if err != nil {
		t.Fatal(err)
	}
	if len(id.Bytes) != 32 || id.Sha256 == "" {
		t.Fatalf("unexpected row: %+v", id)
	}
	out, err := svc.ExportBin(ctx, id.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 32 {
		t.Fatalf("len=%d", len(out))
	}
}

func TestIdentityImportRejectsBadLen(t *testing.T) {
	s := newTestStore(t)
	svc := NewIdentityService(s, NewAuditService(s))
	if _, err := svc.Import(context.Background(), "x", make([]byte, 31), "op"); err == nil {
		t.Fatal("31-byte import accepted")
	}
}

func TestIdentityImport(t *testing.T) {
	s := newTestStore(t)
	svc := NewIdentityService(s, NewAuditService(s))
	ctx := context.Background()

	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	id, err := svc.Import(ctx, "known-v1", raw, "op")
	if err != nil {
		t.Fatal(err)
	}
	if id.Sha256 == "" {
		t.Fatal("sha256 empty after import")
	}
	got, err := svc.ExportBin(ctx, id.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range raw {
		if got[i] != b {
			t.Fatalf("byte %d mismatch: got %x want %x", i, got[i], b)
		}
	}
}

func TestIdentityList(t *testing.T) {
	s := newTestStore(t)
	svc := NewIdentityService(s, NewAuditService(s))
	ctx := context.Background()

	for _, name := range []string{"a", "b", "c"} {
		if _, err := svc.Create(ctx, name, "op"); err != nil {
			t.Fatal(err)
		}
	}
	all, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("list count=%d want 3", len(all))
	}
}

func TestIdentityRegenerateRequiresConfirm(t *testing.T) {
	s := newTestStore(t)
	svc := NewIdentityService(s, NewAuditService(s))
	ctx := context.Background()
	id, _ := svc.Create(ctx, "x", "op")
	if err := svc.Regenerate(ctx, id.ID, false, "op"); !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("err=%v want ErrNotConfirmed", err)
	}
	if err := svc.Regenerate(ctx, id.ID, true, "op"); err != nil {
		t.Fatal(err)
	}
	updated, _ := svc.Get(ctx, id.ID)
	if updated.Sha256 == id.Sha256 {
		t.Fatal("sha256 did not change after regenerate")
	}
}

func TestIdentityDeleteRefusesIfUsed(t *testing.T) {
	s := newTestStore(t)
	kek := newKEK(t)
	issuerSvc := NewIssuerService(s, kek, NewAuditService(s))
	svc := NewIdentityService(s, NewAuditService(s))
	ctx := context.Background()

	iss, err := issuerSvc.Generate(ctx, "Test CA", "k-test", "op")
	if err != nil {
		t.Fatal(err)
	}

	id, _ := svc.Create(ctx, "x", "op")

	_, err = s.Client.License.Create().
		SetLicenseUUID("test-uuid").
		SetSubject("alice").
		SetIdentitySha256(id.Sha256).
		SetNotBefore(time.Now()).
		SetNotAfter(time.Now().Add(24 * time.Hour)).
		SetPem([]byte("pem")).
		SetIssuerID(iss.ID).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, id.ID, "op"); err == nil {
		t.Fatal("expected refusal when usage_count > 0")
	}
}

func TestIdentityDeleteSucceeds(t *testing.T) {
	s := newTestStore(t)
	svc := NewIdentityService(s, NewAuditService(s))
	ctx := context.Background()

	id, _ := svc.Create(ctx, "disposable", "op")
	if err := svc.Delete(ctx, id.ID, "op"); err != nil {
		t.Fatal(err)
	}
	all, _ := svc.List(ctx)
	if len(all) != 0 {
		t.Fatalf("expected 0 identities after delete, got %d", len(all))
	}
}
