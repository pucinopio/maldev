package tui_test

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/tui"
	"github.com/oioio-space/maldev/internal/manager/tui/wizard"
)

// --- Step 1: Identity snapshot -------------------------------------------

func TestWizardStep1IdentitySnapshot(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		root := tui.New(nil, nil, tui.SessionReady)
		m := initModel(root)
		// Navigate to licenses, press "n" to open wizard.
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
		// Inject empty identity list.
		m, _ = m.Update(wizard.IdentityLoadedMsg{Rows: nil, Err: nil})
		compareOrUpdate(t, "wizard_step1_empty", m.View())
	})
}

// --- Step 5: Validity snapshot -------------------------------------------

func TestWizardStep5ValiditySnapshot(t *testing.T) {
	root := tui.New(nil, nil, tui.SessionReady)
	m := initModel(root)
	// Open wizard.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m, _ = m.Update(wizard.IdentityLoadedMsg{})
	// Drive through steps 1-4 with skip/choose msgs.
	m, _ = m.Update(wizard.IdentityChosenMsg{IssuerID: "00000000-0000-0000-0000-000000000001"})
	m, _ = m.Update(wizard.RecipientLoadedMsg{})
	m, _ = m.Update(wizard.RecipientChosenMsg{RecipientID: ""})
	m, _ = m.Update(wizard.MachineBindingMsg{MachineID: ""})
	m, _ = m.Update(wizard.BinaryBindingMsg{})
	compareOrUpdate(t, "wizard_step5_validity", m.View())
}

// --- Step 8: Review snapshot ---------------------------------------------

func TestWizardStep8ReviewSnapshot(t *testing.T) {
	root := tui.New(nil, nil, tui.SessionReady)
	m := initModel(root)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m, _ = m.Update(wizard.IdentityLoadedMsg{})
	m, _ = m.Update(wizard.IdentityChosenMsg{IssuerID: "00000000-0000-0000-0000-000000000001"})
	m, _ = m.Update(wizard.RecipientLoadedMsg{})
	m, _ = m.Update(wizard.RecipientChosenMsg{})
	m, _ = m.Update(wizard.MachineBindingMsg{MachineID: "deadbeef"})
	m, _ = m.Update(wizard.BinaryBindingMsg{SHA256: "abc123", Size: 1024})
	m, _ = m.Update(wizard.ValidityMsg{
		NotBefore: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:  time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	m, _ = m.Update(wizard.FreeFieldsMsg{Fields: map[string]string{"env": "prod"}})
	m, _ = m.Update(wizard.TOTPSecretsLoadedMsg{})
	m, _ = m.Update(wizard.TOTPChoiceMsg{Require: false})
	compareOrUpdate(t, "wizard_step8_review", m.View())
}

// --- Probe drawer snapshot -----------------------------------------------

func TestProbeDrawerSnapshot(t *testing.T) {
	root := tui.New(nil, nil, tui.SessionReady)
	m := initModel(root)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m, _ = m.Update(wizard.IdentityLoadedMsg{})
	m, _ = m.Update(wizard.IdentityChosenMsg{IssuerID: "00000000-0000-0000-0000-000000000001"})
	m, _ = m.Update(wizard.RecipientLoadedMsg{})
	m, _ = m.Update(wizard.RecipientChosenMsg{})
	// Machine step: trigger probe drawer.
	m, _ = m.Update(wizard.OpenProbeDrawerMsg{})
	// Inject probe token issued (nil svc path returns error — snapshot the error state).
	m, _ = m.Update(tui.ProbeTokenIssuedMsg{Err: nil, Token: nil})
	compareOrUpdate(t, "probe_drawer", m.View())
}

// --- QR overlay snapshot -------------------------------------------------

func TestQROverlaySnapshot(t *testing.T) {
	// Drive the QR overlay directly (nil IssuedLicense = empty/cancel state).
	overlay := tui.NewQROverlay(nil)
	overlay.Init()
	overlay, _ = overlay.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	compareOrUpdate(t, "qr_overlay_empty", overlay.View())
}

// --- File picker snapshot ------------------------------------------------

func TestFilePickerSnapshot(t *testing.T) {
	root := tui.New(nil, nil, tui.SessionReady)
	m := initModel(root)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m, _ = m.Update(wizard.IdentityLoadedMsg{})
	m, _ = m.Update(wizard.IdentityChosenMsg{IssuerID: "00000000-0000-0000-0000-000000000001"})
	m, _ = m.Update(wizard.RecipientLoadedMsg{})
	m, _ = m.Update(wizard.RecipientChosenMsg{})
	m, _ = m.Update(wizard.MachineBindingMsg{})
	// Binary step: trigger file picker.
	m, _ = m.Update(wizard.OpenFilePickerMsg{Callback: "binary"})
	compareOrUpdate(t, "file_picker", m.View())
}
