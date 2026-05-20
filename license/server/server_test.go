package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/oioio-space/maldev/license"
	"github.com/oioio-space/maldev/license/heartbeat"
	"github.com/oioio-space/maldev/license/revoke"
)

func TestRevocationHandlerServesSigned(t *testing.T) {
	pub, priv, _ := license.GenerateKey()
	store := FileStore(filepath.Join(t.TempDir(), "rev"))
	h := NewRevocationHandler(RevocationOptions{
		PrivateKey: priv, KeyID: "k1", Store: store, ValidFor: time.Hour,
	})
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if _, err := revoke.VerifyBytes(raw, pub, "k1"); err != nil {
		t.Fatalf("signature: %v", err)
	}
}

func TestRevocationHandlerAdminAddRemove(t *testing.T) {
	_, priv, _ := license.GenerateKey()
	store := FileStore(filepath.Join(t.TempDir(), "rev"))
	h := NewRevocationHandler(RevocationOptions{
		PrivateKey: priv, KeyID: "k1", Store: store, ValidFor: time.Hour, AdminToken: "topsecret",
	})
	srv := httptest.NewServer(h)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL,
		bytes.NewReader([]byte(`{"add":["lic-1","lic-2"]}`)))
	req.Header.Set("Authorization", "Bearer topsecret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%v", resp.StatusCode)
	}
	cur, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cur.Revoked) != 2 {
		t.Fatalf("revoked=%v", cur.Revoked)
	}
}

func TestHeartbeatHandlerActive(t *testing.T) {
	pub, priv, _ := license.GenerateKey()
	h := NewHeartbeatHandler(HeartbeatOptions{
		PrivateKey: priv, KeyID: "k1",
		Store:    StaticLicenseStore{"lic-good": StatusActive},
		ValidFor: time.Hour,
	})
	srv := httptest.NewServer(h)
	defer srv.Close()
	body, _ := json.Marshal(heartbeat.Request{LicenseID: "lic-good", Nonce: []byte("nn")})
	resp, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%v", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	reply, err := heartbeat.VerifyReply(raw, pub, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if !reply.Ok || string(reply.NonceEcho) != "nn" {
		t.Fatalf("bad reply: %+v", reply)
	}
}
