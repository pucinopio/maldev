package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "license-manager:", err)
		os.Exit(1)
	}
}

func run() error {
	flags := parseFlags()
	ctx := context.Background()

	freshDB := !fileExists(flags.DBPath)

	if freshDB {
		return runOnboarding(flags)
	}

	// Existing DB — open the store first.
	st, err := store.New(ctx, flags.DBPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}

	// Resolve passphrase via cascade (no TTY prompt yet — that goes to TUI).
	passphrase := resolvePassphraseSilent(flags)

	if passphrase == "" {
		// Cascade unresolved → run passphrase-prompt sub-program.
		p := tea.NewProgram(tui.NewPassphrasePrompt(st, flags.DBPath), tea.WithAltScreen())
		result, err := p.Run()
		if err != nil {
			_ = st.Close()
			return err
		}
		rp, ok := result.(tui.ResolvedPassphrase)
		if !ok || rp.ResolvedPassphrase() == "" {
			_ = st.Close()
			return errors.New("authentication failed")
		}
		passphrase = rp.ResolvedPassphrase()
	}
	defer wipeString(&passphrase)

	kek, err := tryUnlock(ctx, st, passphrase)
	if err != nil {
		_ = st.Close()
		return err
	}

	svc := service.New(st, kek)
	defer func() { _ = svc.Close() }()

	if flags.NoTUI {
		return printSmokeSummary(ctx, svc)
	}

	root := tui.New(svc, nil /* httpsrv wired in Phase 4 */, tui.SessionReady)
	p := tea.NewProgram(root, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// runOnboarding launches the first-launch wizard for a fresh DB.
// The wizard collects passphrase + issuer details but does not write to disk
// in Phase 1; it prints a restart prompt so the caller can wire the store.
func runOnboarding(flags cliFlags) error {
	root := tui.New(nil, nil, tui.SessionOnboarding)
	p := tea.NewProgram(root, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "license-manager: onboarding complete — restart to use the DB at", flags.DBPath)
	return nil
}

// resolvePassphraseSilent walks the cascade without opening a TTY prompt.
// Returns "" when no source is available.
func resolvePassphraseSilent(f cliFlags) string {
	if f.PassphraseFile != "" {
		s, err := readPassphraseFile(f.PassphraseFile)
		if err == nil {
			return s
		}
	}
	if path := os.Getenv("MALDEV_MGR_PASSPHRASE_FILE"); path != "" {
		s, err := readPassphraseFile(path)
		if err == nil {
			return s
		}
	}
	if env := os.Getenv("MALDEV_MGR_PASSPHRASE"); env != "" {
		return env
	}
	// Terminal prompt — only when running with --no-tui so we don't fight bubbletea.
	if f.NoTUI && term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Print("Passphrase: ")
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err == nil {
			return string(b)
		}
	}
	return ""
}

// tryUnlock derives the KEK from passphrase and verifies the stored canary.
func tryUnlock(ctx context.Context, st *store.Store, passphrase string) (*crypto.KEK, error) {
	row, err := st.Client.Setting.Get(ctx, 1)
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	if len(row.KekSalt) != 16 {
		return nil, fmt.Errorf("corrupt kek_salt (len=%d)", len(row.KekSalt))
	}
	var salt [16]byte
	copy(salt[:], row.KekSalt)
	kek := crypto.DeriveFromPassphrase(passphrase, salt)
	if !kek.VerifyCanary(row.KekCanary) {
		kek.Wipe()
		return nil, errors.New("wrong passphrase (canary mismatch)")
	}
	return kek, nil
}

// printSmokeSummary prints a brief status report and returns (--no-tui mode).
func printSmokeSummary(ctx context.Context, svc *service.Services) error {
	fmt.Println("license-manager: boot ok")
	settings, err := svc.Settings.Get(ctx)
	if err != nil {
		return fmt.Errorf("get settings: %w", err)
	}
	fmt.Printf("  operator: %s\n", settings.OperatorName)
	fmt.Printf("  default_ttl_seconds: %d\n", settings.DefaultTTLSeconds)
	fmt.Printf("  auto_start_servers: %v\n", settings.AutoStartServers)
	issuers, err := svc.Issuer.List(ctx)
	if err != nil {
		return fmt.Errorf("list issuers: %w", err)
	}
	fmt.Printf("  issuers: %d\n", len(issuers))
	return nil
}

func readPassphraseFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return strings.TrimSpace(string(b)), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// wipeString zeroes the underlying string value as a best-effort wipe before
// the GC reclaims the original allocation.
func wipeString(p *string) {
	if p == nil || *p == "" {
		return
	}
	*p = strings.Repeat("\x00", len(*p))
}
