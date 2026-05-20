package service

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestTOTPCreateAndGet(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	kek := newKEK(t)
	audit := NewAuditService(s)

	// Make an Issuer + License (TOTPSecret requires a License FK).
	iss, _ := NewIssuerService(s, kek, audit).Generate(ctx, "lab", "k1", "op")
	lic, err := s.Client.License.Create().
		SetLicenseUUID("lic-uuid").
		SetSubject("alice").
		SetIssuerID(iss.ID).
		SetNotBefore(time.Now()).
		SetNotAfter(time.Now().Add(time.Hour)).
		SetPem([]byte("pem")).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	svc := NewTOTPService(s, kek)

	// Create via tx helper (simulates LicenseService.Issue path).
	tx, _ := s.Client.Tx(ctx)
	secret, err := svc.createForLicenseTx(ctx, tx, lic.ID, "alice@example.com")
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if len(secret) == 0 {
		t.Fatal("empty secret")
	}

	// Read back via Get.
	view, err := svc.Get(ctx, lic.ID, "lab")
	if err != nil {
		t.Fatal(err)
	}
	if view.Secret != secret {
		t.Fatal("Get returned different secret")
	}
	if view.OtpauthURI == "" || view.QRImageASCII == "" {
		t.Fatal("provisioning artefacts empty")
	}
	if len(view.QRImagePNG) < 100 || !bytes.HasPrefix(view.QRImagePNG, []byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Fatal("PNG not produced")
	}
}
