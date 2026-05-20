package service

import (
	"bytes"
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	"github.com/oioio-space/maldev/license/seal"
)

type RecipientService struct {
	store *store.Store
	kek   *crypto.KEK
	audit *AuditService
}

func NewRecipientService(s *store.Store, k *crypto.KEK, a *AuditService) *RecipientService {
	return &RecipientService{store: s, kek: k, audit: a}
}

// Generate creates a fresh X25519 keypair and persists it with the priv
// half encrypted under the KEK.
func (svc *RecipientService) Generate(ctx context.Context, name, actor string) (*ent.RecipientKey, error) {
	pub, priv, err := seal.GenerateRecipient()
	if err != nil {
		return nil, err
	}
	wrapped, err := svc.kek.Wrap(priv)
	if err != nil {
		return nil, err
	}
	// Wipe the priv copy on stack — caller never sees it.
	for i := range priv {
		priv[i] = 0
	}

	tx, err := svc.store.Client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	row, err := tx.RecipientKey.Create().
		SetName(name).
		SetPublicKey(pub).
		SetEncryptedPriv(wrapped).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := svc.audit.AppendTx(ctx, tx, "recipient.generate", actor,
		Target{Kind: "RecipientKey", ID: row.ID.String()},
		map[string]any{"name": name}); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return row, nil
}

// Import accepts raw 32-byte pub + priv from an external source (e.g.
// migration from another keystore).
func (svc *RecipientService) Import(ctx context.Context, name string, pub, priv []byte, actor string) (*ent.RecipientKey, error) {
	if len(pub) != 32 || len(priv) != 32 {
		return nil, fmt.Errorf("recipient: pub and priv must be 32 bytes each")
	}
	wrapped, err := svc.kek.Wrap(priv)
	if err != nil {
		return nil, err
	}
	tx, err := svc.store.Client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	row, err := tx.RecipientKey.Create().
		SetName(name).
		SetPublicKey(pub).
		SetEncryptedPriv(wrapped).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := svc.audit.AppendTx(ctx, tx, "recipient.import", actor,
		Target{Kind: "RecipientKey", ID: row.ID.String()},
		map[string]any{"name": name}); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}

func (svc *RecipientService) List(ctx context.Context) ([]*ent.RecipientKey, error) {
	return svc.store.Client.RecipientKey.Query().All(ctx)
}

func (svc *RecipientService) Get(ctx context.Context, id uuid.UUID) (*ent.RecipientKey, error) {
	return svc.store.Client.RecipientKey.Get(ctx, id)
}

// ExportPublic returns the 32-byte public key (raw, callers wrap in PEM if
// they need it).
func (svc *RecipientService) ExportPublic(ctx context.Context, id uuid.UUID) ([]byte, error) {
	row, err := svc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(row.PublicKey))
	copy(out, row.PublicKey)
	return out, nil
}

// PrivateKey returns the decrypted X25519 private key. Caller's
// responsibility to clear after use.
func (svc *RecipientService) PrivateKey(ctx context.Context, id uuid.UUID) ([]byte, error) {
	row, err := svc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return svc.kek.Unwrap(row.EncryptedPriv)
}

func (svc *RecipientService) Delete(ctx context.Context, id uuid.UUID, actor string) error {
	tx, err := svc.store.Client.Tx(ctx)
	if err != nil {
		return err
	}
	if err := tx.RecipientKey.DeleteOneID(id).Exec(ctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := svc.audit.AppendTx(ctx, tx, "recipient.delete", actor,
		Target{Kind: "RecipientKey", ID: id.String()}, nil); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// suppress unused
var _ = bytes.Equal
