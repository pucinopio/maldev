package httpsrv

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/oioio-space/maldev/internal/manager/probe"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

func TestProbeAgentDownload(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetProbeListen("127.0.0.1:0")
	})
	srv := NewProbeServer(svc.Probe, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	tok, _ := svc.Probe.NewToken(ctx, "test", time.Minute, "op")
	resp, err := http.Get("http://" + srv.Status().ListenAddr + "/probe/" + tok.ID + "/agent/linux-amd64")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if len(b) < 1000 {
		t.Fatalf("agent too small: %d bytes", len(b))
	}
}

func TestProbeResultRoundTrip(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetProbeListen("127.0.0.1:0")
	})
	srv := NewProbeServer(svc.Probe, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	tok, _ := svc.Probe.NewToken(ctx, "test", time.Minute, "op")
	result := probe.AgentResult{
		Hostname:     "host1",
		OS:           "linux",
		Arch:         "amd64",
		LocalHex:     "aabb",
		CompositeHex: "ccdd",
	}
	body, _ := json.Marshal(result)
	resp, err := http.Post("http://"+srv.Status().ListenAddr+"/probe/"+tok.ID+"/result", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	hist, _ := svc.Probe.History(ctx, 10)
	if len(hist) != 1 || hist[0].Hostname != "host1" {
		t.Fatalf("hist=%+v", hist)
	}
}

func TestProbeSnippet(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetProbeListen("127.0.0.1:0")
	})
	srv := NewProbeServer(svc.Probe, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	tok, _ := svc.Probe.NewToken(ctx, "test", time.Minute, "op")
	resp, err := http.Get("http://" + srv.Status().ListenAddr + "/probe/" + tok.ID + "/snippet")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(b, []byte("curl")) || !bytes.Contains(b, []byte(tok.ID)) {
		t.Fatalf("snippet missing expected content: %s", b)
	}
}

func TestProbeResultExpiredToken(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetProbeListen("127.0.0.1:0")
	})
	srv := NewProbeServer(svc.Probe, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	// TTL of 1ns ensures the token is already expired.
	tok, _ := svc.Probe.NewToken(ctx, "test", time.Nanosecond, "op")
	time.Sleep(time.Millisecond)

	result := probe.AgentResult{Hostname: "h", OS: "linux", Arch: "amd64"}
	body, _ := json.Marshal(result)
	resp, err := http.Post("http://"+srv.Status().ListenAddr+"/probe/"+tok.ID+"/result", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for expired token, got %d", resp.StatusCode)
	}
}

func TestProbeResultMethodNotAllowed(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetProbeListen("127.0.0.1:0")
	})
	srv := NewProbeServer(svc.Probe, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	tok, _ := svc.Probe.NewToken(ctx, "test", time.Minute, "op")
	req, _ := http.NewRequest(http.MethodGet, "http://"+srv.Status().ListenAddr+"/probe/"+tok.ID+"/result", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", resp.StatusCode)
	}
}

func TestProbeDoubleStart(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetProbeListen("127.0.0.1:0")
	})
	srv := NewProbeServer(svc.Probe, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	if err := srv.Start(ctx); err == nil {
		t.Fatal("expected error on double Start")
	}
}

func TestProbeStopIdempotent(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetProbeListen("127.0.0.1:0")
	})
	srv := NewProbeServer(svc.Probe, svc.Settings)
	_ = srv.Start(ctx)
	if err := srv.Stop(time.Second); err != nil {
		t.Fatal(err)
	}
	if err := srv.Stop(time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestProbeStatusCounters(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetProbeListen("127.0.0.1:0")
	})
	srv := NewProbeServer(svc.Probe, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	st := srv.Status()
	if !st.Running {
		t.Fatal("should be running")
	}
	tok, _ := svc.Probe.NewToken(ctx, "test", time.Minute, "op")
	resp, _ := http.Get("http://" + st.ListenAddr + "/probe/" + tok.ID + "/snippet")
	resp.Body.Close()

	time.Sleep(10 * time.Millisecond)
	if srv.Status().Requests == 0 {
		t.Fatal("requests counter not incremented")
	}
}

func TestProbeUnknownVerb(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetProbeListen("127.0.0.1:0")
	})
	srv := NewProbeServer(svc.Probe, svc.Settings)
	_ = srv.Start(ctx)
	defer srv.Stop(2 * time.Second)

	resp, err := http.Get("http://" + srv.Status().ListenAddr + "/probe/tok/badverb")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

// ensure context import used by inline declarations above
var _ context.Context
