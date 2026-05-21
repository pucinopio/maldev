package httpsrv

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
	"github.com/oioio-space/maldev/license/heartbeat"
)

func TestHeartbeatActiveLicense(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetHeartbeatListen("127.0.0.1:0")
	})
	srv := NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings)
	if err := srv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(2 * time.Second)

	uuid := issueOne(t, svc, ctx, "alice")

	body, _ := json.Marshal(heartbeat.Request{LicenseID: uuid, Nonce: []byte("nn")})
	resp, err := http.Post("http://"+srv.Status().ListenAddr+"/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	pub, kid := activePublicKey(t, svc, ctx)
	reply, err := heartbeat.VerifyReply(raw, pub, kid)
	if err != nil {
		t.Fatal(err)
	}
	if !reply.Ok || reply.LicenseID != uuid {
		t.Fatalf("reply=%+v", reply)
	}
}

func TestHeartbeatRevokedLicense(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetHeartbeatListen("127.0.0.1:0")
	})
	srv := NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	uuid := issueOne(t, svc, ctx, "alice")
	lic, _ := svc.License.GetByUUID(ctx, uuid)
	_ = svc.Revoke.Revoke(ctx, lic.ID, "leak", "op")

	body, _ := json.Marshal(heartbeat.Request{LicenseID: uuid, Nonce: []byte{1, 2, 3}})
	resp, err := http.Post("http://"+srv.Status().ListenAddr+"/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	pub, kid := activePublicKey(t, svc, ctx)
	reply, err := heartbeat.VerifyReply(raw, pub, kid)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Ok || reply.Reason != "revoked" {
		t.Fatalf("expected revoked, got %+v", reply)
	}
}

func TestHeartbeatUnknownLicense(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetHeartbeatListen("127.0.0.1:0")
	})
	srv := NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	body, _ := json.Marshal(heartbeat.Request{LicenseID: "no-such-uuid", Nonce: []byte("x")})
	resp, err := http.Post("http://"+srv.Status().ListenAddr+"/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	pub, kid := activePublicKey(t, svc, ctx)
	reply, err := heartbeat.VerifyReply(raw, pub, kid)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Ok || reply.Reason != "unknown" {
		t.Fatalf("expected unknown, got %+v", reply)
	}
}

func TestHeartbeatMethodNotAllowed(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetHeartbeatListen("127.0.0.1:0")
	})
	srv := NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	req, _ := http.NewRequest(http.MethodGet, "http://"+srv.Status().ListenAddr+"/heartbeat", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", resp.StatusCode)
	}
}

func TestHeartbeatDoubleStart(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetHeartbeatListen("127.0.0.1:0")
	})
	srv := NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	if err := srv.Start(ctx); err == nil {
		t.Fatal("expected error on double Start")
	}
}

func TestHeartbeatStopIdempotent(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetHeartbeatListen("127.0.0.1:0")
	})
	srv := NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings)
	_ = srv.Start(ctx)
	if err := srv.Stop(time.Second); err != nil {
		t.Fatal(err)
	}
	if err := srv.Stop(time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestHeartbeatEvents(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetHeartbeatListen("127.0.0.1:0")
	})
	srv := NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	body, _ := json.Marshal(heartbeat.Request{LicenseID: "x", Nonce: []byte("y")})
	resp, err := http.Post("http://"+srv.Status().ListenAddr+"/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	events := srv.Events()
	got := 0
	for {
		select {
		case e := <-events:
			got++
			if e.Server != "heartbeat" {
				t.Fatalf("event server=%q want heartbeat", e.Server)
			}
			if got >= 2 {
				return
			}
		case <-time.After(time.Second):
			t.Fatalf("only got %d events, want >=2", got)
		}
	}
}

func TestHeartbeatStatusCounters(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetHeartbeatListen("127.0.0.1:0")
	})
	srv := NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	st := srv.Status()
	if !st.Running {
		t.Fatal("should be running")
	}
	if st.ListenAddr == "" {
		t.Fatal("no listen addr")
	}

	body, _ := json.Marshal(heartbeat.Request{LicenseID: "x", Nonce: []byte("z")})
	resp, _ := http.Post("http://"+st.ListenAddr+"/heartbeat", "application/json", bytes.NewReader(body))
	resp.Body.Close()

	// Give the handler time to increment the counter.
	time.Sleep(10 * time.Millisecond)
	if srv.Status().Requests == 0 {
		t.Fatal("requests counter not incremented")
	}
}

// issueOne is declared in revocation_test.go (same package).
// activePublicKey is declared in revocation_test.go (same package), returns (ed25519.PublicKey, string).
var _ context.Context // ensure context import is used
