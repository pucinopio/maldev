package service

import (
	"context"
	"crypto/ed25519"
	"strings"
	"testing"

	"github.com/google/uuid"

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

// TestIssuerDelete verifies the new hard-delete + the safety guard that
// refuses removal while licences still reference the issuer.
func TestIssuerDelete(t *testing.T) {
	t.Run("deletes orphan issuer", func(t *testing.T) {
		ctx := context.Background()
		s := newTestStore(t)
		kek := newKEK(t)
		audit := NewAuditService(s)
		svc := NewIssuerService(s, kek, audit)
		iss, _ := svc.Generate(ctx, "Lab", "k-1", "op")
		if err := svc.Delete(ctx, iss.ID, "op"); err != nil {
			t.Fatalf("Delete orphan: %v", err)
		}
		if _, err := svc.Get(ctx, iss.ID); err == nil {
			t.Fatal("Issuer row still present after Delete")
		}
	})

	t.Run("refuses while licences reference issuer", func(t *testing.T) {
		lic, issuer, _, _, _, ctx := setupLicSvc(t)
		iss, _ := issuer.Generate(ctx, "Lab", "k-1", "op")
		_, err := lic.Issue(ctx, IssueRequest{
			IssuerID: iss.ID, Subject: "alice", Actor: "op",
		})
		if err != nil {
			t.Fatal(err)
		}
		err = issuer.Delete(ctx, iss.ID, "op")
		if err == nil {
			t.Fatal("expected refusal, got nil")
		}
		// Guard error message includes the count for the operator.
		if !strings.Contains(err.Error(), "1 licence") {
			t.Fatalf("error message missing licence count: %v", err)
		}
	})

	t.Run("missing id errors", func(t *testing.T) {
		ctx := context.Background()
		s := newTestStore(t)
		svc := NewIssuerService(s, newKEK(t), NewAuditService(s))
		if err := svc.Delete(ctx, uuid.New(), "op"); err == nil {
			t.Fatal("expected error for missing id")
		}
	})
}
