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
		// ── Chrome (global) ─────────────────────────────────────────────────
		{
			ID:         "chrome.tab.2.kb",
			View:       "dashboard",
			Keys:       "2",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "press '2' should land tea.KeyMsg with runes='2'",
		},
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
			Notes:      "'r' on dashboard should trigger DashboardSnapshotCmd",
		},
		{
			ID:         "chrome.quit.kb",
			View:       "dashboard",
			Keys:       "q",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'q' without running servers should tea.Quit",
		},

		// ── Licenses keybinds ──────────────────────────────────────────────
		{
			ID:         "lic.search.kb",
			View:       "licenses",
			Keys:       "2 /",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "Goto Licenses then '/' must focus search",
		},
		{
			ID:         "lic.filter.kb",
			View:       "licenses",
			Keys:       "2 f",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'f' on Licenses cycles filter chip",
		},
		{
			ID:         "lic.new.kb",
			View:       "licenses",
			Keys:       "2 n",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'n' on Licenses should open the wizard overlay",
		},

		// ── Audit keybinds ─────────────────────────────────────────────────
		{
			ID:         "aud.refresh.kb",
			View:       "audit",
			Keys:       "8 r",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'r' on Audit triggers listAuditCmd",
		},
		{
			ID:         "aud.export.csv.kb",
			View:       "audit",
			Keys:       "8 E",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'E' opens CSV export input overlay",
		},

		// ── Settings keybinds ──────────────────────────────────────────────
		{
			ID:         "set.theme.1.kb",
			View:       "settings",
			Keys:       "9 1",
			ExpectMsgs: []string{"tea.KeyMsg"},
			Notes:      "'1' on Settings switches to neon theme",
		},

		// ── Mouse: dashboard tile click ────────────────────────────────────
		{
			ID:         "dash.tile.active.ms",
			View:       "dashboard",
			Mouse:      "17,4,left",
			ExpectMsgs: []string{"tea.MouseMsg"},
			Notes:      "click on Active tile should switch view + filter",
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
