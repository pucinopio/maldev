package service

import (
	"context"
	"testing"
	"time"

	"github.com/oioio-space/maldev/internal/manager/probe"
)

func TestProbeNewTokenAndHistory(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	svc := NewProbeService(s, NewAuditService(s))
	row, err := svc.NewToken(ctx, "alice-prod", time.Minute, "op")
	if err != nil {
		t.Fatal(err)
	}
	if len(row.ID) != 32 {
		t.Fatalf("ID len=%d want 32", len(row.ID))
	}
	hist, err := svc.History(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 || hist[0].ID != row.ID {
		t.Fatalf("history=%v", hist)
	}
}

func TestProbeConsumeNotifiesSubscriber(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	svc := NewProbeService(s, NewAuditService(s))

	row, _ := svc.NewToken(ctx, "x", time.Minute, "op")
	ch := svc.Subscribe(row.ID)

	go func() {
		_ = svc.ConsumeToken(ctx, row.ID, probe.AgentResult{
			Hostname:     "host1",
			OS:           "linux",
			Arch:         "amd64",
			LocalHex:     "aa",
			CompositeHex: "bb",
		}, "1.2.3.4")
	}()

	select {
	case got := <-ch:
		if got == nil {
			t.Fatal("subscriber received nil")
		}
		if got.Hostname != "host1" {
			t.Fatalf("hostname=%q", got.Hostname)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber did not receive within 2s")
	}
}

func TestProbeConsumeRejectsExpiredToken(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	svc := NewProbeService(s, NewAuditService(s))
	row, _ := svc.NewToken(ctx, "x", 1*time.Nanosecond, "op")
	time.Sleep(2 * time.Millisecond)
	if err := svc.ConsumeToken(ctx, row.ID, probe.AgentResult{Hostname: "h"}, ""); err == nil {
		t.Fatal("expected expiry rejection")
	}
}

func TestProbeConsumeRejectsAlreadyUsed(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	svc := NewProbeService(s, NewAuditService(s))
	row, _ := svc.NewToken(ctx, "x", time.Minute, "op")
	if err := svc.ConsumeToken(ctx, row.ID, probe.AgentResult{}, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConsumeToken(ctx, row.ID, probe.AgentResult{}, ""); err == nil {
		t.Fatal("expected reuse rejection")
	}
}
