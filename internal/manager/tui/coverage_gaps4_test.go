package tui

// Coverage gaps closed in Session 0004 (batch 4):
//   - screen_identities.handleIdentityInputResult / handleIdentityConfirmResult
//   - screen_issuers.handleIssuerInputResult
//   - screen_recipients.handleRecipientInputResult / handleRecipientConfirmResult
//
// Each handler has three branches worth a guard:
//   1. nil-svc / unknown-ID → returns nil cmd (no-op)
//   2. valid-ID + nil-svc   → returns nil cmd (also no-op)
//   3. valid-ID + wired svc → returns non-nil cmd; executing it produces the
//      matching *LoadedMsg with the post-action row set

import (
	"context"
	"testing"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// ── identities ───────────────────────────────────────────────────────────────

// TestHandleIdentityInputResult_NilSvcNoop asserts nil-svc returns nil cmd
// for both known IDs.
func TestHandleIdentityInputResult_NilSvcNoop(t *testing.T) {
	m := identitiesModel{} // svc == nil
	for _, id := range []string{"identity-name", "identity-export"} {
		_, cmd := m.handleIdentityInputResult(InputResultMsg{ID: id, Value: "x"})
		if cmd != nil {
			t.Errorf("nil-svc handler must return nil cmd for ID=%q", id)
		}
	}
}

// TestHandleIdentityInputResult_UnknownIDNoop asserts unknown IDs are ignored.
func TestHandleIdentityInputResult_UnknownIDNoop(t *testing.T) {
	svc, _ := newTestServices(t)
	m := identitiesModel{svc: svc}
	_, cmd := m.handleIdentityInputResult(InputResultMsg{ID: "no-such-id", Value: "x"})
	if cmd != nil {
		t.Fatal("unknown ID must produce nil cmd")
	}
}

// TestHandleIdentityInputResult_CreatesIdentity wires a real svc, fires the
// 'identity-name' result, executes the returned cmd, and asserts the resulting
// IdentitiesLoadedMsg lists the new row.
func TestHandleIdentityInputResult_CreatesIdentity(t *testing.T) {
	svc, _ := newTestServices(t)
	m := identitiesModel{svc: svc}
	_, cmd := m.handleIdentityInputResult(InputResultMsg{ID: "identity-name", Value: "alice-test"})
	if cmd == nil {
		t.Fatal("cmd must be non-nil with wired svc")
	}
	msg := cmd()
	loaded, ok := msg.(IdentitiesLoadedMsg)
	if !ok {
		t.Fatalf("expected IdentitiesLoadedMsg, got %T", msg)
	}
	if loaded.Err != nil {
		t.Fatalf("Create failed: %v", loaded.Err)
	}
	if len(loaded.Rows) != 1 || loaded.Rows[0].Name != "alice-test" {
		t.Fatalf("rows = %+v, want [alice-test]", loaded.Rows)
	}
}

// TestHandleIdentityConfirmResult_NilSvcNoop — confirm handler must no-op on
// nil-svc for every action ID.
func TestHandleIdentityConfirmResult_NilSvcNoop(t *testing.T) {
	m := identitiesModel{}
	for _, id := range []string{"identity-regen", "identity-delete"} {
		_, cmd := m.handleIdentityConfirmResult(ConfirmResultMsg{ID: id, Confirm: true})
		if cmd != nil {
			t.Errorf("nil-svc confirm must return nil cmd for ID=%q", id)
		}
	}
}

// TestHandleIdentityConfirmResult_NotConfirmedNoop — even with svc wired, a
// Confirm:false response is a no-op (operator cancelled).
func TestHandleIdentityConfirmResult_NotConfirmedNoop(t *testing.T) {
	svc, _ := newTestServices(t)
	m := identitiesModel{svc: svc}
	for _, id := range []string{"identity-regen", "identity-delete"} {
		_, cmd := m.handleIdentityConfirmResult(ConfirmResultMsg{ID: id, Confirm: false})
		if cmd != nil {
			t.Errorf("Confirm:false must return nil cmd for ID=%q", id)
		}
	}
}

// ── issuers ──────────────────────────────────────────────────────────────────

// TestHandleIssuerInputResult_NilSvcNoop — issuer-name + issuer-export-pub
// must return nil cmd when no svc is wired.
func TestHandleIssuerInputResult_NilSvcNoop(t *testing.T) {
	m := issuersModel{}
	for _, id := range []string{"issuer-name", "issuer-export-pub"} {
		_, cmd := m.handleIssuerInputResult(InputResultMsg{ID: id, Value: "x"})
		if cmd != nil {
			t.Errorf("nil-svc handler must return nil cmd for ID=%q", id)
		}
	}
}

// TestHandleIssuerInputResult_GeneratesIssuer fires 'issuer-name' with a real
// svc and asserts the resulting IssuersLoadedMsg shows the new row.
func TestHandleIssuerInputResult_GeneratesIssuer(t *testing.T) {
	svc, _ := newTestServices(t)
	m := issuersModel{svc: svc}
	_, cmd := m.handleIssuerInputResult(InputResultMsg{ID: "issuer-name", Value: "ed25519-test"})
	if cmd == nil {
		t.Fatal("issuer-name with wired svc must emit cmd")
	}
	msg := cmd()
	loaded, ok := msg.(IssuersLoadedMsg)
	if !ok {
		t.Fatalf("expected IssuersLoadedMsg, got %T", msg)
	}
	if loaded.Err != nil {
		t.Fatalf("Generate failed: %v", loaded.Err)
	}
	if len(loaded.Rows) != 1 || loaded.Rows[0].Name != "ed25519-test" {
		t.Fatalf("rows = %+v, want [ed25519-test]", loaded.Rows)
	}
}

// ── recipients ───────────────────────────────────────────────────────────────

// TestHandleRecipientInputResult_NilSvcNoop — recipient-name +
// recipient-export-pub must return nil cmd when no svc is wired.
func TestHandleRecipientInputResult_NilSvcNoop(t *testing.T) {
	m := recipientsModel{}
	for _, id := range []string{"recipient-name", "recipient-export-pub"} {
		_, cmd := m.handleRecipientInputResult(InputResultMsg{ID: id, Value: "x"})
		if cmd != nil {
			t.Errorf("nil-svc handler must return nil cmd for ID=%q", id)
		}
	}
}

// TestHandleRecipientInputResult_GeneratesRecipient fires 'recipient-name'
// against a wired svc.
func TestHandleRecipientInputResult_GeneratesRecipient(t *testing.T) {
	svc, _ := newTestServices(t)
	m := recipientsModel{svc: svc}
	_, cmd := m.handleRecipientInputResult(InputResultMsg{ID: "recipient-name", Value: "x25519-test"})
	if cmd == nil {
		t.Fatal("recipient-name with wired svc must emit cmd")
	}
	msg := cmd()
	loaded, ok := msg.(RecipientsLoadedMsg)
	if !ok {
		t.Fatalf("expected RecipientsLoadedMsg, got %T", msg)
	}
	if loaded.Err != nil {
		t.Fatalf("Generate failed: %v", loaded.Err)
	}
	if len(loaded.Rows) != 1 || loaded.Rows[0].Name != "x25519-test" {
		t.Fatalf("rows = %+v, want [x25519-test]", loaded.Rows)
	}
}

// TestHandleRecipientConfirmResult_DeletesRecipient seeds a recipient,
// confirms deletion, and asserts the row disappears.
func TestHandleRecipientConfirmResult_DeletesRecipient(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	row, err := svc.Recipient.Generate(ctx, "to-be-deleted", "operator")
	if err != nil {
		t.Fatalf("seed Generate: %v", err)
	}

	m := newRecipientsModel(svc)
	m.width = 120
	m.hgt = 40
	// The handler reads selectedRow() — wire the row into the table.
	m.rows = []*ent.RecipientKey{row}
	m.rebuildTable()
	m.table.SetCursor(0)

	_, cmd := m.handleRecipientConfirmResult(ConfirmResultMsg{ID: "recipient-delete", Confirm: true})
	if cmd == nil {
		t.Fatal("recipient-delete confirmed must emit cmd")
	}
	msg := cmd()
	loaded, ok := msg.(RecipientsLoadedMsg)
	if !ok {
		t.Fatalf("expected RecipientsLoadedMsg, got %T", msg)
	}
	if loaded.Err != nil {
		t.Fatalf("Delete failed: %v", loaded.Err)
	}
	if len(loaded.Rows) != 0 {
		t.Fatalf("rows after delete = %d, want 0", len(loaded.Rows))
	}
}

// TestHandleRecipientConfirmResult_NotConfirmedNoop — cancel path returns nil.
func TestHandleRecipientConfirmResult_NotConfirmedNoop(t *testing.T) {
	svc, _ := newTestServices(t)
	m := recipientsModel{svc: svc}
	_, cmd := m.handleRecipientConfirmResult(ConfirmResultMsg{ID: "recipient-delete", Confirm: false})
	if cmd != nil {
		t.Fatal("Confirm:false must return nil cmd")
	}
}
