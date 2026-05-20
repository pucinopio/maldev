package service

import (
	"context"
	"testing"

	"github.com/oioio-space/maldev/internal/manager/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSingletons(context.Background(), []byte("salt-16-byte-xxx"), []byte("c")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestAuditAppendList(t *testing.T) {
	s := newTestStore(t)
	a := NewAuditService(s)
	ctx := context.Background()
	if err := a.Append(ctx, "test.event", "alice", Target{Kind: "X", ID: "id-1"}, map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	rows, err := a.List(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Kind != "test.event" {
		t.Fatalf("got %+v", rows)
	}
}

func TestAuditListForTarget(t *testing.T) {
	s := newTestStore(t)
	a := NewAuditService(s)
	ctx := context.Background()
	_ = a.Append(ctx, "k1", "alice", Target{Kind: "A", ID: "1"}, nil)
	_ = a.Append(ctx, "k2", "alice", Target{Kind: "B", ID: "1"}, nil)
	rows, err := a.ListForTarget(ctx, Target{Kind: "A", ID: "1"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Kind != "k1" {
		t.Fatalf("got %+v", rows)
	}
}
