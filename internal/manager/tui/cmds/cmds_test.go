package cmds

import (
	"testing"
)

// TestDashboardSnapshotCmd_NilService — contract: nil svc returns an empty
// snapshot with nil Err, so cmd/tui-snap (snapshot tool, no service wired)
// can render the dashboard chrome without surfacing a fake error.
func TestDashboardSnapshotCmd_NilService(t *testing.T) {
	cmd := DashboardSnapshotCmd(nil)
	if cmd == nil {
		t.Fatal("DashboardSnapshotCmd(nil) returned nil cmd")
	}
	msg := cmd()
	snap, ok := msg.(DashboardSnapshotMsg)
	if !ok {
		t.Fatalf("cmd produced %T, want DashboardSnapshotMsg", msg)
	}
	if snap.Err != nil {
		t.Errorf("nil svc: snapshot.Err = %v, want nil (empty snapshot expected)", snap.Err)
	}
	if snap.Active != 0 || snap.Revoked != 0 {
		t.Errorf("nil svc snapshot should be zero-valued, got %+v", snap)
	}
}

// TestTryUnlockCmd_NilStore — cmd must produce an UnlockResultMsg with Err
// non-nil instead of panicking when no store is wired.
func TestTryUnlockCmd_NilStore(t *testing.T) {
	cmd := TryUnlockCmd(nil, "any")
	if cmd == nil {
		t.Fatal("TryUnlockCmd(nil) returned nil; expected an error-producing cmd")
	}
	msg := cmd()
	res, ok := msg.(UnlockResultMsg)
	if !ok {
		t.Fatalf("cmd produced %T, want UnlockResultMsg", msg)
	}
	if res.Err == nil {
		t.Error("nil store: res.Err should be non-nil")
	}
}

// TestDashboardSnapshotMsg_TypeAlias — the Msg type is literally a copy of
// the Snapshot struct so consumers can field-access without a conversion.
// Pins the contract.
func TestDashboardSnapshotMsg_FieldsAccessible(t *testing.T) {
	m := DashboardSnapshotMsg{Active: 3, Revoked: 1}
	if m.Active != 3 || m.Revoked != 1 {
		t.Errorf("DashboardSnapshotMsg fields not accessible: got %+v", m)
	}
}
