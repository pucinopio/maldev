package service

import (
	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/store"
)

// Services bundles every domain service the manager exposes to the TUI.
// Construct via New, dispose via Close on shutdown.
type Services struct {
	Store *store.Store
	KEK   *crypto.KEK

	Audit     *AuditService
	Settings  *SettingsService
	Issuer    *IssuerService
	Identity  *IdentityService
	Recipient *RecipientService
	TOTP      *TOTPService
	Probe     *ProbeService
	License   *LicenseService
	Revoke    *RevokeService
}

// New wires every service against a single store + KEK. Callers should not
// instantiate individual services directly.
func New(s *store.Store, k *crypto.KEK) *Services {
	audit := NewAuditService(s)
	settings := NewSettingsService(s, k)
	issuer := NewIssuerService(s, k, audit)
	identity := NewIdentityService(s, audit)
	recipient := NewRecipientService(s, k, audit)
	totp := NewTOTPService(s, k)
	probe := NewProbeService(s, audit)
	license := NewLicenseService(s, k, audit, issuer, identity, recipient, totp)
	revoke := NewRevokeService(s, audit, issuer, license)
	return &Services{
		Store: s, KEK: k,
		Audit: audit, Settings: settings, Issuer: issuer, Identity: identity,
		Recipient: recipient, TOTP: totp, Probe: probe, License: license,
		Revoke: revoke,
	}
}

// Close wipes the KEK and closes the underlying store. Always call on
// clean shutdown so the in-memory key does not survive a heap dump.
func (s *Services) Close() error {
	if s.KEK != nil {
		s.KEK.Wipe()
	}
	return s.Store.Close()
}
