package tui

// Overlay ID constants — used by both the screen that pushes the overlay
// and the app.go dispatcher that reads the ConfirmResultMsg / InputResultMsg
// it produces. Centralised here so a typo in one site fails to compile
// rather than silently breaking the round-trip.
const (
	OverlayIDSettingsRekey   = "settings-rekey"
	OverlayIDSettingsVacuum  = "settings-vacuum"
	OverlayIDSettingsBackup  = "settings-backup"
	OverlayIDServerRegenTok  = "server-regen-token"
	OverlayIDServerEditBind  = "server-edit-bind"
	OverlayIDAuditExportCSV  = "audit-export-csv"
	OverlayIDAuditExportJSON = "audit-export-json"

	OverlayIDTOTPExportPDF    = "totp-export-pdf"

	OverlayIDLicenseReissue   = "license-reissue"
	OverlayIDLicenseExport    = "license-export"
	OverlayIDIssuerRename     = "issuer-rename"
	OverlayIDIssuerExportPub  = "issuer-export-pub"
	OverlayIDIssuerExportPriv     = "issuer-export-priv"      // confirm step
	OverlayIDIssuerExportPrivPath = "issuer-export-priv-path" // path-input step
	OverlayIDIssuerRetire     = "issuer-retire"
	OverlayIDRecipientRename  = "recipient-rename"
	OverlayIDRecipientDelete  = "recipient-delete"
	OverlayIDIdentityRename   = "identity-rename"
	OverlayIDIdentityRegen    = "identity-regen"
	OverlayIDIdentityDelete   = "identity-delete"
)
