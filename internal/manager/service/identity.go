package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	licenseent "github.com/oioio-space/maldev/internal/manager/store/ent/license"
	licensekg "github.com/oioio-space/maldev/license"
)

// IdentityService manages 32-byte identity blobs used to bind licences to
// a specific binary build.
type IdentityService struct {
	store *store.Store
	audit *AuditService
}

// NewIdentityService wires an IdentityService to the given store and audit sink.
func NewIdentityService(s *store.Store, a *AuditService) *IdentityService {
	return &IdentityService{store: s, audit: a}
}

// randIdentityBytes fills a fresh 32-byte array from the OS CSPRNG.
func randIdentityBytes() ([32]byte, error) {
	var b [32]byte
	_, err := rand.Read(b[:])
	return b, err
}

// Create generates 32 random bytes and persists them under name.
func (svc *IdentityService) Create(ctx context.Context, name, actor string) (*ent.Identity, error) {
	b, err := randIdentityBytes()
	if err != nil {
		return nil, err
	}
	return svc.insertIdentity(ctx, name, b[:], actor)
}

// Import registers existing bytes (must be exactly 32 bytes).
func (svc *IdentityService) Import(ctx context.Context, name string, bytes []byte, actor string) (*ent.Identity, error) {
	if len(bytes) != 32 {
		return nil, fmt.Errorf("identity: expected 32 bytes, got %d", len(bytes))
	}
	return svc.insertIdentity(ctx, name, bytes, actor)
}

func (svc *IdentityService) insertIdentity(ctx context.Context, name string, bytes []byte, actor string) (*ent.Identity, error) {
	sha := licensekg.HashIdentity(bytes)
	var row *ent.Identity
	err := withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
		var e error
		row, e = tx.Identity.Create().
			SetName(name).
			SetBytes(bytes).
			SetSha256(sha).
			Save(ctx)
		if e != nil {
			return e
		}
		return svc.audit.AppendTx(ctx, tx, "identity.create", actor,
			Target{Kind: "Identity", ID: row.ID.String()},
			map[string]any{"name": name, "sha256": sha})
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

// List returns all identity rows.
func (svc *IdentityService) List(ctx context.Context) ([]*ent.Identity, error) {
	return svc.store.Client.Identity.Query().All(ctx)
}

// Get retrieves a single identity by ID.
func (svc *IdentityService) Get(ctx context.Context, id uuid.UUID) (*ent.Identity, error) {
	return svc.store.Client.Identity.Get(ctx, id)
}

// ExportBin returns the raw 32 bytes ready for //go:embed.
func (svc *IdentityService) ExportBin(ctx context.Context, id uuid.UUID) ([]byte, error) {
	row, err := svc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(row.Bytes))
	copy(out, row.Bytes)
	return out, nil
}

// ErrNotConfirmed is returned by Regenerate when confirmed is false.
// Regenerate rotates the bytes of an existing identity, breaking every licence
// that pins it, so the caller must explicitly opt in.
var ErrNotConfirmed = errors.New("identity: regenerate requires explicit confirmation")

// Regenerate rotates the bytes (and sha256) of an existing identity.
// Returns ErrNotConfirmed if confirmed is false.
func (svc *IdentityService) Regenerate(ctx context.Context, id uuid.UUID, confirmed bool, actor string) error {
	if !confirmed {
		return ErrNotConfirmed
	}
	b, err := randIdentityBytes()
	if err != nil {
		return err
	}
	sha := licensekg.HashIdentity(b[:])

	return withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
		if _, err := tx.Identity.UpdateOneID(id).
			SetBytes(b[:]).
			SetSha256(sha).
			Save(ctx); err != nil {
			return err
		}
		return svc.audit.AppendTx(ctx, tx, "identity.regenerate", actor,
			Target{Kind: "Identity", ID: id.String()},
			map[string]any{"new_sha256": sha})
	})
}

// UsageCount returns the number of License rows whose IdentitySHA256 matches
// this identity's sha256.
func (svc *IdentityService) UsageCount(ctx context.Context, id uuid.UUID) (int, error) {
	row, err := svc.Get(ctx, id)
	if err != nil {
		return 0, err
	}
	return svc.store.Client.License.Query().
		Where(licenseent.IdentitySha256EQ(row.Sha256)).
		Count(ctx)
}

// Delete refuses if the identity is referenced by any licence.
func (svc *IdentityService) Delete(ctx context.Context, id uuid.UUID, actor string) error {
	count, err := svc.UsageCount(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("identity: %d licence(s) reference this identity", count)
	}
	return withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
		if err := tx.Identity.DeleteOneID(id).Exec(ctx); err != nil {
			return err
		}
		return svc.audit.AppendTx(ctx, tx, "identity.delete", actor,
			Target{Kind: "Identity", ID: id.String()}, nil)
	})
}
