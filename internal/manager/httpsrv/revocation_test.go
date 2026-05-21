package httpsrv

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	licensekg "github.com/oioio-space/maldev/license"
	"github.com/oioio-space/maldev/license/revoke"
)

func setupForTest(t *testing.T) (*service.Services, context.Context) {
	t.Helper()
	ctx := context.Background()
	s, err := store.New(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSingletons(ctx, []byte("salt-16-byte-xxx"), []byte("canary")); err != nil {
		t.Fatal(err)
	}
	kek := crypto.DeriveFromPassphrase("test", [16]byte{1})
	svc := service.New(s, kek)
	iss, _ := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	_ = svc.Issuer.SetActive(ctx, iss.ID, "op")
	t.Cleanup(func() { _ = svc.Close() })
	return svc, ctx
}

func issueOne(t *testing.T, svc *service.Services, ctx context.Context, subject string) string {
	t.Helper()
	iss, _ := svc.Issuer.Active(ctx)
	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: subject,
		NotAfter: time.Now().Add(time.Hour),
		Actor:    "op",
	})
	if err != nil {
		t.Fatal(err)
	}
	return out.Row.LicenseUUID
}

// activePublicKey returns the active issuer's parsed public key and key ID.
func activePublicKey(t *testing.T, svc *service.Services, ctx context.Context) (ed25519.PublicKey, string) {
	t.Helper()
	iss, err := svc.Issuer.Active(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pubBytes, err := svc.Issuer.ExportPublic(ctx, iss.ID)
	if err != nil {
		t.Fatal(err)
	}
	pub, kid, err := licensekg.ParsePublicKey(pubBytes)
	if err != nil {
		t.Fatal(err)
	}
	return pub, kid
}

func startRevServer(t *testing.T, svc *service.Services, ctx context.Context) *RevocationServer {
	t.Helper()
	_, err := svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetRevocationListen("127.0.0.1:0")
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := NewRevocationServer(svc.Revoke, svc.License, svc.Settings, svc.KEK)
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Stop(2 * time.Second) })
	return srv
}

func TestRevocationServerGet(t *testing.T) {
	svc, ctx := setupForTest(t)
	srv := startRevServer(t, svc, ctx)

	addr := srv.Status().ListenAddr
	if addr == "" {
		t.Fatal("no listen addr")
	}

	resp, err := http.Get("http://" + addr + "/revoked.pem")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-pem-file" {
		t.Fatalf("content-type=%q", ct)
	}
	// Empty list is still valid signed PEM.
	pub, kid := activePublicKey(t, svc, ctx)
	parsed, err := revoke.VerifyBytes(body, pub, kid)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Revoked) != 0 {
		t.Fatalf("expected empty list, got %v", parsed.Revoked)
	}
}

func TestRevocationServerLifecycleAndGet(t *testing.T) {
	svc, ctx := setupForTest(t)
	srv := startRevServer(t, svc, ctx)

	addr := srv.Status().ListenAddr
	if addr == "" {
		t.Fatal("no listen addr")
	}

	// Revoke a licence first.
	licUUID := issueOne(t, svc, ctx, "alice")
	lic, err := svc.License.GetByUUID(ctx, licUUID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Revoke.Revoke(ctx, lic.ID, "test", "op"); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get("http://" + addr + "/revoked.pem")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)

	pub, kid := activePublicKey(t, svc, ctx)
	parsed, err := revoke.VerifyBytes(body, pub, kid)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Revoked) != 1 || parsed.Revoked[0] != licUUID {
		t.Fatalf("revoked=%v want [%s]", parsed.Revoked, licUUID)
	}

	// Status counters.
	st := srv.Status()
	if !st.Running {
		t.Fatal("server should be running")
	}
	if st.Requests == 0 {
		t.Fatal("requests counter not incremented")
	}
}

func TestRevocationServerMethodNotAllowed(t *testing.T) {
	svc, ctx := setupForTest(t)
	srv := startRevServer(t, svc, ctx)

	addr := srv.Status().ListenAddr
	req, _ := http.NewRequest(http.MethodPut, "http://"+addr+"/revoked.pem", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", resp.StatusCode)
	}
}

func TestRevocationServerAdminPost(t *testing.T) {
	svc, ctx := setupForTest(t)

	// Wrap a known token and store it in config.
	token := "super-secret-token"
	enc, err := svc.KEK.Wrap([]byte(token))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetRevocationListen("127.0.0.1:0")
		u.SetRevocationAdminTokenEnc(enc)
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := NewRevocationServer(svc.Revoke, svc.License, svc.Settings, svc.KEK)
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Stop(2 * time.Second) })

	addr := srv.Status().ListenAddr
	licUUID := issueOne(t, svc, ctx, "bob")

	// POST add.
	payload, _ := json.Marshal(map[string]any{
		"add":    []string{licUUID},
		"reason": "test-admin",
	})
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/revoked.pem", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin POST add: want 200, got %d", resp.StatusCode)
	}

	// Verify revocation appears in the list.
	getResp, err := http.Get("http://" + addr + "/revoked.pem")
	if err != nil {
		t.Fatal(err)
	}
	defer getResp.Body.Close()
	body, _ := io.ReadAll(getResp.Body)
	pub, kid := activePublicKey(t, svc, ctx)
	parsed, err := revoke.VerifyBytes(body, pub, kid)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Revoked) != 1 || parsed.Revoked[0] != licUUID {
		t.Fatalf("after admin add: revoked=%v want [%s]", parsed.Revoked, licUUID)
	}

	// POST remove.
	payload2, _ := json.Marshal(map[string]any{"remove": []string{licUUID}})
	req2, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/revoked.pem", bytes.NewReader(payload2))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("admin POST remove: want 200, got %d", resp2.StatusCode)
	}
}

func TestRevocationServerAdminUnauthorised(t *testing.T) {
	svc, ctx := setupForTest(t)

	enc, _ := svc.KEK.Wrap([]byte("real-token"))
	_, err := svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetRevocationListen("127.0.0.1:0")
		u.SetRevocationAdminTokenEnc(enc)
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := NewRevocationServer(svc.Revoke, svc.License, svc.Settings, svc.KEK)
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Stop(2 * time.Second) })

	addr := srv.Status().ListenAddr
	payload, _ := json.Marshal(map[string]any{"add": []string{}})
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/revoked.pem", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestRevocationServerAdminDisabled(t *testing.T) {
	svc, ctx := setupForTest(t)
	srv := startRevServer(t, svc, ctx) // no admin token set

	addr := srv.Status().ListenAddr
	payload, _ := json.Marshal(map[string]any{"add": []string{}})
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/revoked.pem", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer anything")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestRevocationServerEvents(t *testing.T) {
	svc, ctx := setupForTest(t)
	srv := startRevServer(t, svc, ctx)

	addr := srv.Status().ListenAddr
	resp, err := http.Get("http://" + addr + "/revoked.pem")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Should have at least "started" + "request" events.
	events := srv.Events()
	got := 0
	for {
		select {
		case e := <-events:
			got++
			if e.Server != "revocation" {
				t.Fatalf("event server=%q want revocation", e.Server)
			}
			if got >= 2 {
				return
			}
		case <-time.After(time.Second):
			t.Fatalf("only got %d events, want >=2", got)
		}
	}
}

func TestRevocationServerDoubleStart(t *testing.T) {
	svc, ctx := setupForTest(t)
	srv := startRevServer(t, svc, ctx)

	if err := srv.Start(ctx); err == nil {
		t.Fatal("expected error on double Start")
	}
}

func TestRevocationServerStopIdempotent(t *testing.T) {
	svc, ctx := setupForTest(t)
	srv := startRevServer(t, svc, ctx)

	if err := srv.Stop(time.Second); err != nil {
		t.Fatal(err)
	}
	// Second stop should not error.
	if err := srv.Stop(time.Second); err != nil {
		t.Fatal(err)
	}
}
