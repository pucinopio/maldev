package heartbeat

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPClientOK(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		reply := Reply{
			Version:    1,
			KeyID:      "k1",
			LicenseID:  req.LicenseID,
			Ok:         true,
			NonceEcho:  req.Nonce,
			ServerTime: time.Now().UTC(),
			ValidUntil: time.Now().Add(time.Hour).UTC(),
		}
		signed, _ := SignReply(reply, priv)
		_, _ = w.Write(signed)
	}))
	defer srv.Close()
	cli := HTTPClient(srv.URL, nil)
	reply, raw, err := cli.Ping(context.Background(), "lic-1", []byte("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if !reply.Ok || string(reply.NonceEcho) != "abc" {
		t.Fatalf("bad reply: %+v", reply)
	}
	if _, err := VerifyReply(raw, pub, "k1"); err != nil {
		t.Fatalf("reply signature: %v", err)
	}
}
