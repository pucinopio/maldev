// tui-orphan-scan inventories every "[hotkey]" hint visible in the rendered
// TUI output and cross-references it against the actual key handlers grep-ed
// out of the screen sources. A hint that appears on screen but has no
// matching `case "<k>":` in the same screen file is an "orphan" — a visual
// promise the code never delivers.
//
// Usage:
//
//	tui-orphan-scan                       # scan every view with default size
//	tui-orphan-scan -view dashboard       # one view only
//	tui-orphan-scan -view licenses -keys d  # apply keys then scan (e.g. with detail panel open)
//	tui-orphan-scan -json                 # machine-readable output
//
// Implementation: shells out to bin/tui-snap with the same flags, strips ANSI,
// finds every `[X]` and `[abc]` token, then `grep`s the matching screen Go
// source for `case "<x>"` substrings. ASCII-only chars in [...] are honoured;
// braces with French words ("[k] gérer") are split on whitespace and the
// leading token is the hotkey.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// screenSource maps a tui-snap view name to its primary source file
// (where the keybinds live). Used for the orphan cross-reference.
var screenSource = map[string]string{
	"dashboard":   "internal/manager/tui/screen_dashboard.go",
	"licenses":    "internal/manager/tui/screen_licenses.go",
	"issuers":     "internal/manager/tui/screen_issuers.go",
	"recipients":  "internal/manager/tui/screen_recipients.go",
	"identities":  "internal/manager/tui/screen_identities.go",
	"revocation":  "internal/manager/tui/screen_revocation.go",
	"servers":     "internal/manager/tui/screen_servers.go",
	"audit":       "internal/manager/tui/screen_audit.go",
	"settings":    "internal/manager/tui/screen_settings.go",
	"totp":        "internal/manager/tui/screen_totp.go",
	"onboarding":  "internal/manager/tui/screen_onboarding.go",
	"wizard":      "internal/manager/tui/screen_wizard.go",
}

// globalKeys are bindings owned by the chrome (app.go) — every screen
// inherits them implicitly so they should never count as orphans.
var globalKeys = map[string]bool{
	"1": true, "2": true, "3": true, "4": true, "5": true,
	"6": true, "7": true, "8": true, "9": true,
	"tab": true, "shift+tab": true, "q": true, "?": true,
	"r": true, "A": true, "Z": true,
}

// hintTokenRE captures the hotkey inside a bracketed hint. Examples that
// match: "[k]", "[d]", "[↵]", "[/]", "[ctrl+n]". The negative-class char
// list intentionally allows multi-char hotkeys like "ctrl+n" and the
// French enter glyph "↵".
var hintTokenRE = regexp.MustCompile(`\[([^\]]{1,12})\]`)

// ansiRE strips SGR / cursor-position escape sequences before hint scanning.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// findings is the per-view scan result.
type findings struct {
	View        string   `json:"view"`
	HintsOnScreen []string `json:"hints_on_screen"`
	HandlersInCode []string `json:"handlers_in_code"`
	Orphans     []string   `json:"orphans"`
}

func main() {
	view := flag.String("view", "", "single view to scan (empty = all)")
	keys := flag.String("keys", "", "keys to send before scan")
	jsonOut := flag.Bool("json", false, "machine-readable output")
	width := flag.Int("width", 160, "terminal width")
	height := flag.Int("height", 48, "terminal height")
	binPath := flag.String("bin", "bin/tui-snap.exe", "path to tui-snap binary")
	flag.Parse()

	if _, err := os.Stat(*binPath); os.IsNotExist(err) {
		// Try Linux name.
		alt := strings.TrimSuffix(*binPath, ".exe")
		if _, err := os.Stat(alt); err == nil {
			*binPath = alt
		} else {
			fmt.Fprintln(os.Stderr, "tui-orphan-scan: binary not found — run 'make tui-snap' first")
			os.Exit(1)
		}
	}

	views := []string{}
	if *view != "" {
		views = []string{*view}
	} else {
		for k := range screenSource {
			views = append(views, k)
		}
		sort.Strings(views)
	}

	var results []findings
	for _, v := range views {
		f, err := scanView(*binPath, v, *keys, *width, *height)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", v, err)
			continue
		}
		results = append(results, f)
	}

	if *jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(results)
		return
	}
	printReport(results)
}

func scanView(bin, view, keys string, w, h int) (findings, error) {
	src, ok := screenSource[view]
	if !ok {
		return findings{}, fmt.Errorf("unknown view %q", view)
	}
	args := []string{"-view", view, "-width", fmt.Sprint(w), "-height", fmt.Sprint(h)}
	if keys != "" {
		args = append(args, "-keys", keys)
	}
	seed := filepath.Join("scripts", "tui-snap-seeds", view+".json")
	if _, err := os.Stat(seed); err == nil {
		args = append(args, "-seed", seed)
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		return findings{}, fmt.Errorf("tui-snap exec: %w", err)
	}
	stripped := ansiRE.ReplaceAllString(string(out), "")

	// Collect every bracketed hint token, lowercased, deduplicated.
	hintSet := map[string]bool{}
	// Symbols inside [...] that aren't hotkeys (pills, glyphs, status badges).
	symbolGlyphs := map[string]bool{
		"✓": true, "✗": true, "•": true, "·": true, "ON": true, "OFF": true,
		"ACTIVE": true, "WARN": true, "ERR": true, "OK": true, "→": true, "←": true,
	}
	for _, m := range hintTokenRE.FindAllStringSubmatch(stripped, -1) {
		raw := strings.TrimSpace(m[1])
		// Skip empty + obvious non-hotkey brackets (digits with no meaning, etc.).
		if raw == "" {
			continue
		}
		// Skip "[ID]" headers in tables (uppercase 2+ letter word treated as a
		// column header, not a hotkey).
		if len(raw) > 4 && raw == strings.ToUpper(raw) {
			continue
		}
		if symbolGlyphs[raw] {
			continue
		}
		hintSet[raw] = true
	}
	hints := keysOf(hintSet)
	sort.Strings(hints)

	// Read the screen source + extract every case "x" string.
	body, err := os.ReadFile(src)
	if err != nil {
		return findings{}, fmt.Errorf("read source: %w", err)
	}
	caseRE := regexp.MustCompile(`case\s+"([^"]+)"`)
	handlers := map[string]bool{}
	for _, m := range caseRE.FindAllStringSubmatch(string(body), -1) {
		// Skip overlay-result IDs like "identity-name" — those aren't keybinds.
		if strings.Contains(m[1], "-") {
			continue
		}
		handlers[m[1]] = true
	}
	handlersSlice := keysOf(handlers)
	sort.Strings(handlersSlice)

	var orphans []string
	for _, h := range hints {
		if handlers[h] {
			continue
		}
		if globalKeys[h] {
			continue
		}
		// Hint like "↵" is bound to tea.KeyEnter ("enter") — recognise this
		// alias so it doesn't count as orphan.
		if h == "↵" && handlers["enter"] {
			continue
		}
		// Single-letter inverse-case alias: many screens bind both lowercase
		// and uppercase variants implicitly through the global keymap. Skip
		// if the inverse case is in handlers.
		if len(h) == 1 {
			lower := strings.ToLower(h)
			upper := strings.ToUpper(h)
			if handlers[lower] || handlers[upper] {
				// Only count as orphan if neither case appears AND the global
				// keymap doesn't claim it either.
				if !globalKeys[lower] && !globalKeys[upper] {
					continue
				}
			}
		}
		orphans = append(orphans, h)
	}

	return findings{
		View:           view,
		HintsOnScreen:  hints,
		HandlersInCode: handlersSlice,
		Orphans:        orphans,
	}, nil
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func printReport(rs []findings) {
	totalOrphans := 0
	fmt.Println("# TUI orphan-hint scan")
	fmt.Println()
	fmt.Println("Each row: hints visible on screen vs handlers wired in code.")
	fmt.Println("Orphans = visual promise without a code handler in the same screen.")
	fmt.Println()
	for _, r := range rs {
		fmt.Printf("## %s\n", r.View)
		fmt.Printf("  hints (screen):    %s\n", strings.Join(r.HintsOnScreen, " "))
		fmt.Printf("  handlers (code):   %s\n", strings.Join(r.HandlersInCode, " "))
		if len(r.Orphans) == 0 {
			fmt.Println("  orphans:           ✓ none")
		} else {
			fmt.Printf("  orphans:           ⚠ %s\n", strings.Join(r.Orphans, " "))
			totalOrphans += len(r.Orphans)
		}
		fmt.Println()
	}
	fmt.Printf("Total orphan hints across %d views: %d\n", len(rs), totalOrphans)
	if totalOrphans > 0 {
		os.Exit(2)
	}
}
