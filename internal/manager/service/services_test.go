package service

import (
	"context"
	"testing"
	"time"
)

func TestServicesEndToEnd(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	kek := newKEK(t)
	svc := New(s, kek)
	t.Cleanup(func() { _ = svc.Close() })

	// Generate an issuer + activate.
	iss, err := svc.Issuer.Generate(ctx, "Lab EU", "k1", "op")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Issuer.SetActive(ctx, iss.ID, "op"); err != nil {
		t.Fatal(err)
	}

	// Create an identity.
	id, err := svc.Identity.Create(ctx, "rshell-v1", "op")
	if err != nil {
		t.Fatal(err)
	}

	// Issue a licence with machine binding + identity pinning + features.
	out, err := svc.License.Issue(ctx, IssueRequest{
		IssuerID:   iss.ID,
		Subject:    "alice",
		NotAfter:   time.Now().Add(24 * time.Hour),
		IdentityID: &id.ID,
		Features:   []string{"export"},
		Bindings:   []BindingSpec{{Type: "machine", Values: []string{"machine-1"}}},
		Actor:      "op",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Revoke it.
	if err := svc.Revoke.Revoke(ctx, out.Row.ID, "leak", "op"); err != nil {
		t.Fatal(err)
	}

	// Publish CRL.
	signed, err := svc.Revoke.PublishSignedList(ctx, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(signed) < 100 {
		t.Fatalf("CRL too small: %d", len(signed))
	}

	// Audit trail should contain all the actions.
	events, _ := svc.Audit.List(ctx, 50)
	want := []string{
		"issuer.generate", "issuer.set_active",
		"identity.create", "license.issue", "license.revoke",
	}
	gotKinds := map[string]bool{}
	for _, e := range events {
		gotKinds[e.Kind] = true
	}
	for _, w := range want {
		if !gotKinds[w] {
			t.Fatalf("audit missing kind %q (got %v)", w, gotKinds)
		}
	}
}
