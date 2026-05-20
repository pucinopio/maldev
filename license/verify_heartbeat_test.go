package license

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oioio-space/maldev/license/heartbeat"
)

func TestVerifyHeartbeatFailureRejects(t *testing.T) {
	pub, priv, _ := GenerateKey()
	data, _ := Issue(IssueOptions{PrivateKey: priv, KeyID: "k1", Subject: "x", NotAfter: time.Now().Add(time.Hour)})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req heartbeat.Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		reply := heartbeat.Reply{
			Version: 1, KeyID: "k1", LicenseID: req.LicenseID,
			Ok: false, Reason: "revoked", NonceEcho: req.Nonce,
			ServerTime: time.Now().UTC(),
		}
		signed, _ := heartbeat.SignReply(reply, priv)
		_, _ = w.Write(signed)
	}))
	defer srv.Close()

	_, err := Verify(data, Trusted{Keys: map[string]ed25519.PublicKey{"k1": pub}},
		WithHeartbeat(heartbeat.HTTPClient(srv.URL, nil), time.Hour),
		WithContext(context.Background()),
	)
	if !errors.Is(err, ErrLicenseInvalid) {
		t.Fatalf("expected reject, got %v", err)
	}
}
