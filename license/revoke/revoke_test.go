package revoke

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func keypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func TestListSignVerifyRoundTrip(t *testing.T) {
	pub, priv := keypair(t)
	l := List{
		Version:    1,
		KeyID:      "k1",
		Sequence:   42,
		IssuedAt:   time.Now().UTC(),
		ExpiresAt:  time.Now().Add(time.Hour).UTC(),
		ServerTime: time.Now().UTC(),
		Revoked:    []string{"lic-1", "lic-2"},
	}
	raw, err := Sign(l, priv)
	if err != nil {
		t.Fatal(err)
	}
	back, err := VerifyBytes(raw, pub, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if back.Sequence != 42 || len(back.Revoked) != 2 {
		t.Fatalf("roundtrip mismatch: %+v", back)
	}
}

func TestVerifyBytesRejectsTampered(t *testing.T) {
	pub, priv := keypair(t)
	raw, _ := Sign(List{Version: 1, KeyID: "k1", Sequence: 1, ExpiresAt: time.Now().Add(time.Hour)}, priv)
	blk, _ := pem.Decode(raw)
	blk.Bytes[5] ^= 0x01
	raw = pem.EncodeToMemory(blk)
	if _, err := VerifyBytes(raw, pub, "k1"); err == nil {
		t.Fatal("tampered list accepted")
	}
}

func TestIsRevoked(t *testing.T) {
	l := &List{Revoked: []string{"a", "b"}}
	if !l.IsRevoked("a") || l.IsRevoked("c") {
		t.Fatal("IsRevoked logic broken")
	}
}

func TestHTTPSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("payload-x"))
	}))
	defer srv.Close()
	src := HTTPSource(srv.URL, nil)
	got, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload-x" {
		t.Fatalf("got %s", got)
	}
}

func TestFileSource(t *testing.T) {
	f := filepath.Join(t.TempDir(), "rev")
	_ = os.WriteFile(f, []byte("file-payload"), 0o644)
	src := FileSource(f)
	got, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "file-payload" {
		t.Fatalf("got %s", got)
	}
}

func TestMultiSourceFallsBack(t *testing.T) {
	bad := SourceFunc(func(ctx context.Context) ([]byte, error) { return nil, errors.New("bad") })
	good := SourceFunc(func(ctx context.Context) ([]byte, error) { return []byte("ok"), nil })
	src := MultiSource(bad, good)
	got, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ok" {
		t.Fatalf("got %s", got)
	}
}

func TestCacheSequenceMonotonic(t *testing.T) {
	pub, priv := keypair(t)
	cachePath := filepath.Join(t.TempDir(), "cache")

	rawA, _ := Sign(List{Version: 1, KeyID: "k1", Sequence: 5, ExpiresAt: time.Now().Add(time.Hour)}, priv)
	if err := StoreCache(cachePath, rawA, 5); err != nil {
		t.Fatal(err)
	}
	rawB, _ := Sign(List{Version: 1, KeyID: "k1", Sequence: 3, ExpiresAt: time.Now().Add(time.Hour)}, priv)

	if _, err := LoadCache(cachePath, pub, "k1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := StoreCache(cachePath, rawB, 3); err == nil {
		t.Fatal("expected rejection on sequence regression")
	}
}
