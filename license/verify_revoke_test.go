package license

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/oioio-space/maldev/license/revoke"
)

func TestVerifyRejectsRevokedLicense(t *testing.T) {
	issuerPub, issuerPriv, _ := GenerateKey()

	data, err := Issue(IssueOptions{
		PrivateKey: issuerPriv,
		KeyID:      "k1",
		Subject:    "test",
		NotAfter:   time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	lic, _ := Inspect(data)

	listBytes, err := revoke.Sign(revoke.List{
		Version:   1,
		KeyID:     "k1",
		Sequence:  1,
		ExpiresAt: time.Now().Add(time.Hour),
		Revoked:   []string{lic.ID},
	}, issuerPriv)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(listBytes)
	}))
	defer srv.Close()

	cachePath := filepath.Join(t.TempDir(), "rev")

	_, err = Verify(data, Trusted{Keys: map[string]ed25519.PublicKey{"k1": issuerPub}},
		WithRevocation(revoke.HTTPSource(srv.URL, nil), 24*time.Hour, cachePath),
		WithContext(context.Background()),
	)
	if !errors.Is(err, ErrLicenseInvalid) {
		t.Fatalf("expected revocation rejection, got %v", err)
	}
}
