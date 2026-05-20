package revoke

import (
	"testing"
	"time"
)

func TestLookupEntryReturnsMetadata(t *testing.T) {
	now := time.Now().UTC()
	l := List{
		Revoked: []string{"lic-1", "lic-2"},
		Entries: []Entry{
			{ID: "lic-1", Reason: "refunded", RevokedAt: &now},
			{ID: "lic-2", Reason: "key-compromised"},
		},
	}
	e, ok := l.LookupEntry("lic-1")
	if !ok {
		t.Fatal("lic-1 not found")
	}
	if e.Reason != "refunded" || e.RevokedAt == nil || !e.RevokedAt.Equal(now) {
		t.Fatalf("got %+v", e)
	}
}

func TestLookupEntryAbsent(t *testing.T) {
	l := List{Revoked: []string{"lic-1"}}
	if _, ok := l.LookupEntry("lic-1"); ok {
		t.Fatal("Entries was empty — LookupEntry should return false")
	}
	if _, ok := l.LookupEntry("absent"); ok {
		t.Fatal("absent id returned ok")
	}
}

func TestEntriesSignVerifyRoundTrip(t *testing.T) {
	pub, priv := keypair(t)
	l := List{
		Version:    1,
		KeyID:      "k1",
		Sequence:   7,
		IssuedAt:   time.Now().UTC(),
		ExpiresAt:  time.Now().Add(time.Hour).UTC(),
		ServerTime: time.Now().UTC(),
		Revoked:    []string{"lic-x"},
		Entries:    []Entry{{ID: "lic-x", Reason: "fraud"}},
	}
	raw, err := Sign(l, priv)
	if err != nil {
		t.Fatal(err)
	}
	back, err := VerifyBytes(raw, pub, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Entries) != 1 || back.Entries[0].Reason != "fraud" {
		t.Fatalf("entries lost: %+v", back.Entries)
	}
}
