package service

import (
	"bytes"
	"context"
	"testing"

	"github.com/oioio-space/maldev/license/seal"
)

func TestRecipientGenerateRoundTrip(t *testing.T) {
	s := newTestStore(t)
	kek := newKEK(t)
	svc := NewRecipientService(s, kek, NewAuditService(s))
	ctx := context.Background()

	row, err := svc.Generate(ctx, "alice-prod", "operator")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := svc.ExportPublic(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := svc.PrivateKey(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Roundtrip seal/open to validate keypair integrity.
	sealed, err := seal.Seal(pub, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := seal.Open(priv, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(opened, []byte("hello")) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestRecipientImportRejectsBadLen(t *testing.T) {
	s := newTestStore(t)
	svc := NewRecipientService(s, newKEK(t), NewAuditService(s))
	if _, err := svc.Import(context.Background(), "x", make([]byte, 31), make([]byte, 32), "op"); err == nil {
		t.Fatal("bad pub len accepted")
	}
}

func TestRecipientDelete(t *testing.T) {
	s := newTestStore(t)
	svc := NewRecipientService(s, newKEK(t), NewAuditService(s))
	ctx := context.Background()
	row, _ := svc.Generate(ctx, "x", "op")
	if err := svc.Delete(ctx, row.ID, "op"); err != nil {
		t.Fatal(err)
	}
	rows, _ := svc.List(ctx)
	if len(rows) != 0 {
		t.Fatalf("after delete, list has %d", len(rows))
	}
}
