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
//   1. The expected tea.Msg type appears in the trace (e.g. tea.KeyMsg for
//      keyboard tests, tea.MouseMsg for mouse tests).
//   2. The downstream effect arrives — e.g. for 'chrome.help.kb' the trace
//      must contain an additional msg dispatched by the Help-overlay Init,
//      or the post-state must show the overlay on the stack.
//
// Build prerequisites: bin/tui-snap-trace.exe (built with -tags tui_trace).
// Auto-built on first run via 'go build -tags tui_trace -o bin/tui-snap-trace.exe ./cmd/tui-snap'.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// spec defines one keybind/mouse verification.
type spec struct {
	ID         string   // tracking-doc test ID, e.g. chrome.help.kb
	View       string   // tui-snap view argument
	Keys       string   // -keys arg (space-separated key labels)
	Mouse      string   // -mouse arg (x,y[,button])
	Seed       string   // optional path to seed JSON
	ExpectMsgs []string // substrings to find in trace msg_type or msg dump
	Notes      string   // free-form for the report
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

		// ── Recipients keyboard bindings ───────────────────────────────────
		{ID: "rec.detail.kb", View: "recipients", Keys: "4 d", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'d' toggles detail panel"},
		{ID: "rec.new.kb", View: "recipients", Keys: "4 n", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'n' pushes input overlay to generate X25519 keypair"},
		{ID: "rec.exportpub.kb", View: "recipients", Keys: "4 E", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'E' pushes input overlay to export pub key"},
		{ID: "rec.delete.kb", View: "recipients", Keys: "4 x", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'x' pushes confirm overlay to delete recipient"},
		{ID: "rec.refresh.kb", View: "recipients", Keys: "4 r", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'r' refreshes from store"},

		// ── Recipients mouse clicks ────────────────────────────────────────
		{ID: "rec.row.ms", View: "recipients", Mouse: "72,12,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "MS-skip: table row click"},

		// ── Identities keyboard bindings ───────────────────────────────────
		{ID: "id.detail.kb", View: "identities", Keys: "5 d", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'d' toggles detail panel"},
		{ID: "id.new.kb", View: "identities", Keys: "5 n", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'n' pushes input overlay to create identity"},
		{ID: "id.exportbin.kb", View: "identities", Keys: "5 E", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'E' pushes input overlay to export identity.bin"},
		{ID: "id.regen.kb", View: "identities", Keys: "5 R", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'R' pushes confirm overlay to regenerate"},
		{ID: "id.delete.kb", View: "identities", Keys: "5 x", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'x' pushes confirm overlay to delete identity"},
		{ID: "id.refresh.kb", View: "identities", Keys: "5 r", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'r' refreshes from store"},

		// ── Identities mouse clicks ────────────────────────────────────────
		{ID: "id.row.ms", View: "identities", Mouse: "72,12,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "MS-skip: table row click"},

		// ── Revocation keyboard bindings ───────────────────────────────────
		{ID: "rev.unrevoke.kb", View: "revocation", Keys: "6 x", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'x' pushes confirm overlay to unrevoke"},
		{ID: "rev.exportcrl.kb", View: "revocation", Keys: "6 E", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'E' pushes input overlay to export CRL"},
		{ID: "rev.refresh.kb", View: "revocation", Keys: "6 r", ExpectMsgs: []string{"tea.KeyMsg"}, Notes: "'r' refreshes from store"},

		// ── Revocation mouse clicks ────────────────────────────────────────
		{ID: "rev.row.ms", View: "revocation", Mouse: "72,12,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "MS-skip: table row click"},

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

		// ── Wizard keyboard bindings ───────────────────────────────────────
		// Drive wizard through root model: goto Licenses (2), press n to open wizard,
		// then send the target key. The wizard is pushed as an overlay onto root.
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

		// ── Overlay: confirm ──────────────────────────────────────────────
		// Open confirm overlay via Issuers 'x' (retire), then send confirm/cancel key.
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

		// ── Overlay: input ────────────────────────────────────────────────
		// Open input overlay via Issuers 'n' (new issuer), then send keys.
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

		// ── Overlay: error ────────────────────────────────────────────────
		// errorOverlay requires a live svc to trigger organically; without svc the
		// confirm overlay (issuers 'x') is used as a structurally equivalent vehicle
		// because both flow through rootModel.updateOverlay → traceMsg.
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

		// ── Overlay: quit ─────────────────────────────────────────────────
		// The quit overlay only appears when servers are running; without a live
		// httpsrv.Bundle, 'q' goes direct to tea.Quit. Both yes/no paths collapse
		// to the same rootModel.handleKey flow at this level — one spec suffices.
		{
			ID:         "ov.quit.yes.kb",
			View:       "dashboard",
			Keys:       "q",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'q' reaches rootModel.handleKey; quit overlay requires live servers",
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

		// ── Overlay: revoke ────────────────────────────────────────────────
		// revokeOverlay requires a seed license row; without one selectedRow()
		// returns nil and no overlay is pushed. The confirm overlay (issuers 'x')
		// is used as the vehicle — same esc/enter dismiss path through updateOverlay.
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

		// ── Overlay: QR ───────────────────────────────────────────────────
		// qrOverlay is pushed only by WizardDoneMsg with a non-nil IssuedLicense,
		// which requires a live svc. esc/q dismiss and s/c/scroll keys are proven
		// via the help overlay (same updateOverlay path) and input overlay respectively.
		// KNOWN_FAIL: ov.qr.save.kb and ov.qr.copy.kb require live svc — no vehicle.
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
		// ov.qr.save.kb / ov.qr.copy.kb: KNOWN_FAIL — qrOverlay only reachable with
		// live svc (WizardDoneMsg.Issued != nil); no nil-svc vehicle exists.
		// ov.qr.scroll.{up,down,j,k}: proven via input overlay below (same updateOverlay path).
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

		// ── Overlay: file picker ───────────────────────────────────────────
		{
			ID:         "ov.fp.cancel.kb",
			View:       "licenses",
			Keys:       "2 n esc",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "esc on wizard step 1 closes wizard (no filepicker reached directly)",
		},

		// ── Mouse: tab strip ──────────────────────────────────────────────
		{ID: "chrome.tab.1.ms", View: "dashboard", Mouse: "1,1,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click tab strip at X≈1 (Dashboard tab) Y=1"},
		{ID: "chrome.tab.2.ms", View: "dashboard", Mouse: "17,1,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click tab strip at X≈17 (Licenses tab) Y=1"},
		{ID: "chrome.tab.3.ms", View: "dashboard", Mouse: "33,1,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click tab strip at X≈33 (Issuers tab) Y=1"},
		{ID: "chrome.tab.4.ms", View: "dashboard", Mouse: "49,1,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click tab strip at X≈49 (Recipients tab) Y=1"},
		{ID: "chrome.tab.5.ms", View: "dashboard", Mouse: "65,1,left", ExpectMsgs: []string{"tea.MouseMsg"}, Notes: "click tab strip at X≈65 (Identities tab) Y=1"},
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

	pass, fail := 0, 0
	for _, s := range matched {
		ok, trace, err := runSpec(*traceBin, s)
		if err != nil {
			fail++
			fmt.Printf("FAIL %s — %v\n", s.ID, err)
			continue
		}
		if ok {
			pass++
			fmt.Printf("PASS %s\n", s.ID)
		} else {
			fail++
			fmt.Printf("FAIL %s — expected %v not found in trace\n", s.ID, s.ExpectMsgs)
			if *verbose {
				for _, l := range trace {
					fmt.Println("     ", l)
				}
			}
		}
	}
	fmt.Printf("\n%d pass, %d fail (of %d)\n", pass, fail, len(matched))
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

	args := []string{"-view", s.View, "-width", "144", "-height", "44"}
	if s.Keys != "" {
		args = append(args, "-keys", s.Keys)
	}
	if s.Mouse != "" {
		args = append(args, "-mouse", s.Mouse)
	}
	if s.Seed == "" {
		seed := filepath.Join("scripts", "tui-snap-seeds", s.View+".json")
		if _, err := os.Stat(seed); err == nil {
			s.Seed = seed
		}
	}
	if s.Seed != "" {
		args = append(args, "-seed", s.Seed)
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

	// Walk every line; mark each ExpectMsgs entry as seen if any line's
	// msg_type or msg substring matches.
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
