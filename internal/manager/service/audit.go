package service

import (
	"context"

	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	"github.com/oioio-space/maldev/internal/manager/store/ent/auditevent"
)

// Target identifies the entity an audit event refers to. ID is the
// stringified UUID (or "-" for global events).
type Target struct {
	Kind string
	ID   string
}

// AuditService writes structured operator events to the store.
type AuditService struct {
	store *store.Store
}

func NewAuditService(s *store.Store) *AuditService {
	return &AuditService{store: s}
}

// Append writes a new audit row. Pass nil payload for events with no
// extra detail.
func (a *AuditService) Append(ctx context.Context, kind, actor string, target Target, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	_, err := a.store.Client.AuditEvent.Create().
		SetKind(kind).
		SetTargetKind(target.Kind).
		SetTargetID(target.ID).
		SetActor(actor).
		SetPayload(payload).
		Save(ctx)
	return err
}

// AppendTx is the tx-aware variant used by services that compose multiple
// writes atomically.
func (a *AuditService) AppendTx(ctx context.Context, tx *ent.Tx, kind, actor string, target Target, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	_, err := tx.AuditEvent.Create().
		SetKind(kind).
		SetTargetKind(target.Kind).
		SetTargetID(target.ID).
		SetActor(actor).
		SetPayload(payload).
		Save(ctx)
	return err
}

// List returns up to limit recent events, newest first.
func (a *AuditService) List(ctx context.Context, limit int) ([]*ent.AuditEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	return a.store.Client.AuditEvent.Query().
		Order(ent.Desc(auditevent.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
}

// ListForTarget returns events filtered to one target.
func (a *AuditService) ListForTarget(ctx context.Context, t Target, limit int) ([]*ent.AuditEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	return a.store.Client.AuditEvent.Query().
		Where(
			auditevent.TargetKindEQ(t.Kind),
			auditevent.TargetIDEQ(t.ID),
		).
		Order(ent.Desc(auditevent.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
}
