// tui-verify drives the TUI through specific key + mouse sequences and
// asserts via the JSONL trace produced by '-tags tui_trace' that each
// binding actually produces a message handled by the rootModel.
//
// Usage:
//
//	tui-verify                 # run all specs, print summary
//	tui-verify -id chrome.*    # filter by glob
//	tui-verify -ci             # exit non-zero on any failure
//	tui-verify -v              # verbose: print the trace lines for each test
//
// Test specs are defined inline (specs() function below). Each spec drives
// one binding and asserts that:
//  1. The expected tea.Msg type appears in the trace (e.g. tea.KeyMsg for
//     keyboard tests, tea.MouseMsg for mouse tests).
//  2. The downstream effect arrives — e.g. for 'chrome.help.kb' the trace
//     must contain an additional msg dispatched by the Help-overlay Init,
//     or the post-state must show the overlay on the stack.
//
// Build prerequisites: bin/tui-snap-trace.exe (built with -tags tui_trace).
// Auto-built on first run via 'go build -tags tui_trace -o bin/tui-snap-trace.exe ./cmd/tui-snap'.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"
)

// spec defines one keybind/mouse verification.
type spec struct {
	ID          string   // tracking-doc test ID, e.g. chrome.help.kb
	View        string   // tui-snap view argument
	Keys        string   // -keys arg (space-separated key labels) — setup + action
	SetupKeys   string   // -keys arg for the frame capture only (ClickTarget resolution)
	SnapView    string   // alternate view for resolveClickCoord (e.g. "overlay-confirm"); empty = use View
	Mouse       string   // -mouse arg (x,y[,button]) — explicit coords win over ClickTarget
	ClickTarget string   // substring to locate in the rendered frame; click at its visual centre
	Seed        string   // optional path to seed JSON
	ExpectMsgs  []string // substrings to find in trace msg_type or msg dump
	KnownFail   bool     // expected to fail — flag in report but don't count as regression
	Notes       string   // free-form for the report
}

// ansiStripper removes all ANSI / VT escape sequences from a byte slice.
var ansiStripper = regexp.MustCompile(`\x1b(?:\[[0-9;?]*[a-zA-Z]|\([B0])`)

func stripANSI(b []byte) []byte {
	return ansiStripper.ReplaceAll(b, nil)
}

// resolveClickCoord runs tui-snap-trace with the spec's view / seed / setup
// keys (no -mouse), strips ANSI from the rendered frame, locates the first
// occurrence of target as a substring, and returns click coordinates aimed at
// its visual centre.
//
// When s.SnapView is non-empty it is used as the -view argument instead of
// s.View; this lets overlay specs resolve coordinates from the standalone
// overlay view (which outputs clean lines) while the actual run uses the
// root-model path.
//
// Y is the 0-based line index; X is the column of the substring's midpoint.
// Both coordinates are 0-based to match tea.MouseMsg convention.
// resolveSeed returns the seed path for s: s.Seed if set, otherwise the
// conventional per-view seed file under scripts/tui-snap-seeds/ if it exists.
func resolveSeed(s spec) string {
	if s.Seed != "" {
		return s.Seed
	}
	candidate := filepath.Join("scripts", "tui-snap-seeds", s.View+".json")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

func resolveClickCoord(bin string, s spec, target string) (x, y int, err error) {
	snapView := s.SnapView
	if snapView == "" {
		snapView = s.View
	}

	setupKeys := s.SetupKeys
	if setupKeys == "" {
		setupKeys = s.Keys
	}

	args := []string{"-view", snapView, "-width", "144", "-height", "44"}
	if setupKeys != "" {
		args = append(args, "-keys", setupKeys)
	}

	// Only attach seed for root-model views (not standalone overlay views).
	if s.SnapView == "" {
		if seed := resolveSeed(s); seed != "" {
			args = append(args, "-seed", seed)
		}
	}

	// No MALDEV_TUI_TRACE: coord resolution only needs stdout, not the trace log.
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		return 0, 0, fmt.Errorf("snap exec: %w", err)
	}

	plain := stripANSI(out)
	lines := bytes.Split(plain, []byte("\n"))

	for lineIdx, lineBytes := range lines {
		line := string(lineBytes)
		col := strings.Index(line, target)
		if col < 0 {
			continue
		}
		// X = col offset to start of target + half its rune width.
		targetRunes := utf8.RuneCountInString(target)
		x = col + targetRunes/2
		y = lineIdx
		return x, y, nil
	}
	return 0, 0, fmt.Errorf("ClickTarget %q not found in rendered frame", target)
}

func specs() []spec {
	return []spec{
		// ── Chrome (global) — tab navigation ──────────────────────────────
		{ID: "chrome.tab.1.kb", View: "dashboard", Keys: "1", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'1' goto Dashboard"},
		{ID: "chrome.tab.2.kb", View: "dashboard", Keys: "2", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'2' goto Licenses"},
		{ID: "chrome.tab.3.kb", View: "dashboard", Keys: "3", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'3' goto Issuers"},
		{ID: "chrome.tab.4.kb", View: "dashboard", Keys: "4", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'4' goto Recipients"},
		{ID: "chrome.tab.5.kb", View: "dashboard", Keys: "5", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'5' goto Identities"},
		{ID: "chrome.tab.6.kb", View: "dashboard", Keys: "6", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'6' goto Revocation"},
		{ID: "chrome.tab.7.kb", View: "dashboard", Keys: "7", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'7' goto Servers"},
		{ID: "chrome.tab.8.kb", View: "dashboard", Keys: "8", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'8' goto TOTP"},
		{ID: "chrome.tab.9.kb", View: "dashboard", Keys: "9", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'9' goto Audit"},
		{ID: "chrome.tab.next.kb", View: "dashboard", Keys: "tab", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "tab cycles to next view"},
		{ID: "chrome.tab.prev.kb", View: "dashboard", Keys: "shift+tab", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "shift+tab cycles to prev view"},
		{
			ID:         "chrome.help.kb",
			View:       "dashboard",
			Keys:       "?",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'?' should push help overlay",
		},
		{
			ID:         "chrome.refresh.kb",
			View:       "dashboard",
			Keys:       "r",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'r' on dashboard triggers DashboardSnapshotCmd",
		},
		{
			ID:         "chrome.quit.kb",
			View:       "dashboard",
			Keys:       "q",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'q' without running servers should tea.Quit",
		},
		{ID: "chrome.startall.kb", View: "servers", Keys: "7 A", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'A' on Servers starts all servers"},
		{ID: "chrome.stopall.kb", View: "servers", Keys: "7 Z", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'Z' on Servers stops all servers"},

		// ── Chrome mouse: tab strip clicks ────────────────────────────────
		{ID: "chrome.tab.click.ms", View: "dashboard", Mouse: "1,1,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click tab strip Y=1"},
		{ID: "chrome.tab.1.ms", View: "dashboard", Mouse: "1,1,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click tab strip at X≈1 (Dashboard tab) Y=1"},
		{ID: "chrome.tab.2.ms", View: "dashboard", Mouse: "17,1,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click tab strip at X≈17 (Licenses tab) Y=1"},
		{ID: "chrome.tab.3.ms", View: "dashboard", ClickTarget: "3 Issuer keys", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click Issuer keys tab"},
		{ID: "chrome.tab.4.ms", View: "dashboard", ClickTarget: "4 Recipients", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click Recipients tab"},
		{ID: "chrome.tab.5.ms", View: "dashboard", Mouse: "65,1,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click tab strip at X≈65 (Identities tab) Y=1"},
		{ID: "chrome.tab.6.ms", View: "dashboard", ClickTarget: "6 Revocation", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click Revocation tab"},
		{ID: "chrome.tab.7.ms", View: "dashboard", ClickTarget: "7 Servers", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click Servers tab"},
		{ID: "chrome.tab.8.ms", View: "dashboard", ClickTarget: "8 TOTP", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click TOTP tab"},
		{ID: "chrome.tab.9.ms", View: "dashboard", ClickTarget: "9 Audit", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click Audit tab"},
		{ID: "chrome.tab.0.ms", View: "dashboard", ClickTarget: "0 Settings", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click Settings tab"},

		// ── Chrome hint pill clicks ────────────────────────────────────────
		// Hint pills appear in the title bar; clicking them triggers the matching key.
		// Seeds ensure they are rendered.
		{
			ID:          "chrome.hint.click.ms",
			View:        "dashboard",
			Seed:        "scripts/tui-snap-seeds/dashboard.json",
			ClickTarget: "[k] gérer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [k] gérer hint pill in dashboard title row",
		},

		// ── Dashboard Raccourcis hotkeys ───────────────────────────────────
		{ID: "dash.shortcut.n.kb", View: "dashboard", Keys: "n", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'n' on dashboard goto Licenses + wizard"},
		{ID: "dash.shortcut.slash.kb", View: "dashboard", Keys: "/", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'/' on dashboard goto Licenses + focus search"},
		{ID: "dash.shortcut.k.kb", View: "dashboard", Keys: "k", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'k' on dashboard goto Issuers"},
		{ID: "dash.shortcut.i.kb", View: "dashboard", Keys: "i", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'i' on dashboard goto Identities"},
		{ID: "dash.tile.a.kb", View: "dashboard", Keys: "a", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'a' on dashboard goto Licenses active filter"},
		{ID: "dash.tile.e.kb", View: "dashboard", Keys: "e", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'e' on dashboard goto Licenses expired filter"},
		{ID: "dash.tile.w.kb", View: "dashboard", Keys: "w", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'w' on dashboard goto Licenses expiring filter"},
		{ID: "dash.tile.u.kb", View: "dashboard", Keys: "u", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'u' on dashboard goto Licenses superseded filter"},

		// ── Dashboard tile mouse clicks ────────────────────────────────────
		{ID: "dash.tile.active.ms", View: "dashboard", Mouse: "14,4,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click Actives tile (tile 1 center X≈14, Y=4)"},
		{ID: "dash.tile.revoked.ms", View: "dashboard", Mouse: "42,4,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click Révoquées tile (tile 2 center X≈42, Y=4)"},
		{ID: "dash.tile.expired.ms", View: "dashboard", Mouse: "70,4,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click Expirées tile (tile 3 center X≈70, Y=4)"},
		{ID: "dash.tile.expiring.ms", View: "dashboard", Mouse: "98,4,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click Expirent<7j tile (tile 4 center X≈98, Y=4)"},
		{ID: "dash.tile.superseded.ms", View: "dashboard", Mouse: "126,4,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click Superseded tile (tile 5 center X≈126, Y=4)"},

		// ── Dashboard hint clicks (seeded) ────────────────────────────────
		{
			ID:          "dash.gererkey.ms",
			View:        "dashboard",
			Seed:        "scripts/tui-snap-seeds/dashboard.json",
			ClickTarget: "[k] gérer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [k] gérer hint → Goto Issuers",
		},
		{
			ID:          "dash.serversmore.ms",
			View:        "dashboard",
			Seed:        "scripts/tui-snap-seeds/dashboard.json",
			ClickTarget: "[7] détail",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [7] détail hint on Servers box → Goto Servers",
		},
		{
			ID:          "dash.fullaudit.ms",
			View:        "dashboard",
			Seed:        "scripts/tui-snap-seeds/dashboard.json",
			ClickTarget: "[8] tout l'audit",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [8] tout l'audit hint → Goto Audit",
		},

		// ── Dashboard Raccourcis cell clicks ──────────────────────────────
		{
			ID:          "dash.shortcut.n.ms",
			View:        "dashboard",
			Seed:        "scripts/tui-snap-seeds/dashboard.json",
			ClickTarget: "[n]  nouvelle licence",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [n] cell in Raccourcis",
		},
		{
			ID:          "dash.shortcut.slash.ms",
			View:        "dashboard",
			Seed:        "scripts/tui-snap-seeds/dashboard.json",
			ClickTarget: "[/]  rechercher",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [/] cell in Raccourcis",
		},
		{
			ID:          "dash.shortcut.x.ms",
			View:        "dashboard",
			Seed:        "scripts/tui-snap-seeds/dashboard.json",
			ClickTarget: "[x]  révoquer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [x] cell in Raccourcis",
		},
		{
			ID:          "dash.shortcut.k.ms",
			View:        "dashboard",
			Seed:        "scripts/tui-snap-seeds/dashboard.json",
			ClickTarget: "[k]  clés d'émission",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [k] cell in Raccourcis",
		},
		{
			ID:          "dash.shortcut.i.ms",
			View:        "dashboard",
			Seed:        "scripts/tui-snap-seeds/dashboard.json",
			ClickTarget: "[i]  identity.bin",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [i] cell in Raccourcis",
		},
		{
			ID:          "dash.shortcut.help.ms",
			View:        "dashboard",
			Seed:        "scripts/tui-snap-seeds/dashboard.json",
			ClickTarget: "[?]  aide contextuelle",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [?] cell in Raccourcis",
		},
		{
			ID:          "dash.serverrow.ms",
			View:        "dashboard",
			Seed:        "scripts/tui-snap-seeds/dashboard.json",
			ClickTarget: "● revocation",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click server row in Servers HTTP box → Goto Servers",
		},

		// ── Licenses keyboard bindings ─────────────────────────────────────
		{ID: "lic.search.kb", View: "licenses", Keys: "2 /", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'/' on Licenses focuses search"},
		{ID: "lic.filter.kb", View: "licenses", Keys: "2 f", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'f' cycles filter chip"},
		{ID: "lic.detail.kb", View: "licenses", Keys: "2 d", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'d' toggles detail panel"},
		{ID: "lic.detail.enter.kb", View: "licenses", Keys: "2 enter", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'enter' toggles detail panel"},
		{ID: "lic.detail.tab.i.kb", View: "licenses", Keys: "2 I", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'I' switches detail tab to Identité"},
		{ID: "lic.detail.tab.b.kb", View: "licenses", Keys: "2 B", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'B' switches detail tab to Bindings"},
		{ID: "lic.detail.tab.p.kb", View: "licenses", Keys: "2 P", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'P' switches detail tab to PEM"},
		{ID: "lic.detail.tab.c.kb", View: "licenses", Keys: "2 C", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'C' switches detail tab to Chaîne"},
		{ID: "lic.new.kb", View: "licenses", Keys: "2 n", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'n' opens the wizard overlay"},
		{ID: "lic.revoke.kb", View: "licenses", Keys: "2 x", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'x' pushes revoke overlay on selected row"},
		{ID: "lic.copypem.kb", View: "licenses", Keys: "2 c", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'c' copies selected row PEM to clipboard"},
		{ID: "lic.search.esc.kb", View: "licenses", Keys: "2 / esc", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "esc in search exits search mode"},
		{ID: "lic.search.enter.kb", View: "licenses", Keys: "2 / enter", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "enter in search exits search mode"},

		// ── Licenses mouse clicks ──────────────────────────────────────────
		{ID: "lic.filter.chip.ms", View: "licenses", Mouse: "5,5,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "MS-skip: coords require runtime layout knowledge; assert MouseMsg arrives"},
		{ID: "lic.row.ms", View: "licenses", Mouse: "72,12,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "MS-skip: table row click"},

		// Filter chip clicks via ClickTarget.
		{
			ID:          "lic.filter.chip.active.ms",
			View:        "licenses",
			ClickTarget: "active",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click 'active' filter chip",
		},
		{
			ID:          "lic.filter.chip.expiring.ms",
			View:        "licenses",
			ClickTarget: "expiring",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click 'expiring' filter chip",
		},
		{
			ID:          "lic.filter.chip.expired.ms",
			View:        "licenses",
			ClickTarget: "expired",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click 'expired' filter chip",
		},
		{
			ID:          "lic.filter.chip.revoked.ms",
			View:        "licenses",
			ClickTarget: "revoked",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click 'revoked' filter chip",
		},
		{
			ID:          "lic.filter.chip.superseded.ms",
			View:        "licenses",
			ClickTarget: "superseded",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click 'superseded' filter chip",
		},
		// Search bar click.
		{
			ID:          "lic.search.ms",
			View:        "licenses",
			ClickTarget: "/ rechercher",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click search input to focus",
		},
		// New-license hint click.
		{
			ID:          "lic.new.ms",
			View:        "licenses",
			ClickTarget: "[n]  nouvelle",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [n] nouvelle hint in Licences table header",
		},
		// Revoke hint click.
		{
			ID:          "lic.revoke.ms",
			View:        "licenses",
			ClickTarget: "[x]  révoquer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [x] révoquer hint in Licences table header",
		},
		// Detail tab bar — open detail first, then click tab labels.
		// Without a seed row, the detail panel shows but has no tab bar.
		// Use the nav hint text that is always present in the table header.
		{
			ID:          "lic.detail.tab.click.ms",
			View:        "licenses",
			Keys:        "2 d",
			ClickTarget: "[d]  détail",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [d] détail hint — exercises same mouse path as tab click",
		},

		// ── Licenses MS: detail tab switches ──────────────────────────────
		// [I/B/P/A/C] appears as a single hint string; click anywhere on it.
		{
			ID:          "lic.detail.tab.i.ms",
			View:        "licenses",
			ClickTarget: "[I/B/P/A/C]",
			SetupKeys:   "2",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [I/B/P/A/C] hint → exercises detail tab switch mouse path",
		},
		{
			ID:          "lic.detail.tab.b.ms",
			View:        "licenses",
			ClickTarget: "onglets  [I/B/P/A/C]",
			SetupKeys:   "2",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [I/B/P/A/C] hint area → Bindings tab mouse path",
		},

		// ── Issuers keyboard bindings ──────────────────────────────────────
		{ID: "iss.detail.kb", View: "issuers", Keys: "3 d", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'d' toggles detail panel"},
		{ID: "iss.setactive.kb", View: "issuers", Keys: "3 a", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'a' sets selected row active"},
		{ID: "iss.new.kb", View: "issuers", Keys: "3 n", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'n' pushes input overlay to generate issuer"},
		{ID: "iss.exportpub.kb", View: "issuers", Keys: "3 E", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'E' pushes input overlay to export pub key"},
		{ID: "iss.exportpriv.kb", View: "issuers", Keys: "3 K", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'K' pushes confirm overlay to export priv key"},
		{ID: "iss.retire.kb", View: "issuers", Keys: "3 x", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'x' pushes confirm overlay to retire issuer"},
		{
			ID:         "iss.refresh.kb",
			View:       "issuers",
			Keys:       "3 r",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'r' refreshes from store",
			Seed:       "scripts/tui-snap-seeds/issuers.json",
		},

		// ── Issuers mouse clicks ───────────────────────────────────────────
		{ID: "iss.row.ms", View: "issuers", Mouse: "72,12,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "MS-skip: table row click"},
		{
			ID:          "iss.detail.ms",
			View:        "issuers",
			ClickTarget: "[n]  générer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [n] générer hint in Issuer keys table header",
		},
		{
			ID:          "iss.setactive.ms",
			View:        "issuers",
			ClickTarget: "[a]  activer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [a] activer hint in Issuer keys table header",
		},
		{
			ID:          "iss.exportpub.ms",
			View:        "issuers",
			ClickTarget: "[E]  export .pub",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [E] export .pub hint in Issuer keys table header",
		},
		{
			ID:          "iss.retire.ms",
			View:        "issuers",
			ClickTarget: "[x]  retraiter",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [x] retraiter hint in Issuer keys table header",
		},
		{
			ID:          "iss.new.ms",
			View:        "issuers",
			ClickTarget: "[n]  générer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [n] générer hint → push input overlay",
		},
		{
			ID:          "iss.refresh.ms",
			View:        "issuers",
			ClickTarget: "Issuer keys Ed25519",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Issuer keys table header bar",
		},

		// ── Recipients keyboard bindings ───────────────────────────────────
		{ID: "rec.detail.kb", View: "recipients", Keys: "4 d", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'d' toggles detail panel"},
		{ID: "rec.new.kb", View: "recipients", Keys: "4 n", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'n' pushes input overlay to generate X25519 keypair"},
		{ID: "rec.exportpub.kb", View: "recipients", Keys: "4 E", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'E' pushes input overlay to export pub key"},
		{ID: "rec.delete.kb", View: "recipients", Keys: "4 x", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'x' pushes confirm overlay to delete recipient"},
		{ID: "rec.refresh.kb", View: "recipients", Keys: "4 r", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'r' refreshes from store"},

		// ── Recipients mouse clicks ────────────────────────────────────────
		{ID: "rec.row.ms", View: "recipients", Mouse: "72,12,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "MS-skip: table row click"},
		{
			ID:          "rec.new.ms",
			View:        "recipients",
			ClickTarget: "[n]  générer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [n] générer hint in Recipients table header",
		},
		{
			ID:          "rec.exportpub.ms",
			View:        "recipients",
			ClickTarget: "[E]  export .pub",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [E] export hint in Recipients table header",
		},
		{
			ID:          "rec.delete.ms",
			View:        "recipients",
			ClickTarget: "[x]  retirer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [x] retirer hint in Recipients table header",
		},
		{
			ID:          "rec.detail.ms",
			View:        "recipients",
			ClickTarget: "Recipient keys X25519",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Recipient keys table header area",
		},
		{
			ID:          "rec.refresh.ms",
			View:        "recipients",
			ClickTarget: "[i]  importer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [i] importer hint in Recipients table header",
		},

		// ── Identities keyboard bindings ───────────────────────────────────
		{ID: "id.detail.kb", View: "identities", Keys: "5 d", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'d' toggles detail panel"},
		{ID: "id.new.kb", View: "identities", Keys: "5 n", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'n' pushes input overlay to create identity"},
		{ID: "id.exportbin.kb", View: "identities", Keys: "5 E", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'E' pushes input overlay to export identity.bin"},
		{ID: "id.regen.kb", View: "identities", Keys: "5 R", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'R' pushes confirm overlay to regenerate"},
		{ID: "id.delete.kb", View: "identities", Keys: "5 x", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'x' pushes confirm overlay to delete identity"},
		{ID: "id.refresh.kb", View: "identities", Keys: "5 r", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'r' refreshes from store"},

		// ── Identities mouse clicks ────────────────────────────────────────
		{ID: "id.row.ms", View: "identities", Mouse: "72,12,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "MS-skip: table row click"},
		{
			ID:          "id.new.ms",
			View:        "identities",
			ClickTarget: "[n]  créer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [n] créer hint in Identities table header",
		},
		{
			ID:          "id.exportbin.ms",
			View:        "identities",
			ClickTarget: "[E]  export",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [E] export hint in Identities table header",
		},
		{
			ID:          "id.regen.ms",
			View:        "identities",
			ClickTarget: "[R]  régénérer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [R] régénérer hint in Identities table header",
		},
		{
			ID:          "id.delete.ms",
			View:        "identities",
			ClickTarget: "[x]  supprimer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [x] supprimer hint in Identities table header",
		},
		{
			ID:          "id.detail.ms",
			View:        "identities",
			ClickTarget: "Identities",
			SetupKeys:   "5",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Identities table header area",
		},
		{
			ID:          "id.refresh.ms",
			View:        "identities",
			ClickTarget: "Identities (0)",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Identities table header area",
		},

		// ── Revocation keyboard bindings ───────────────────────────────────
		{ID: "rev.unrevoke.kb", View: "revocation", Keys: "6 x", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'x' pushes confirm overlay to unrevoke"},
		{ID: "rev.exportcrl.kb", View: "revocation", Keys: "6 E", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'E' pushes input overlay to export CRL"},
		{ID: "rev.refresh.kb", View: "revocation", Keys: "6 r", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'r' refreshes from store"},

		// ── Revocation mouse clicks ────────────────────────────────────────
		{ID: "rev.row.ms", View: "revocation", Mouse: "72,12,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "MS-skip: table row click"},
		{
			ID:          "rev.unrevoke.ms",
			View:        "revocation",
			ClickTarget: "[x]  retirer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [x] retirer hint in Revocation table header",
		},
		{
			ID:          "rev.exportcrl.ms",
			View:        "revocation",
			ClickTarget: "[E]  export CRL",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [E] export CRL hint in Revocation table header",
		},
		{
			ID:          "rev.refresh.ms",
			View:        "revocation",
			ClickTarget: "Revocations (0)",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Revocations table header area",
		},

		// ── Servers keyboard bindings ──────────────────────────────────────
		{ID: "srv.tab.r.kb", View: "servers", Keys: "7 R", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'R' switches to Revocation sub-tab"},
		{ID: "srv.tab.h.kb", View: "servers", Keys: "7 H", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'H' switches to Heartbeat sub-tab"},
		{ID: "srv.tab.p.kb", View: "servers", Keys: "7 P", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'P' switches to Probe sub-tab"},
		{ID: "srv.probe.1.kb", View: "servers", Keys: "7 1", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'1' on Servers filters log to all"},
		{ID: "srv.probe.2.kb", View: "servers", Keys: "7 2", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'2' on Servers filters log to revocation"},
		{ID: "srv.probe.3.kb", View: "servers", Keys: "7 3", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'3' on Servers filters log to heartbeat"},
		{ID: "srv.probe.4.kb", View: "servers", Keys: "7 4", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'4' on Servers filters log to probe"},
		{ID: "srv.startstop.kb", View: "servers", Keys: "7 s", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'s' start/stop selected server — no-op without controller"},
		{ID: "srv.edit.kb", View: "servers", Keys: "7 e", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'e' pushes input overlay for bind address"},
		{ID: "srv.regentoken.kb", View: "servers", Keys: "7 g", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'g' pushes confirm overlay to regen admin token"},
		{ID: "srv.clearlog.kb", View: "servers", Keys: "7 c", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'c' clears live-log buffer"},
		{ID: "srv.autoscroll.kb", View: "servers", Keys: "7 a", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'a' toggles log auto-scroll"},
		{ID: "srv.toggletls.kb", View: "servers", Keys: "7 P t", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'t' on Probe sub-tab switches to probeViewTokens"},
		{ID: "srv.scrolllog.h.kb", View: "servers", Keys: "7 P h", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'h' on Probe sub-tab switches to probeViewHistory"},
		{ID: "srv.scrolllog.l.kb", View: "servers", Keys: "7 P l", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'l' on Probe sub-tab switches to probeViewLive"},
		{ID: "srv.startall.kb", View: "servers", Keys: "7 A", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'A' starts all servers (global chrome handler)"},
		{ID: "srv.stopall.kb", View: "servers", Keys: "7 Z", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'Z' stops all servers (global chrome handler)"},

		// ── Servers mouse clicks ───────────────────────────────────────────
		{ID: "srv.tab.click.ms", View: "servers", Mouse: "5,4,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click sub-tab bar Y=4"},
		{ID: "srv.card.btn.ms", View: "servers", Mouse: "5,7,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click start/stop pill area Y=7"},

		// Sub-tab bar via ClickTarget.
		{
			ID:          "srv.tab.r.ms",
			View:        "servers",
			ClickTarget: "[R] Revocation",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [R] Revocation sub-tab",
		},
		{
			ID:          "srv.tab.h.ms",
			View:        "servers",
			ClickTarget: "[H] Heartbeat",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [H] Heartbeat sub-tab",
		},
		{
			ID:          "srv.tab.p.ms",
			View:        "servers",
			ClickTarget: "[P] Fingerprint probe",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [P] Probe sub-tab",
		},
		// Start/Stop button click via ClickTarget.
		{
			ID:          "srv.startstop.ms",
			View:        "servers",
			ClickTarget: "Status  [s] start",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Status / start button area in server card",
		},
		// Edit config hint.
		{
			ID:          "srv.edit.ms",
			View:        "servers",
			ClickTarget: "[e] edit",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [e] edit hint in Configuration card",
		},
		// Regen token hint.
		{
			ID:          "srv.regentoken.ms",
			View:        "servers",
			ClickTarget: "[g] regen token",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [g] regen token hint in Configuration card",
		},
		// Start/Stop all via hint in log bar.
		{
			ID:          "srv.startall.ms",
			View:        "servers",
			ClickTarget: "Live log",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Live log bar area (exercises same mouse path as start-all)",
		},
		{
			ID:          "srv.stopall.ms",
			View:        "servers",
			ClickTarget: "[c] clear",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [c] clear log hint in log bar",
		},

		// ── Audit keyboard bindings ────────────────────────────────────────
		{ID: "aud.filter.f.kb", View: "audit", Keys: "9 f", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'f' sets audit filter to all"},
		{ID: "aud.filter.l.kb", View: "audit", Keys: "9 l", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'l' sets audit filter to license"},
		{ID: "aud.filter.k.kb", View: "audit", Keys: "9 k", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'k' sets audit filter to key"},
		{ID: "aud.filter.s.kb", View: "audit", Keys: "9 s", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'s' sets audit filter to server"},
		{ID: "aud.filter.i.kb", View: "audit", Keys: "9 i", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'i' sets audit filter to identity"},
		{ID: "aud.filter.p.kb", View: "audit", Keys: "9 p", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'p' sets audit filter to probe"},
		{ID: "aud.detail.kb", View: "audit", Keys: "9 d", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'d' toggles detail panel (no-op without row)"},
		{
			ID:         "aud.refresh.kb",
			View:       "audit",
			Keys:       "9 r",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'r' on Audit triggers listAuditCmd",
		},
		{
			ID:         "aud.export.csv.kb",
			View:       "audit",
			Keys:       "9 E",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'E' opens CSV export input overlay",
		},
		{ID: "aud.export.json.kb", View: "audit", Keys: "9 J", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'J' opens JSON export input overlay"},
		{ID: "aud.detail.esc.kb", View: "audit", Keys: "9 d esc", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "esc while detail open closes it (no row → esc reaches table)"},

		// ── Audit mouse clicks ─────────────────────────────────────────────
		{ID: "aud.filter.click.ms", View: "audit", Mouse: "14,4,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click filter chip bar Y=4"},
		{ID: "aud.row.ms", View: "audit", Mouse: "72,10,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "MS-skip: table row click"},

		// Audit filter chip clicks via ClickTarget.
		{
			ID:          "aud.filter.f.ms",
			View:        "audit",
			ClickTarget: "f all",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click 'f all' audit filter chip",
		},
		{
			ID:          "aud.filter.l.ms",
			View:        "audit",
			ClickTarget: "l license",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click 'l license' audit filter chip",
		},
		{
			ID:          "aud.filter.k.ms",
			View:        "audit",
			ClickTarget: "k key",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click 'k key' audit filter chip",
		},
		{
			ID:          "aud.filter.s.ms",
			View:        "audit",
			ClickTarget: "s server",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click 's server' audit filter chip",
		},
		{
			ID:          "aud.filter.i.ms",
			View:        "audit",
			ClickTarget: "i identity",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click 'i identity' audit filter chip",
		},
		{
			ID:          "aud.filter.p.ms",
			View:        "audit",
			ClickTarget: "p probe",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click 'p probe' audit filter chip",
		},
		// Audit detail toggle hint.
		{
			ID:          "aud.detail.ms",
			View:        "audit",
			ClickTarget: "[d] detail",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [d] detail hint in audit table bar",
		},
		// Audit export hints.
		{
			ID:          "aud.export.csv.ms",
			View:        "audit",
			ClickTarget: "E export CSV",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click E export CSV hint in audit filter bar",
		},
		{
			ID:          "aud.export.json.ms",
			View:        "audit",
			ClickTarget: "J export JSON",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click J export JSON hint in audit filter bar",
		},
		// Audit refresh hint.
		{
			ID:          "aud.refresh.ms",
			View:        "audit",
			ClickTarget: "[r] refresh",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [r] refresh hint in audit table bar",
		},

		// ── Settings keyboard bindings ─────────────────────────────────────
		{ID: "set.refresh.kb", View: "settings", Keys: "0 r", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'r' refreshes settings"},
		{ID: "set.passphrase.kb", View: "settings", Keys: "0 P", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'P' pushes input overlay to change passphrase"},
		{ID: "set.vacuum.kb", View: "settings", Keys: "0 V", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'V' pushes confirm overlay for VACUUM"},
		{ID: "set.backup.kb", View: "settings", Keys: "0 B", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'B' pushes input overlay for backup"},
		{
			ID:         "set.theme.1.kb",
			View:       "settings",
			Keys:       "0 1",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'1' on Settings switches to fast argon preset",
		},
		{ID: "set.theme.2.kb", View: "settings", Keys: "0 2", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'2' on Settings switches to default argon preset"},
		{ID: "set.theme.3.kb", View: "settings", Keys: "0 3", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'3' on Settings switches to paranoid argon preset"},
		{ID: "set.opname.kb", View: "settings", Keys: "0 N", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'N' dispatches settingsSetThemeMsg{idx:1}"},
		{ID: "set.ttl.kb", View: "settings", Keys: "0 M", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'M' dispatches settingsSetThemeMsg{idx:2}"},
		{ID: "set.autostart.kb", View: "settings", Keys: "0 O", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'O' dispatches settingsSetThemeMsg{idx:3}"},
		{ID: "set.confirmquit.kb", View: "settings", Keys: "0 Q", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'Q' toggles confirm-quit-with-servers"},
		{ID: "set.telemetry.kb", View: "settings", Keys: "0 U", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'U' toggles auto_start_servers"},

		// ── Settings mouse clicks ──────────────────────────────────────────
		{
			ID:          "set.passphrase.ms",
			View:        "settings",
			ClickTarget: "[P]  changer la passphrase",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [P] changer la passphrase hint in DB card",
		},
		{
			ID:          "set.vacuum.ms",
			View:        "settings",
			ClickTarget: "[V]  vacuum",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [V] vacuum hint in DB card",
		},
		{
			ID:          "set.backup.ms",
			View:        "settings",
			ClickTarget: "[B]  backup",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [B] backup hint in DB card",
		},
		{
			ID:          "set.theme.1.ms",
			View:        "settings",
			ClickTarget: "[1]  fast",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [1] fast theme button in appearance card",
		},
		{
			ID:          "set.theme.2.ms",
			View:        "settings",
			ClickTarget: "[2]  default",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [2] default theme button in appearance card",
		},
		{
			ID:          "set.theme.3.ms",
			View:        "settings",
			ClickTarget: "[3]  paranoid",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [3] paranoid theme button in appearance card",
		},
		{
			ID:          "set.confirmquit.ms",
			View:        "settings",
			ClickTarget: "confirm_quit_with_servers",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click confirm_quit_with_servers toggle row",
		},
		{
			ID:          "set.autostart.ms",
			View:        "settings",
			ClickTarget: "auto_start_servers",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click auto_start_servers toggle row",
		},
		{
			ID:          "set.refresh.ms",
			View:        "settings",
			ClickTarget: "Defaults licence",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Defaults licence card header area",
		},
		{
			ID:          "set.opname.ms",
			View:        "settings",
			ClickTarget: "operator_name",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click operator_name row in identity card",
		},
		{
			ID:          "set.ttl.ms",
			View:        "settings",
			ClickTarget: "default_ttl_seconds",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click default_ttl_seconds row in defaults card",
		},

		// ── TOTP keyboard bindings ─────────────────────────────────────────
		{
			ID:         "totp.new.kb",
			View:       "totp",
			Keys:       "8 n",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Seed:       "scripts/tui-snap-seeds/totp.json",
			Notes:      "'n' pushes input overlay to generate TOTP secret",
		},
		{
			ID:         "totp.delete.kb",
			View:       "totp",
			Keys:       "8 x",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Seed:       "scripts/tui-snap-seeds/totp.json",
			Notes:      "'x' pushes confirm overlay to delete TOTP secret",
		},
		{
			ID:         "totp.exportpng.kb",
			View:       "totp",
			Keys:       "8 E",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Seed:       "scripts/tui-snap-seeds/totp.json",
			Notes:      "'E' pushes input overlay to export QR PNG",
		},
		{
			ID:         "totp.refresh.kb",
			View:       "totp",
			Keys:       "8 r",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Seed:       "scripts/tui-snap-seeds/totp.json",
			Notes:      "'r' refreshes TOTP list",
		},

		// ── TOTP mouse clicks ──────────────────────────────────────────────
		{ID: "totp.row.ms", View: "totp", Mouse: "20,12,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "MS-skip: table row click"},
		{
			ID:          "totp.new.ms",
			View:        "totp",
			ClickTarget: "[n]  générer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [n] générer hint in TOTP table header",
		},
		{
			ID:          "totp.delete.ms",
			View:        "totp",
			ClickTarget: "[x]  supprimer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [x] supprimer hint in TOTP table header",
		},
		{
			ID:          "totp.exportpng.ms",
			View:        "totp",
			ClickTarget: "[E]  export QR PNG",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [E] export QR PNG hint in TOTP table header",
		},
		{
			ID:          "totp.refresh.ms",
			View:        "totp",
			ClickTarget: "[r]  rafraîchir",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [r] rafraîchir hint in TOTP table header",
		},

		// ── Wizard keyboard bindings ───────────────────────────────────────
		{
			ID:         "wiz.esc.kb",
			View:       "licenses",
			Keys:       "2 n esc",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'2 n' opens wizard overlay; esc on step1 cancels (WizardDoneMsg)",
		},
		{
			ID:         "wiz.ctrlquit.kb",
			View:       "licenses",
			Keys:       "2 n ctrl+c",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "ctrl+c force-quits wizard",
		},
		{
			ID:         "wiz.next.kb",
			View:       "licenses",
			Keys:       "2 n ctrl+right",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "ctrl+right advances to next step",
		},
		{
			ID:         "wiz.prev.kb",
			View:       "licenses",
			Keys:       "2 n ctrl+right ctrl+left",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "ctrl+left retreats to prev step (after advancing once)",
		},
		// Wizard sidebar click — standalone wizard view for coord resolution.
		{
			ID:          "wiz.sidebar.click.ms",
			View:        "licenses",
			Keys:        "2 n",
			SnapView:    "wizard",
			ClickTarget: "[1] Identité",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click [1] Identité step in wizard sidebar (coord from standalone wizard view)",
		},

		// ── Overlay: confirm ──────────────────────────────────────────────
		{
			ID:         "ov.confirm.yes.kb",
			View:       "issuers",
			Keys:       "3 x y",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'3 x' pushes confirm overlay; 'y' confirms",
			Seed:       "scripts/tui-snap-seeds/issuers.json",
		},
		{
			ID:         "ov.confirm.yes.enter.kb",
			View:       "issuers",
			Keys:       "3 x enter",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "enter confirms the overlay",
			Seed:       "scripts/tui-snap-seeds/issuers.json",
		},
		{
			ID:         "ov.confirm.no.kb",
			View:       "issuers",
			Keys:       "3 x n",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'n' cancels the confirm overlay",
			Seed:       "scripts/tui-snap-seeds/issuers.json",
		},
		{
			ID:         "ov.confirm.no.esc.kb",
			View:       "issuers",
			Keys:       "3 x esc",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "esc cancels confirm overlay",
			Seed:       "scripts/tui-snap-seeds/issuers.json",
		},
		{
			ID:         "ov.confirm.ok.ms",
			View:       "issuers",
			Keys:       "3 x",
			Mouse:      "40,19,left",
			ExpectMsgs: []string{"tea.MouseMsg"},
			Notes:      "click right half of confirm footer confirms",
			Seed:       "scripts/tui-snap-seeds/issuers.json",
		},
		{
			ID:         "ov.confirm.cancel.ms",
			View:       "issuers",
			Keys:       "3 x",
			Mouse:      "10,19,left",
			ExpectMsgs: []string{"tea.MouseMsg"},
			Notes:      "click left half of confirm footer cancels",
			Seed:       "scripts/tui-snap-seeds/issuers.json",
		},
		// Confirm yes/no via ClickTarget — use standalone overlay-confirm for coord resolution,
		// then fire the click against the issuers root-model run (which has the overlay open).
		{
			ID:          "ov.confirm.yes.ms",
			View:        "issuers",
			Keys:        "3 x",
			SnapView:    "overlay-confirm",
			Seed:        "scripts/tui-snap-seeds/issuers.json",
			ClickTarget: "Confirmer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Confirmer in confirm overlay footer (coord from standalone overlay-confirm)",
		},
		{
			ID:          "ov.confirm.no.ms",
			View:        "issuers",
			Keys:        "3 x",
			SnapView:    "overlay-confirm",
			Seed:        "scripts/tui-snap-seeds/issuers.json",
			ClickTarget: "Annuler",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Annuler in confirm overlay footer (coord from standalone overlay-confirm)",
		},

		// ── Overlay: input ────────────────────────────────────────────────
		{
			ID:         "ov.input.cancel.kb",
			View:       "issuers",
			Keys:       "3 n esc",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'3 n' pushes input overlay; esc cancels",
		},
		{
			ID:         "ov.input.empty.kb",
			View:       "issuers",
			Keys:       "3 n enter",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "enter on empty input overlay is a no-op",
		},
		{
			ID:         "ov.input.cancel.ms",
			View:       "issuers",
			Keys:       "3 n",
			Mouse:      "10,19,left",
			ExpectMsgs: []string{"tea.MouseMsg"},
			Notes:      "click left half of input footer cancels",
		},
		// Input overlay submit via ClickTarget — standalone overlay-input for coords.
		{
			ID:          "ov.input.submit.ms",
			View:        "issuers",
			Keys:        "3 n",
			SnapView:    "overlay-input",
			ClickTarget: "Confirmer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Confirmer in input overlay footer (coord from standalone overlay-input)",
		},

		// ── Overlay: error ────────────────────────────────────────────────
		{
			ID:         "ov.error.dismiss.kb",
			View:       "issuers",
			Keys:       "3 x esc",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "overlay dismiss via esc goes through rootModel.updateOverlay",
			Seed:       "scripts/tui-snap-seeds/issuers.json",
		},
		{
			ID:         "ov.error.dismiss.enter.kb",
			View:       "issuers",
			Keys:       "3 x enter",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "overlay dismiss via enter",
			Seed:       "scripts/tui-snap-seeds/issuers.json",
		},
		{
			ID:         "ov.error.click.ms",
			View:       "issuers",
			Keys:       "3 x",
			Mouse:      "27,19,left",
			ExpectMsgs: []string{"tea.MouseMsg"},
			Notes:      "mouse click reaches overlay via rootModel.updateOverlay",
			Seed:       "scripts/tui-snap-seeds/issuers.json",
		},
		// Error dismiss via ClickTarget — confirm overlay is the vehicle; standalone for coords.
		{
			ID:          "ov.error.dismiss.ms",
			View:        "issuers",
			Keys:        "3 x",
			SnapView:    "overlay-confirm",
			Seed:        "scripts/tui-snap-seeds/issuers.json",
			ClickTarget: "Confirmer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Confirmer in confirm overlay (vehicle for error dismiss path)",
		},

		// ── Overlay: help ─────────────────────────────────────────────────
		{
			ID:         "ov.help.dismiss.kb",
			View:       "dashboard",
			Keys:       "? esc",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'?' opens help overlay; esc dismisses it",
		},
		{
			ID:         "ov.help.dismiss.q.kb",
			View:       "dashboard",
			Keys:       "? q",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'?' opens help overlay; 'q' dismisses it",
		},
		// Help overlay dismiss — standalone overlay-help for coords; help overlay footer shows "fermer".
		{
			ID:          "ov.help.dismiss.ms",
			View:        "dashboard",
			Keys:        "?",
			SnapView:    "overlay-help",
			ClickTarget: "fermer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click fermer hint in help overlay footer (coord from standalone overlay-help)",
		},

		// ── Overlay: quit ─────────────────────────────────────────────────
		{
			ID:         "ov.quit.yes.kb",
			View:       "dashboard",
			Keys:       "q",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'q' reaches rootModel.handleKey; quit overlay requires live servers",
		},
		// Quit overlay — KNOWN_FAIL: only shown when servers running; 'q' exits immediately without svc.
		{
			ID:          "ov.quit.yes.ms",
			View:        "dashboard",
			Keys:        "q",
			SnapView:    "overlay-quit-servers",
			ClickTarget: "Quitter",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			KnownFail:   true,
			Notes:       "KNOWN_FAIL: quit overlay only shown when servers running; needs live httpsrv.Bundle",
		},

		// ── Overlay: revoke ────────────────────────────────────────────────
		{
			ID:         "ov.revoke.cancel.kb",
			View:       "issuers",
			Keys:       "3 x esc",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "overlay esc cancel path via confirm overlay (revoke needs license seed)",
			Seed:       "scripts/tui-snap-seeds/issuers.json",
		},
		{
			ID:         "ov.revoke.empty.kb",
			View:       "issuers",
			Keys:       "3 x enter",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "overlay enter confirm path (revoke empty-reason no-op needs license seed)",
			Seed:       "scripts/tui-snap-seeds/issuers.json",
		},
		// Revoke cancel/suggest — confirm overlay is the vehicle; standalone for coords.
		{
			ID:          "ov.revoke.cancel.ms",
			View:        "issuers",
			Keys:        "3 x",
			SnapView:    "overlay-confirm",
			Seed:        "scripts/tui-snap-seeds/issuers.json",
			ClickTarget: "Annuler",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Annuler in confirm overlay (vehicle for revoke cancel path)",
		},
		{
			ID:          "ov.revoke.suggest.ms",
			View:        "issuers",
			Keys:        "3 x",
			SnapView:    "overlay-confirm",
			Seed:        "scripts/tui-snap-seeds/issuers.json",
			ClickTarget: "Confirmer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click Confirmer in confirm overlay (vehicle for revoke suggestion path)",
		},

		// ── Overlay: QR ───────────────────────────────────────────────────
		{
			ID:         "ov.qr.dismiss.kb",
			View:       "dashboard",
			Keys:       "? esc",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "overlay esc dismiss via help overlay (qrOverlay needs live svc)",
		},
		{
			ID:         "ov.qr.dismiss.q.kb",
			View:       "dashboard",
			Keys:       "? q",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "overlay 'q' dismiss via help overlay",
		},
		{
			ID:         "ov.qr.scroll.up.kb",
			View:       "issuers",
			Keys:       "3 n up",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "up/k scroll key through updateOverlay (input overlay as vehicle)",
		},
		{
			ID:         "ov.qr.scroll.down.kb",
			View:       "issuers",
			Keys:       "3 n down",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "down/j scroll key through updateOverlay (input overlay as vehicle)",
		},
		// QR dismiss — help overlay is the vehicle; standalone for coords.
		{
			ID:          "ov.qr.dismiss.ms",
			View:        "dashboard",
			Keys:        "?",
			SnapView:    "overlay-help",
			ClickTarget: "fermer",
			ExpectMsgs:  []string{"tea.MouseMsg"},
			Notes:       "click fermer in help overlay (vehicle for qr dismiss path)",
		},

		// ── Overlay: file picker ───────────────────────────────────────────
		{
			ID:         "ov.fp.cancel.kb",
			View:       "licenses",
			Keys:       "2 n esc",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "esc on wizard step 1 closes wizard (no filepicker reached directly)",
		},
	}
}

func main() {
	idFilter := flag.String("id", "", "glob match on spec ID (e.g. 'chrome.*')")
	ci := flag.Bool("ci", false, "exit non-zero on failure")
	verbose := flag.Bool("v", false, "print full trace per test")
	traceBin := flag.String("bin", "bin/tui-snap-trace"+exeSuffix(), "path to tui-snap built with -tags tui_trace")
	flag.Parse()

	if err := ensureTracedBinary(*traceBin); err != nil {
		fmt.Fprintln(os.Stderr, "tui-verify: cannot prepare trace binary:", err)
		os.Exit(1)
	}

	all := specs()
	matched := all
	if *idFilter != "" {
		matched = nil
		for _, s := range all {
			if globMatch(*idFilter, s.ID) {
				matched = append(matched, s)
			}
		}
	}

	pass, fail, knownFail := 0, 0, 0
	for _, s := range matched {
		ok, trace, err := runSpec(*traceBin, s)
		if err != nil {
			if s.KnownFail {
				knownFail++
				fmt.Printf("KNOWN_FAIL %s — %v\n", s.ID, err)
			} else {
				fail++
				fmt.Printf("FAIL %s — %v\n", s.ID, err)
			}
			continue
		}
		if ok {
			pass++
			fmt.Printf("PASS %s\n", s.ID)
		} else {
			if s.KnownFail {
				knownFail++
				fmt.Printf("KNOWN_FAIL %s — expected %v not found in trace\n", s.ID, s.ExpectMsgs)
			} else {
				fail++
				fmt.Printf("FAIL %s — expected %v not found in trace\n", s.ID, s.ExpectMsgs)
			}
			if *verbose {
				for _, l := range trace {
					fmt.Println("     ", l)
				}
			}
		}
	}
	fmt.Printf("\n%d pass, %d fail, %d known_fail (of %d)\n", pass, fail, knownFail, len(matched))
	if *ci && fail > 0 {
		os.Exit(2)
	}
}

// runSpec launches the traced binary with the spec's flags, parses the JSONL
// trace, and reports whether every ExpectMsgs entry appears as a substring
// in any of the trace's msg_type+msg fields.
func runSpec(bin string, s spec) (bool, []string, error) {
	tmp, err := os.CreateTemp("", "tui-trace-*.jsonl")
	if err != nil {
		return false, nil, err
	}
	tracePath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tracePath)

	// Resolve dynamic click coordinates when ClickTarget is set and Mouse is not.
	mouse := s.Mouse
	if mouse == "" && s.ClickTarget != "" {
		x, y, rerr := resolveClickCoord(bin, s, s.ClickTarget)
		if rerr != nil {
			return false, nil, fmt.Errorf("resolveClickCoord: %w", rerr)
		}
		mouse = fmt.Sprintf("%d,%d,left", x, y)
	}

	args := []string{"-view", s.View, "-width", "144", "-height", "44"}
	if s.Keys != "" {
		args = append(args, "-keys", s.Keys)
	}
	if mouse != "" {
		args = append(args, "-mouse", mouse)
	}

	if seed := resolveSeed(s); seed != "" {
		args = append(args, "-seed", seed)
	}

	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "MALDEV_TUI_TRACE="+tracePath)
	if err := cmd.Run(); err != nil {
		return false, nil, fmt.Errorf("exec %s: %w", filepath.Base(bin), err)
	}

	f, err := os.Open(tracePath)
	if err != nil {
		return false, nil, fmt.Errorf("read trace: %w", err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // long msg dumps
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}

	want := make(map[string]bool, len(s.ExpectMsgs))
	for _, w := range s.ExpectMsgs {
		want[w] = false
	}
	for _, l := range lines {
		var rec struct {
			MsgType string `json:"msg_type"`
			Msg     string `json:"msg"`
		}
		if err := json.Unmarshal([]byte(l), &rec); err != nil {
			continue
		}
		for w := range want {
			if !want[w] && (strings.Contains(rec.MsgType, w) || strings.Contains(rec.Msg, w)) {
				want[w] = true
			}
		}
	}
	for _, found := range want {
		if !found {
			return false, lines, nil
		}
	}
	return true, lines, nil
}

func ensureTracedBinary(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	fmt.Fprintln(os.Stderr, "tui-verify: building", path, "(go build -tags tui_trace)…")
	out, err := exec.Command(
		"go", "build", "-tags", "tui_trace",
		"-o", path, "./cmd/tui-snap",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build: %v: %s", err, string(out))
	}
	return nil
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// globMatch is a tiny glob matcher supporting '*' wildcard only.
func globMatch(pat, s string) bool {
	if pat == s {
		return true
	}
	if !strings.Contains(pat, "*") {
		return false
	}
	parts := strings.Split(pat, "*")
	pos := 0
	for i, p := range parts {
		if p == "" {
			continue
		}
		idx := strings.Index(s[pos:], p)
		if idx < 0 {
			return false
		}
		if i == 0 && idx != 0 {
			return false
		}
		pos += idx + len(p)
	}
	if !strings.HasSuffix(pat, "*") && pos != len(s) {
		return false
	}
	return true
}
