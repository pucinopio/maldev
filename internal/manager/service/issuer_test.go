package service

import (
	"context"
	"crypto/ed25519"
	"testing"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/license"
)

func newKEK(t *testing.T) *crypto.KEK {
	t.Helper()
	return crypto.DeriveFromPassphrase("test", [16]byte{1})
}

func TestIssuerGenerateAndActivate(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	kek := newKEK(t)
	audit := NewAuditService(s)
	svc := NewIssuerService(s, kek, audit)

	iss, err := svc.Generate(ctx, "Lab EU", "k-1", "operator")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetActive(ctx, iss.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	active, err := svc.Active(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if active.ID != iss.ID {
		t.Fatalf("active=%s want %s", active.ID, iss.ID)
	}

	priv, err := svc.PrivateKey(ctx, iss.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Fatalf("priv len=%d", len(priv))
	}

	pemBytes, err := svc.ExportPublic(ctx, iss.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pemBytes) < 50 {
		t.Fatalf("PEM too small: %s", pemBytes)
	}

	// Audit should have 2 events: generate + set_active.
	rows, _ := audit.List(ctx, 10)
	if len(rows) != 2 {
		t.Fatalf("audit count=%d want 2", len(rows))
	}
}

func TestIssuerSetActiveDeactivatesOthers(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	kek := newKEK(t)
	svc := NewIssuerService(s, kek, NewAuditService(s))

	first, err := svc.Generate(ctx, "First", "k-first", "op")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Generate(ctx, "Second", "k-second", "op")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.SetActive(ctx, first.ID, "op"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetActive(ctx, second.ID, "op"); err != nil {
		t.Fatal(err)
	}

	active, err := svc.Active(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if active.ID != second.ID {
		t.Fatalf("active=%s want %s", active.ID, second.ID)
	}

	// first must now be inactive.
	firstRow, err := svc.Get(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if firstRow.Active {
		t.Fatal("first issuer should be inactive after second was activated")
	}
}

// TestIssuerExportPrivate guards the round-trip: generate → ExportPrivate →
// ParsePrivateKey must recover bytes identical to the original signing key.
// Symmetrical to TestIssuerImport which goes the other direction.
func TestIssuerExportPrivate(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	kek := newKEK(t)
	svc := NewIssuerService(s, kek, NewAuditService(s))

	iss, err := svc.Generate(ctx, "exp-test", "k-exp", "op")
	if err != nil {
		t.Fatal(err)
	}
	pem, err := svc.ExportPrivate(ctx, iss.ID)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := license.ParsePrivateKey(pem)
	if err != nil {
		t.Fatalf("re-parse exported PEM: %v", err)
	}
	// The in-DB private key (after KEK unwrap) must match what ExportPrivate
	// rendered as PEM.
	stored, err := svc.PrivateKey(ctx, iss.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(priv) != len(stored) {
		t.Fatalf("len mismatch: exported=%d stored=%d", len(priv), len(stored))
	}
	for i := range priv {
		if priv[i] != stored[i] {
			t.Fatalf("byte %d differs: exported=%x stored=%x", i, priv[i], stored[i])
		}
	}
}

func TestIssuerImport(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	kek := newKEK(t)
	svc := NewIssuerService(s, kek, NewAuditService(s))

	_, priv, err := license.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	privPEM, err := license.MarshalPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}

	iss, err := svc.Import(ctx, "Imported", "k-imp", privPEM, "op")
	if err != nil {
		t.Fatal(err)
	}

	// Round-trip: unwrap and verify the private key bytes match.
	got, err := svc.PrivateKey(ctx, iss.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 64 {
		t.Fatalf("imported priv len=%d", len(got))
	}
	for i, b := range []byte(priv) {
		if got[i] != b {
			t.Fatalf("priv byte %d mismatch", i)
		}
	}
}

func TestIssuerList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	kek := newKEK(t)
	svc := NewIssuerService(s, kek, NewAuditService(s))

	for i, name := range []string{"A", "B", "C"} {
		keyID := "k-" + name
		if _, err := svc.Generate(ctx, name, keyID, "op"); err != nil {
			t.Fatalf("generate %d: %v", i, err)
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
