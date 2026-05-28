package service

import (
	"context"
	"crypto/ed25519"
	"fmt"

	"github.com/google/uuid"

	"github.com/oioio-space/maldev/cleanup/memory"
	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	"github.com/oioio-space/maldev/internal/manager/store/ent/issuer"
	"github.com/oioio-space/maldev/license"
)

// IssuerService manages signing-key issuers: creation, import, activation and
// private-key retrieval. Private keys are always stored encrypted under the
// KEK; only the in-memory methods (PrivateKey) expose the plaintext.
type IssuerService struct {
	store *store.Store
	kek   *crypto.KEK
	audit *AuditService
}

// NewIssuerService wires an IssuerService to the given store, KEK and audit
// sink.
func NewIssuerService(s *store.Store, k *crypto.KEK, a *AuditService) *IssuerService {
	return &IssuerService{store: s, kek: k, audit: a}
}

// Generate creates a fresh Ed25519 keypair and persists it with the private
// key encrypted under the KEK. The new issuer is NOT marked active
// automatically — caller decides via SetActive.
func (svc *IssuerService) Generate(ctx context.Context, name, keyID, actor string) (*ent.Issuer, error) {
	pub, priv, err := license.GenerateKey()
	if err != nil {
		return nil, err
	}
	wrapped, err := svc.kek.Wrap(priv)
	memory.SecureZero(priv) // zeroize in-place; wrapped blob is the durable copy
	if err != nil {
		return nil, err
	}

	var row *ent.Issuer
	err = withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
		var e error
		row, e = tx.Issuer.Create().
			SetName(name).
			SetKeyID(keyID).
			SetPublicKey(pub).
			SetEncryptedPriv(wrapped).
			SetActive(false).
			Save(ctx)
		if e != nil {
			return e
		}
		return svc.audit.AppendTx(ctx, tx, "issuer.generate", actor,
			Target{Kind: "Issuer", ID: row.ID.String()},
			map[string]any{"name": name, "key_id": keyID})
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

// Import accepts a PEM-encoded MALDEV PRIVATE KEY block (as emitted by
// license.MarshalPrivateKey) and registers it as a new Issuer. The public
// half is derived via ed25519.PrivateKey.Public().
func (svc *IssuerService) Import(ctx context.Context, name, keyID string, privatePEM []byte, actor string) (*ent.Issuer, error) {
	priv, err := license.ParsePrivateKey(privatePEM)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	pub := ed25519.PrivateKey(priv).Public().(ed25519.PublicKey)
	wrapped, err := svc.kek.Wrap(priv)
	memory.SecureZero(priv) // zeroize before any early return
	if err != nil {
		return nil, err
	}

	var row *ent.Issuer
	err = withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
		var e error
		row, e = tx.Issuer.Create().
			SetName(name).
			SetKeyID(keyID).
			SetPublicKey(pub).
			SetEncryptedPriv(wrapped).
			SetActive(false).
			Save(ctx)
		if e != nil {
			return e
		}
		return svc.audit.AppendTx(ctx, tx, "issuer.import", actor,
			Target{Kind: "Issuer", ID: row.ID.String()},
			map[string]any{"name": name, "key_id": keyID})
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

// SetActive marks id active and unsets all other issuers, in one tx.
func (svc *IssuerService) SetActive(ctx context.Context, id uuid.UUID, actor string) error {
	return withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
		if _, err := tx.Issuer.Update().SetActive(false).Save(ctx); err != nil {
			return err
		}
		if _, err := tx.Issuer.UpdateOneID(id).SetActive(true).Save(ctx); err != nil {
			return err
		}
		return svc.audit.AppendTx(ctx, tx, "issuer.set_active", actor,
			Target{Kind: "Issuer", ID: id.String()}, nil)
	})
}

// Active returns the currently active issuer, or an error if none is set.
func (svc *IssuerService) Active(ctx context.Context) (*ent.Issuer, error) {
	return svc.store.Client.Issuer.Query().Where(issuer.ActiveEQ(true)).First(ctx)
}

// List returns all issuers.
func (svc *IssuerService) List(ctx context.Context) ([]*ent.Issuer, error) {
	return svc.store.Client.Issuer.Query().All(ctx)
}

// Get returns a single issuer by its UUID.
func (svc *IssuerService) Get(ctx context.Context, id uuid.UUID) (*ent.Issuer, error) {
	return svc.store.Client.Issuer.Get(ctx, id)
}

// PrivateKey returns the decrypted Ed25519 private key for in-memory use.
// Caller must take care not to leak it.
func (svc *IssuerService) PrivateKey(ctx context.Context, id uuid.UUID) ([]byte, error) {
	row, err := svc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return svc.kek.Unwrap(row.EncryptedPriv)
}

// Delete hard-removes an issuer row after refusing if any licence still
// references it. The encrypted private key buffer is securely zeroed in
// memory before the DB row is dropped. Refusal is the safe default because
// removing the issuer also makes every signed licence un-verifiable on
// re-import (the matching public key is gone). Operators wanting to retire
// an issuer without breaking outstanding licences should use Retire (status
// flip, no row deletion) — Delete is reserved for issuers that never signed
// anything or for cleanup after every dependent licence was deleted first.
func (svc *IssuerService) Delete(ctx context.Context, id uuid.UUID, actor string) error {
	row, err := svc.store.Client.Issuer.Get(ctx, id)
	if err != nil {
		return err
	}
	licCount, err := svc.store.Client.Issuer.QueryLicenses(row).Count(ctx)
	if err != nil {
		return fmt.Errorf("count licences: %w", err)
	}
	if licCount > 0 {
		return fmt.Errorf("issuer %q still has %d licence(s) — delete them first or use Retire",
			row.Name, licCount)
	}
	return withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
		// Wipe the wrapped key bytes in-place before drop so the bytes are
		// zeroed across the deleted page even if the DB never reuses it.
		memory.SecureZero(row.EncryptedPriv)
		if err := tx.Issuer.DeleteOneID(id).Exec(ctx); err != nil {
			return fmt.Errorf("delete issuer: %w", err)
		}
		return svc.audit.AppendTx(ctx, tx, "issuer.delete", actor,
			Target{Kind: "Issuer", ID: id.String()},
			map[string]any{"name": row.Name, "key_id": row.KeyID})
	})
}

// ExportPublic returns the public key as a PEM "MALDEV PUBLIC KEY" with the
// KID header populated.
func (svc *IssuerService) ExportPublic(ctx context.Context, id uuid.UUID) ([]byte, error) {
	row, err := svc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return license.MarshalPublicKey(row.PublicKey, row.KeyID)
}

// ExportPrivate returns the decrypted Ed25519 signing key as a PEM
// "MALDEV PRIVATE KEY" block, symmetrical to ExportPublic. Callers MUST treat
// the returned bytes as sensitive: this is the bytes the Import() path
// accepts to register a foreign issuer key, so anyone who obtains them can
// sign new licences. The slice returned by MarshalPrivateKey is a copy of
// the in-memory key, so the operator writes it to disk without retaining the
// raw key in process memory longer than necessary — memory.SecureZero is
// applied to the intermediate private-key buffer before returning.
func (svc *IssuerService) ExportPrivate(ctx context.Context, id uuid.UUID) ([]byte, error) {
	priv, err := svc.PrivateKey(ctx, id)
	if err != nil {
		return nil, err
	}
	pemBytes, marshalErr := license.MarshalPrivateKey(priv)
	memory.SecureZero(priv)
	if marshalErr != nil {
		return nil, marshalErr
	}
	return pemBytes, nil
}
