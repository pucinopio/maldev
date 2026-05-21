// Command tui-snap renders a license-manager TUI frame to stdout as ANSI text.
// Pipe the output to charmbracelet/freeze to produce PNG/SVG screenshots.
//
// Usage:
//
//	go run ./cmd/tui-snap -view dashboard -width 144 -height 44 | freeze --output out.png
//
// Supported views:
//
//	dashboard, licenses, issuers, recipients, identities, revocation,
//	servers, audit, settings, onboarding, passphrase,
//	wizard, wizard-step<N>  (N=1..8)
//	overlay-confirm, overlay-error, overlay-quit, overlay-revoke,
//	overlay-input, overlay-qr, overlay-filepicker
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/tui"
	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
	"github.com/oioio-space/maldev/internal/manager/tui/wizard"
)

// seedData describes the in-memory state to inject via messages.
type seedData struct {
	// Counters
	Active       int `json:"active"`
	Revoked      int `json:"revoked"`
	Expired      int `json:"expired"`
	ExpiringSoon int `json:"expiring_soon"`
	Superseded   int `json:"superseded"`

	// Active issuer
	ActiveKeyID   string `json:"active_key_id"`
	ActiveKeyName string `json:"active_key_name"`
	ActiveKeyFP   string `json:"active_key_fingerprint"`

	// HTTP servers (up to 3: revocation/heartbeat/probe)
	Servers []seedServer `json:"servers"`

	// Recent audit events (up to 5)
	Audit []seedAudit `json:"audit"`
}

type seedServer struct {
	Name     string `json:"name"`
	On       bool   `json:"on"`
	URL      string `json:"url"`
	Requests uint64 `json:"requests"`
	Uptime   string `json:"uptime"`
}

type seedAudit struct {
	At     string `json:"at"` // RFC3339 or HH:MM:SS
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Actor  string `json:"actor"`
	Note   string `json:"note"`
}

func main() {
	var (
		viewFlag   = flag.String("view", "dashboard", "view to render: dashboard|licenses|issuers|recipients|identities|revocation|servers|audit|settings|onboarding|passphrase|wizard|wizard-step<N>|overlay-confirm|overlay-error|overlay-quit|overlay-revoke|overlay-input|overlay-qr|overlay-filepicker")
		widthFlag  = flag.Int("width", 144, "terminal width in cells")
		heightFlag = flag.Int("height", 44, "terminal height in cells")
		seedFlag   = flag.String("seed", "", "path to seed JSON file; empty = no data")
		keysFlag   = flag.String("keys", "", `space-separated key labels to send after seed, e.g. "1 d / esc"`)
		mouseFlag  = flag.String("mouse", "", `click to send after layout: "x,y" or "x,y,left|right"`)
	)
	flag.Parse()

	view := *viewFlag
	w := *widthFlag
	h := *heightFlag

	// ── Standalone overlay views ──────────────────────────────────────────────
	if ov := overlayForView(view, w, h); ov != nil {
		fmt.Print(ov.View())
		return
	}

	// ── Wizard standalone view ────────────────────────────────────────────────
	if view == "wizard" || strings.HasPrefix(view, "wizard-step") {
		fmt.Print(renderWizardView(view, w, h, *keysFlag))
		return
	}

	// ── Onboarding view ───────────────────────────────────────────────────────
	if view == "onboarding" || strings.HasPrefix(view, "onboarding-step") {
		fmt.Print(renderOnboardingView(view, w, h))
		return
	}

	// ── Standard root-model views ─────────────────────────────────────────────
	sd := loadSeed(*seedFlag)

	sess := tui.SessionReady
	switch view {
	case "passphrase":
		sess = tui.SessionLocked
	}

	root := tui.New(nil, nil, sess)

	var m tea.Model = root

	m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})

	viewKeyMap := map[string]rune{
		"dashboard":  '1',
		"licenses":   '2',
		"issuers":    '3',
		"recipients": '4',
		"identities": '5',
		"revocation": '6',
		"servers":    '7',
		"audit":      '8',
		"settings":   '9',
	}
	if r, ok := viewKeyMap[view]; ok && sess == tui.SessionReady {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if sd != nil && (view == "dashboard" || view == "") {
		snap := buildSnapshotMsg(sd)
		m, _ = m.Update(snap)
	}

	if *keysFlag != "" {
		for _, k := range strings.Fields(*keysFlag) {
			msg := keyMsgFromLabel(k)
			if msg != nil {
				m, _ = m.Update(msg)
			}
		}
	}

	if *mouseFlag != "" {
		if msg, ok := mouseFromFlag(*mouseFlag); ok {
			_ = m.View()
			m, _ = m.Update(msg)
		}
	}

	fmt.Print(m.View())
}

// overlayForView constructs and initialises a standalone overlay for rendering.
// Returns nil when view is not an overlay view.
func overlayForView(view string, w, h int) tui.Overlay {
	switch view {
	case "overlay-confirm":
		return tui.NewConfirmOverlay(
			"snap-confirm",
			"Confirmer l'opération ?",
			"Cette opération est irréversible. Confirmez-vous ?",
			"Confirmer", "Annuler",
			false,
		)
	case "overlay-confirm-danger":
		return tui.NewConfirmOverlay(
			"snap-danger",
			"Action destructrice",
			"La base de données sera supprimée définitivement.",
			"Supprimer", "Annuler",
			true,
		)
	case "overlay-error":
		return tui.NewErrorOverlay(
			"Échec de l'opération",
			"Impossible de contacter le service : connection refused (127.0.0.1:8443)",
		)
	case "overlay-quit":
		return tui.NewQuitOverlay(false)
	case "overlay-quit-servers":
		return tui.NewQuitOverlay(true)
	case "overlay-revoke":
		id, _ := uuid.Parse("a1b2c3d4-0000-0000-0000-000000000001")
		return tui.NewRevokeOverlay(id, "alice@research")
	case "overlay-input":
		return tui.NewInputOverlay("snap-input", "Nommer la ressource", "e.g. prod-2026-Q3", 80)
	case "overlay-qr":
		return tui.NewQROverlay(nil)
	case "overlay-filepicker":
		// newFilePickerOverlay reads the filesystem; it's unexported so we trigger
		// it through the root model by navigating to the binary wizard step.
		// Return nil here — handled via root model path in main().
		return nil
	}
	return nil
}

// renderWizardView renders the wizard at the specified step.
// view is "wizard" (step 1) or "wizard-step<N>" (N = 1..8).
func renderWizardView(view string, w, h int, keysStr string) string {
	step := 1
	if strings.HasPrefix(view, "wizard-step") {
		if n, err := strconv.Atoi(strings.TrimPrefix(view, "wizard-step")); err == nil && n >= 1 && n <= 8 {
			step = n
		}
	}

	m := tui.NewWizardSnap(w, h)

	// Drive wizard to the requested step by injecting the appropriate advancement
	// messages (same sequence used in wizard_test.go).
	switch {
	case step >= 2:
		m, _ = m.Update(wizard.IdentityLoadedMsg{})
		m, _ = m.Update(wizard.IdentityChosenMsg{IssuerID: "00000000-0000-0000-0000-000000000001"})
		fallthrough
	case step == 1:
		// step 1 is the default; IdentityLoadedMsg settles the empty list.
		if step == 1 {
			m, _ = m.Update(wizard.IdentityLoadedMsg{})
		}
	}

	if step >= 3 {
		m, _ = m.Update(wizard.RecipientLoadedMsg{})
		m, _ = m.Update(wizard.RecipientChosenMsg{RecipientID: ""})
	}
	if step >= 4 {
		m, _ = m.Update(wizard.MachineBindingMsg{MachineID: "deadbeef-0000-0000-0000-000000000000"})
	}
	if step >= 5 {
		m, _ = m.Update(wizard.BinaryBindingMsg{SHA256: "8b3c91ad2e1abc", Size: 7340032})
	}
	if step >= 6 {
		m, _ = m.Update(wizard.ValidityMsg{
			NotBefore: time.Date(2026, 5, 20, 13, 42, 18, 0, time.UTC),
			NotAfter:  time.Date(2026, 8, 18, 13, 42, 18, 0, time.UTC),
		})
	}
	if step >= 7 {
		m, _ = m.Update(wizard.FreeFieldsMsg{Fields: map[string]string{
			"tier":  "pro",
			"seats": "3",
			"note":  "trial pour partner",
		}})
	}
	if step >= 8 {
		m, _ = m.Update(wizard.TOTPSecretsLoadedMsg{})
		m, _ = m.Update(wizard.TOTPChoiceMsg{Require: false})
	}

	// Apply any additional key presses from -keys flag.
	if keysStr != "" {
		for _, k := range strings.Fields(keysStr) {
			if msg := keyMsgFromLabel(k); msg != nil {
				m, _ = m.Update(msg)
			}
		}
	}

	return m.View()
}

// renderOnboardingView renders the onboarding wizard at the specified step.
// view is "onboarding" (step 0 = welcome) or "onboarding-step<N>" (N = 0..3).
func renderOnboardingView(view string, w, h int) string {
	step := 0
	if strings.HasPrefix(view, "onboarding-step") {
		if n, err := strconv.Atoi(strings.TrimPrefix(view, "onboarding-step")); err == nil && n >= 0 && n <= 3 {
			step = n
		}
	}
	m := tui.NewOnboardingSnap(w, h, step)
	return m.View()
}

func loadSeed(path string) *seedData {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var sd seedData
	if err := json.NewDecoder(f).Decode(&sd); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "tui-snap: seed parse error: %v\n", err)
		return nil
	}
	return &sd
}

func buildSnapshotMsg(sd *seedData) cmds.DashboardSnapshotMsg {
	snap := cmds.DashboardSnapshotMsg{
		Active:               sd.Active,
		Revoked:              sd.Revoked,
		Expired:              sd.Expired,
		ExpiringSoon:         sd.ExpiringSoon,
		Superseded:           sd.Superseded,
		ActiveKeyID:          sd.ActiveKeyID,
		ActiveKeyName:        sd.ActiveKeyName,
		ActiveKeyFingerprint: sd.ActiveKeyFP,
	}
	for _, s := range sd.Servers {
		snap.Servers = append(snap.Servers, cmds.ServerStatus{
			Name:     s.Name,
			On:       s.On,
			URL:      s.URL,
			Requests: s.Requests,
			Uptime:   s.Uptime,
		})
	}
	now := time.Now()
	for _, a := range sd.Audit {
		at := now
		if a.At != "" {
			if t, err := time.Parse(time.RFC3339, a.At); err == nil {
				at = t
			} else if t, err := time.Parse("15:04:05", a.At); err == nil {
				at = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, now.Location())
			}
		}
		snap.RecentAudit = append(snap.RecentAudit, cmds.AuditEntry{
			At:       at,
			Kind:     a.Kind,
			TargetID: a.Target,
			Actor:    a.Actor,
			Note:     a.Note,
		})
	}
	return snap
}

// keyMsgFromLabel converts a label like "esc", "enter", "tab", "/" to tea.KeyMsg.
func keyMsgFromLabel(label string) tea.Msg {
	switch label {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		if len([]rune(label)) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(label)}
		}
	}
	return nil
}

// mouseFromFlag parses "x,y" or "x,y,left|right" into a tea.MouseMsg.
func mouseFromFlag(s string) (tea.MouseMsg, bool) {
	parts := strings.Split(s, ",")
	if len(parts) < 2 {
		return tea.MouseMsg{}, false
	}
	x, y := 0, 0
	fmt.Sscanf(parts[0], "%d", &x)
	fmt.Sscanf(parts[1], "%d", &y)
	btn := tea.MouseButtonLeft
	if len(parts) >= 3 && parts[2] == "right" {
		btn = tea.MouseButtonRight
	}
	return tea.MouseMsg{
		X:      x,
		Y:      y,
		Action: tea.MouseActionRelease,
		Button: btn,
		Type:   tea.MouseLeft,
	}, true
}
