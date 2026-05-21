// Command tui-snap renders a license-manager TUI frame to stdout as ANSI text.
// Pipe the output to charmbracelet/freeze to produce PNG/SVG screenshots.
//
// Usage:
//
//	go run ./cmd/tui-snap -view dashboard -width 144 -height 44 | freeze --output out.png
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/tui"
	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
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
		viewFlag   = flag.String("view", "dashboard", "view to render: dashboard|licenses|issuers|recipients|identities|revocation|servers|audit|settings|onboarding|passphrase")
		widthFlag  = flag.Int("width", 144, "terminal width in cells")
		heightFlag = flag.Int("height", 44, "terminal height in cells")
		seedFlag   = flag.String("seed", "", "path to seed JSON file; empty = no data")
		keysFlag   = flag.String("keys", "", `space-separated key labels to send after seed, e.g. "1 d / esc"`)
		mouseFlag  = flag.String("mouse", "", `click to send after layout: "x,y" or "x,y,left|right"`)
	)
	flag.Parse()

	sd := loadSeed(*seedFlag)

	// Build the session state based on requested view.
	sess := tui.SessionReady
	switch *viewFlag {
	case "onboarding":
		sess = tui.SessionOnboarding
	case "passphrase":
		sess = tui.SessionLocked
	}

	root := tui.New(nil, nil, sess)

	var m tea.Model = root

	// Apply window size first so layout is valid.
	m, _ = m.Update(tea.WindowSizeMsg{Width: *widthFlag, Height: *heightFlag})

	// Navigate to the requested view via key press (1-9).
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
	if r, ok := viewKeyMap[*viewFlag]; ok && sess == tui.SessionReady {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if sd != nil && (*viewFlag == "dashboard" || *viewFlag == "") {
		snap := buildSnapshotMsg(sd)
		m, _ = m.Update(snap)
	}

	// Apply additional key presses from -keys flag.
	if *keysFlag != "" {
		for _, k := range strings.Fields(*keysFlag) {
			msg := keyMsgFromLabel(k)
			if msg != nil {
				m, _ = m.Update(msg)
			}
		}
	}

	// Apply mouse click from -mouse flag.
	if *mouseFlag != "" {
		if msg, ok := mouseFromFlag(*mouseFlag); ok {
			_ = m.View() // trigger layout before click
			m, _ = m.Update(msg)
		}
	}

	// Emit the rendered view to stdout.
	fmt.Print(m.View())
}

func loadSeed(path string) *seedData {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		// Seed file optional — silently return nil.
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
// Returns (msg, true) on success, (zero, false) when s is malformed.
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
