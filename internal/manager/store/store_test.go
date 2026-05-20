package store

import (
	"context"
	"testing"
)

func TestNewInMemory(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.EnsureSingletons(ctx, []byte("salt-12345-6789-0"), []byte("canary")); err != nil {
		t.Fatal(err)
	}
	setting, err := s.Client.Setting.Get(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if string(setting.KekCanary) != "canary" {
		t.Fatalf("canary not persisted: %q", setting.KekCanary)
	}
}

func TestIssuerInsert(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	iss, err := s.Client.Issuer.Create().
		SetName("Lab EU").
		SetKeyID("k-test").
		SetPublicKey(make([]byte, 32)).
		SetEncryptedPriv([]byte("enc")).
		SetActive(true).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if iss.ID.String() == "" {
		t.Fatal("ID not populated")
	}
}
