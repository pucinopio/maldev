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
)
