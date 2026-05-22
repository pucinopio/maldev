package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	"github.com/oioio-space/maldev/license/totp"
)

// TOTPService manages TOTP secrets linked to licences. Secret creation happens
// inside LicenseService.Issue (T14) via createForLicenseTx; Get provides the
// decrypted view for the manager UI.
type TOTPService struct {
	store *store.Store
	kek   *crypto.KEK
}

// NewTOTPService wires a TOTPService to the given store and KEK.
func NewTOTPService(s *store.Store, k *crypto.KEK) *TOTPService {
	return &TOTPService{store: s, kek: k}
}

// TOTPSecretView is the decrypted view returned to the UI, with pre-rendered
// provisioning QR artefacts ready for display.
type TOTPSecretView struct {
	Secret       string
	AccountLabel string
	OtpauthURI   string
	QRImageASCII string
	QRImagePNG   []byte
}

// Get returns the decrypted TOTP secret for a licence together with QR
// artefacts. Returns an error if no TOTP secret has been issued yet.
func (svc *TOTPService) Get(ctx context.Context, licenseID uuid.UUID, issuerName string) (*TOTPSecretView, error) {
	lic, err := svc.store.Client.License.Get(ctx, licenseID)
	if err != nil {
		return nil, err
	}
	rows, err := lic.QueryTotps().All(ctx)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("totp_secret not found for license %s", licenseID)
	}
	r := rows[0]
	secret, err := svc.kek.Unwrap(r.EncryptedSecret)
	if err != nil {
		return nil, err
	}
	uri := totp.URI(string(secret), r.AccountLabel, issuerName)
	ascii, _ := totp.QRImageASCII(string(secret), r.AccountLabel, issuerName)
	png, _ := totp.QRImagePNG(string(secret), r.AccountLabel, issuerName, 256)
	return &TOTPSecretView{
		Secret:       string(secret),
		AccountLabel: r.AccountLabel,
		OtpauthURI:   uri,
		QRImageASCII: ascii,
		QRImagePNG:   png,
	}, nil
}

// List returns every TOTPSecret row (linked + standalone), most-recent first.
// Used by the manager UI's TOTP management tab.
func (svc *TOTPService) List(ctx context.Context) ([]*ent.TOTPSecret, error) {
	return svc.store.Client.TOTPSecret.Query().All(ctx)
}

// Generate creates a fresh standalone TOTP secret (no licence binding) with
// the given account label and persists it KEK-wrapped. Returns the persisted
// row + the cleartext secret so the caller can display the provisioning QR
// exactly once.
func (svc *TOTPService) Generate(ctx context.Context, accountLabel string) (*ent.TOTPSecret, string, error) {
	if svc.kek == nil {
		return nil, "", fmt.Errorf("kek not initialised")
	}
	secret, err := totp.NewSecret()
	if err != nil {
		return nil, "", fmt.Errorf("new secret: %w", err)
	}
	wrapped, err := svc.kek.Wrap([]byte(secret))
	if err != nil {
		return nil, "", fmt.Errorf("wrap: %w", err)
	}
	row, err := svc.store.Client.TOTPSecret.Create().
		SetEncryptedSecret(wrapped).
		SetAccountLabel(accountLabel).
		Save(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("save: %w", err)
	}
	return row, string(secret), nil
}

// Delete removes the TOTP secret with the given ID. Safe to call on a secret
// that is currently bound to a licence — the licence keeps a reference but
// QueryTotps will return zero rows afterwards.
func (svc *TOTPService) Delete(ctx context.Context, id uuid.UUID) error {
	return svc.store.Client.TOTPSecret.DeleteOneID(id).Exec(ctx)
}

// GetByID returns the decrypted view of a TOTPSecret by its own ID (rather
// than the parent licence ID like Get). issuerName is used when rendering the
// otpauth:// URI + QR artefacts.
func (svc *TOTPService) GetByID(ctx context.Context, id uuid.UUID, issuerName string) (*TOTPSecretView, error) {
	r, err := svc.store.Client.TOTPSecret.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	secret, err := svc.kek.Unwrap(r.EncryptedSecret)
	if err != nil {
		return nil, err
	}
	uri := totp.URI(string(secret), r.AccountLabel, issuerName)
	ascii, _ := totp.QRImageASCII(string(secret), r.AccountLabel, issuerName)
	png, _ := totp.QRImagePNG(string(secret), r.AccountLabel, issuerName, 256)
	return &TOTPSecretView{
		Secret:       string(secret),
		AccountLabel: r.AccountLabel,
		OtpauthURI:   uri,
		QRImageASCII: ascii,
		QRImagePNG:   png,
	}, nil
}

// createForLicenseTx generates a new TOTP secret and persists it linked to
// the licence inside the caller's transaction. Returns the cleartext secret
// so the caller can embed it in the licence payload via license.BindTOTP.
func (svc *TOTPService) createForLicenseTx(ctx context.Context, tx *ent.Tx, licenseID uuid.UUID, accountLabel string) (string, error) {
	secret, err := totp.NewSecret()
	if err != nil {
		return "", err
	}
	wrapped, err := svc.kek.Wrap([]byte(secret))
	if err != nil {
		return "", err
	}
	_, err = tx.TOTPSecret.Create().
		SetEncryptedSecret(wrapped).
		SetAccountLabel(accountLabel).
		SetLicenseID(licenseID).
		Save(ctx)
	if err != nil {
		return "", err
	}
	return secret, nil
}
