package httpsrv

import (
	"testing"
	"time"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

func TestBundleMergedEvents(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetRevocationListen("127.0.0.1:0")
		u.SetHeartbeatListen("127.0.0.1:0")
		u.SetProbeListen("127.0.0.1:0")
	})
	rev := NewRevocationServer(svc.Revoke, svc.License, svc.Settings, svc.KEK)
	hb := NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings)
	pb := NewProbeServer(svc.Probe, svc.Settings)
	bundle := NewBundle(rev, hb, pb)

	if err := rev.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer rev.Stop(time.Second)
	if err := hb.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer hb.Stop(time.Second)
	if err := pb.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer pb.Stop(time.Second)

	merged := bundle.MergedEvents()

	// Each Start emits a "started" event; drain 3 within 2 seconds.
	got := 0
	timeout := time.After(2 * time.Second)
	for got < 3 {
		select {
		case e := <-merged:
			if e.Kind == "started" {
				got++
			}
		case <-timeout:
			t.Fatalf("only got %d started events", got)
		}
	}
}

func TestBundleMergedEventsIdempotent(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetRevocationListen("127.0.0.1:0")
		u.SetHeartbeatListen("127.0.0.1:0")
		u.SetProbeListen("127.0.0.1:0")
	})
	bundle := NewBundle(
		NewRevocationServer(svc.Revoke, svc.License, svc.Settings, svc.KEK),
		NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings),
		NewProbeServer(svc.Probe, svc.Settings),
	)
	ch1 := bundle.MergedEvents()
	ch2 := bundle.MergedEvents()
	if ch1 != ch2 {
		t.Fatal("MergedEvents must return the same channel on repeated calls")
	}
}

func TestBundleStopAll(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetRevocationListen("127.0.0.1:0")
		u.SetHeartbeatListen("127.0.0.1:0")
		u.SetProbeListen("127.0.0.1:0")
	})
	rev := NewRevocationServer(svc.Revoke, svc.License, svc.Settings, svc.KEK)
	hb := NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings)
	pb := NewProbeServer(svc.Probe, svc.Settings)
	bundle := NewBundle(rev, hb, pb)

	_ = rev.Start(ctx)
	_ = hb.Start(ctx)
	_ = pb.Start(ctx)

	if err := bundle.StopAll(2 * time.Second); err != nil {
		t.Fatalf("StopAll: %v", err)
	}
	// After StopAll each server should report not running.
	if rev.Status().Running || hb.Status().Running || pb.Status().Running {
		t.Fatal("at least one server still running after StopAll")
	}
}

func TestBundleStopAllIdempotent(t *testing.T) {
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetRevocationListen("127.0.0.1:0")
		u.SetHeartbeatListen("127.0.0.1:0")
		u.SetProbeListen("127.0.0.1:0")
	})
	bundle := NewBundle(
		NewRevocationServer(svc.Revoke, svc.License, svc.Settings, svc.KEK),
		NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings),
		NewProbeServer(svc.Probe, svc.Settings),
	)
	// Servers never started — StopAll must not panic or error.
	if err := bundle.StopAll(time.Second); err != nil {
		t.Fatalf("StopAll on unstarted bundle: %v", err)
	}
	if err := bundle.StopAll(time.Second); err != nil {
		t.Fatalf("second StopAll: %v", err)
	}
}

func TestBundleNilServers(t *testing.T) {
	// Bundle must tolerate nil fields (e.g. heartbeat disabled in config).
	// Pass untyped nil so the struct fields are genuinely nil pointers.
	bundle := &Bundle{}
	ch := bundle.MergedEvents()
	if ch == nil {
		t.Fatal("MergedEvents returned nil channel")
	}
	if err := bundle.StopAll(time.Second); err != nil {
		t.Fatalf("StopAll on nil-server bundle: %v", err)
	}
}
