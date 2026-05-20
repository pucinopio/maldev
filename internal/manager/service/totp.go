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
