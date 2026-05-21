package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/oioio-space/maldev/cleanup/memory"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	licenseent "github.com/oioio-space/maldev/internal/manager/store/ent/license"
	"github.com/oioio-space/maldev/license/revoke"
)

type RevokeService struct {
	store  *store.Store
	audit  *AuditService
	issuer *IssuerService

	cacheMu     sync.Mutex
	cachedPEM   []byte
	cachedUntil time.Time
}

// NewRevokeService wires a RevokeService. The lic parameter has been removed —
// RevokeService does not depend on LicenseService.
func NewRevokeService(s *store.Store, a *AuditService, iss *IssuerService) *RevokeService {
	return &RevokeService{store: s, audit: a, issuer: iss}
}

// invalidateCache clears the PublishSignedList cache. Call after any mutation
// that changes revocation state.
func (svc *RevokeService) invalidateCache() {
	svc.cacheMu.Lock()
	svc.cachedPEM = nil
	svc.cachedUntil = time.Time{}
	svc.cacheMu.Unlock()
}

// Revoke marks a licence revoked with the given reason. License.status is
// updated to "revoked" and a Revocation row is created. Idempotent — if
// already revoked, returns nil without re-writing.
func (svc *RevokeService) Revoke(ctx context.Context, licenseID uuid.UUID, reason, actor string) error {
	if reason == "" {
		return errors.New("revoke: reason required")
	}
	row, err := svc.store.Client.License.Get(ctx, licenseID)
	if err != nil {
		return err
	}
	if row.Status == licenseent.StatusRevoked {
		return nil // idempotent
	}

	if err := withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
		if _, err := tx.Revocation.Create().
			SetReason(reason).
			SetRevokedBy(actor).
			SetLicenseID(licenseID).
			Save(ctx); err != nil {
			return fmt.Errorf("create revocation: %w", err)
		}
		if _, err := tx.License.UpdateOneID(licenseID).
			SetStatus(licenseent.StatusRevoked).
			Save(ctx); err != nil {
			return err
		}
		return svc.audit.AppendTx(ctx, tx, "license.revoke", actor,
			Target{Kind: "License", ID: licenseID.String()},
			map[string]any{"reason": reason})
	}); err != nil {
		return err
	}
	svc.invalidateCache()
	return nil
}

// Unrevoke deletes the Revocation row and resets status back to active.
// Useful for admin error correction.
func (svc *RevokeService) Unrevoke(ctx context.Context, licenseID uuid.UUID, actor string) error {
	if err := withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
		row, err := tx.License.Get(ctx, licenseID)
		if err != nil {
			return err
		}
		rev, err := row.QueryRevocation().Only(ctx)
		if err == nil && rev != nil {
			if err := tx.Revocation.DeleteOneID(rev.ID).Exec(ctx); err != nil {
				return err
			}
		}
		if _, err := tx.License.UpdateOneID(licenseID).
			SetStatus(licenseent.StatusActive).
			Save(ctx); err != nil {
			return err
		}
		return svc.audit.AppendTx(ctx, tx, "license.unrevoke", actor,
			Target{Kind: "License", ID: licenseID.String()}, nil)
	}); err != nil {
		return err
	}
	svc.invalidateCache()
	return nil
}

// RevocationView aggregates a revoked licence's metadata for the UI.
type RevocationView struct {
	LicenseID   uuid.UUID
	LicenseUUID string
	Subject     string
	Reason      string
	RevokedAt   time.Time
	RevokedBy   string
	KeyID       string
}

// ListRevoked returns all currently-revoked licences with their reasons.
func (svc *RevokeService) ListRevoked(ctx context.Context) ([]RevocationView, error) {
	rows, err := svc.store.Client.License.Query().
		Where(licenseent.StatusEQ(licenseent.StatusRevoked)).
		WithRevocation().
		WithIssuer().
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RevocationView, 0, len(rows))
	for _, r := range rows {
		view := RevocationView{
			LicenseID:   r.ID,
			LicenseUUID: r.LicenseUUID,
			Subject:     r.Subject,
		}
		if r.Edges.Revocation != nil {
			view.Reason = r.Edges.Revocation.Reason
			view.RevokedAt = r.Edges.Revocation.RevokedAt
			view.RevokedBy = r.Edges.Revocation.RevokedBy
		}
		if r.Edges.Issuer != nil {
			view.KeyID = r.Edges.Issuer.KeyID
		}
		out = append(out, view)
	}
	return out, nil
}

// PublishSignedList builds a fresh revoke.List from the current revocation
// state, signs it with the active issuer, and returns the PEM. ValidFor
// defaults to 7 days if zero. Sequence is the current Unix timestamp — it
// strictly increases between calls, which is enough monotonicity for the
// client cache check. The HTTP revocation server calls this on every GET.
//
// The signed PEM is cached for validFor/2 and invalidated by Revoke/Unrevoke,
// so repeated GET requests during a quiet period avoid redundant signing.
func (svc *RevokeService) PublishSignedList(ctx context.Context, validFor time.Duration) ([]byte, error) {
	if validFor <= 0 {
		validFor = 7 * 24 * time.Hour
	}

	svc.cacheMu.Lock()
	if svc.cachedPEM != nil && time.Now().Before(svc.cachedUntil) {
		out := make([]byte, len(svc.cachedPEM))
		copy(out, svc.cachedPEM)
		svc.cacheMu.Unlock()
		return out, nil
	}
	svc.cacheMu.Unlock()

	iss, err := svc.issuer.Active(ctx)
	if err != nil {
		return nil, fmt.Errorf("active issuer: %w", err)
	}
	priv, err := svc.issuer.PrivateKey(ctx, iss.ID)
	if err != nil {
		return nil, err
	}
	defer memory.SecureZero(priv)

	revoked, err := svc.ListRevoked(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(revoked))
	entries := make([]revoke.Entry, 0, len(revoked))
	for _, r := range revoked {
		ids = append(ids, r.LicenseUUID)
		at := r.RevokedAt
		entries = append(entries, revoke.Entry{ID: r.LicenseUUID, Reason: r.Reason, RevokedAt: &at})
	}
	now := time.Now().UTC()
	list := revoke.List{
		Version:    1,
		KeyID:      iss.KeyID,
		Sequence:   uint64(now.Unix()),
		IssuedAt:   now,
		ExpiresAt:  now.Add(validFor),
		ServerTime: now,
		Revoked:    ids,
		Entries:    entries,
	}
	pem, err := revoke.Sign(list, priv)
	if err != nil {
		return nil, err
	}

	svc.cacheMu.Lock()
	svc.cachedPEM = pem
	svc.cachedUntil = now.Add(validFor / 2)
	svc.cacheMu.Unlock()

	out := make([]byte, len(pem))
	copy(out, pem)
	return out, nil
}
