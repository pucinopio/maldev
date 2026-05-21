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

func TestBundleStartStop(t *testing.T) {
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

	if err := bundle.Start(ctx, "revocation"); err != nil {
		t.Fatalf("Start revocation: %v", err)
	}
	if !bundle.Revocation.Status().Running {
		t.Fatal("revocation should be running after Start")
	}

	if err := bundle.Stop("revocation"); err != nil {
		t.Fatalf("Stop revocation: %v", err)
	}
	if bundle.Revocation.Status().Running {
		t.Fatal("revocation should not be running after Stop")
	}

	// Unknown server name must return error, not panic.
	if err := bundle.Start(ctx, "unknown"); err == nil {
		t.Fatal("expected error for unknown server name")
	}
	if err := bundle.Stop("unknown"); err == nil {
		t.Fatal("expected error for unknown server name")
	}
}

func TestBundleStatuses(t *testing.T) {
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

	// Before any Start all three servers report not running.
	statuses := bundle.Statuses()
	for _, name := range []string{"revocation", "heartbeat", "probe"} {
		if _, ok := statuses[name]; !ok {
			t.Fatalf("Statuses missing key %q", name)
		}
		if statuses[name].Running {
			t.Fatalf("server %q should not be running before Start", name)
		}
	}

	// Start heartbeat and verify the map reflects the new state.
	if err := bundle.Start(ctx, "heartbeat"); err != nil {
		t.Fatalf("Start heartbeat: %v", err)
	}
	defer bundle.Stop("heartbeat") //nolint:errcheck

	statuses = bundle.Statuses()
	if !statuses["heartbeat"].Running {
		t.Fatal("heartbeat should be running in Statuses after Start")
	}
	if statuses["revocation"].Running || statuses["probe"].Running {
		t.Fatal("only heartbeat should be running")
	}
}

func TestBundleControllerInterface(t *testing.T) {
	// Compile-time assertion is in bundle.go; this runtime check documents
	// that a *Bundle passed as Controller is usable end-to-end.
	svc, ctx := setupForTest(t)
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetRevocationListen("127.0.0.1:0")
		u.SetHeartbeatListen("127.0.0.1:0")
		u.SetProbeListen("127.0.0.1:0")
	})
	var ctrl Controller = NewBundle(
		NewRevocationServer(svc.Revoke, svc.License, svc.Settings, svc.KEK),
		NewHeartbeatServer(svc.Issuer, svc.License, svc.Settings),
		NewProbeServer(svc.Probe, svc.Settings),
	)

	if err := ctrl.Start(ctx, "probe"); err != nil {
		t.Fatalf("Controller.Start: %v", err)
	}
	defer ctrl.Stop("probe") //nolint:errcheck

	ch := ctrl.MergedEvents()
	if ch == nil {
		t.Fatal("Controller.MergedEvents returned nil")
	}
	if !ctrl.Statuses()["probe"].Running {
		t.Fatal("probe should be running via Controller interface")
	}
}
