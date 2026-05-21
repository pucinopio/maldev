package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/oioio-space/maldev/internal/manager/probe"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	probeent "github.com/oioio-space/maldev/internal/manager/store/ent/probetoken"
)

type ProbeService struct {
	store *store.Store
	audit *AuditService

	mu          sync.Mutex
	subscribers map[string]chan *ent.ProbeToken
}

func NewProbeService(s *store.Store, a *AuditService) *ProbeService {
	return &ProbeService{
		store:       s,
		audit:       a,
		subscribers: map[string]chan *ent.ProbeToken{},
	}
}

// NewToken generates a random token id, persists a row, and returns it.
func (svc *ProbeService) NewToken(ctx context.Context, label string, ttl time.Duration, actor string) (*ent.ProbeToken, error) {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(raw[:])

	tx, err := svc.store.Client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	row, err := tx.ProbeToken.Create().
		SetID(id).
		SetLabel(label).
		SetExpiresAt(time.Now().Add(ttl)).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := svc.audit.AppendTx(ctx, tx, "probe.token.new", actor,
		Target{Kind: "ProbeToken", ID: id},
		map[string]any{"label": label, "ttl_seconds": int(ttl.Seconds())}); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return row, nil
}

// Subscribe registers a channel that receives the populated ProbeToken
// when ConsumeToken accepts a matching POST. Returns a closed channel if
// the token doesn't exist or is already used/expired.
func (svc *ProbeService) Subscribe(tokenID string) <-chan *ent.ProbeToken {
	ch := make(chan *ent.ProbeToken, 1)
	svc.mu.Lock()
	svc.subscribers[tokenID] = ch
	svc.mu.Unlock()
	return ch
}

// Unsubscribe drops the subscriber for tokenID. Safe to call after close.
func (svc *ProbeService) Unsubscribe(tokenID string) {
	svc.mu.Lock()
	if ch, ok := svc.subscribers[tokenID]; ok {
		close(ch)
		delete(svc.subscribers, tokenID)
	}
	svc.mu.Unlock()
}

// ConsumeToken validates the token, persists the agent result, marks used,
// and notifies any subscriber. Called by the HTTP probe server POST handler.
func (svc *ProbeService) ConsumeToken(ctx context.Context, tokenID string, result probe.AgentResult, remoteAddr string) error {
	row, err := svc.store.Client.ProbeToken.Get(ctx, tokenID)
	if err != nil {
		return err
	}
	now := time.Now()
	if now.After(row.ExpiresAt) {
		return fmt.Errorf("probe: token %s expired", tokenID)
	}
	if row.UsedAt != nil {
		return fmt.Errorf("probe: token %s already used", tokenID)
	}

	tx, err := svc.store.Client.Tx(ctx)
	if err != nil {
		return err
	}
	updated, err := tx.ProbeToken.UpdateOneID(tokenID).
		SetUsedAt(now).
		SetRemoteAddr(remoteAddr).
		SetHostname(result.Hostname).
		SetOs(result.OS).
		SetArch(result.Arch).
		SetCPUBrand(result.CPUBrand).
		SetLocalHex(result.LocalHex).
		SetCompositeHex(result.CompositeHex).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := svc.audit.AppendTx(ctx, tx, "probe.token.consumed", "remote",
		Target{Kind: "ProbeToken", ID: tokenID},
		map[string]any{"hostname": result.Hostname, "os": result.OS}); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	svc.mu.Lock()
	if ch, ok := svc.subscribers[tokenID]; ok {
		select {
		case ch <- updated:
		default:
		}
		close(ch)
		delete(svc.subscribers, tokenID)
	}
	svc.mu.Unlock()
	return nil
}

// History returns the most recently created probe tokens (used or not),
// limited to limit rows.
func (svc *ProbeService) History(ctx context.Context, limit int) ([]*ent.ProbeToken, error) {
	if limit <= 0 {
		limit = 100
	}
	return svc.store.Client.ProbeToken.Query().
		Order(ent.Desc(probeent.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
}

// Revoke marks an unused token as expired (sets expires_at to now).
func (svc *ProbeService) Revoke(ctx context.Context, tokenID, actor string) error {
	tx, err := svc.store.Client.Tx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.ProbeToken.UpdateOneID(tokenID).SetExpiresAt(time.Now()).Save(ctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := svc.audit.AppendTx(ctx, tx, "probe.token.revoke", actor,
		Target{Kind: "ProbeToken", ID: tokenID}, nil); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
