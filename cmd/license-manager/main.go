package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "license-manager:", err)
		os.Exit(1)
	}
}

func run() error {
	flags := parseFlags()

	passphrase, err := resolvePassphrase(flags)
	if err != nil {
		return err
	}
	defer wipeString(&passphrase)

	ctx := context.Background()

	freshDB := !fileExists(flags.DBPath)
	st, err := store.New(ctx, flags.DBPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	// st is owned by svc after service.New; close it directly only on error
	// paths that occur before svc is constructed.
	var svc *service.Services
	defer func() {
		if svc == nil {
			_ = st.Close()
		}
	}()

	var kek *crypto.KEK
	if freshDB {
		salt, err := crypto.GenerateSalt()
		if err != nil {
			return err
		}
		kek = crypto.DeriveFromPassphrase(passphrase, salt)
		canary, err := crypto.NewCanary(kek)
		if err != nil {
			kek.Wipe()
			return err
		}
		if err := st.EnsureSingletons(ctx, salt[:], canary); err != nil {
			kek.Wipe()
			return err
		}
		fmt.Println("license-manager: new DB initialised at", flags.DBPath)
	} else {
		row, err := st.Client.Setting.Get(ctx, 1)
		if err != nil {
			return fmt.Errorf("read settings: %w", err)
		}
		if len(row.KekSalt) != 16 {
			return fmt.Errorf("corrupt kek_salt (len=%d)", len(row.KekSalt))
		}
		var salt [16]byte
		copy(salt[:], row.KekSalt)
		kek = crypto.DeriveFromPassphrase(passphrase, salt)
		if !kek.VerifyCanary(row.KekCanary) {
			kek.Wipe()
			return errors.New("wrong passphrase (canary mismatch)")
		}
	}

	svc = service.New(st, kek)
	defer func() { _ = svc.Close() }()

	if flags.NoTUI {
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

	// TUI launch lands here in a future task.
	fmt.Println("license-manager: TUI not implemented yet (run with --no-tui to smoke-test)")
	return nil
}

// resolvePassphrase walks the cascade:
//  1. --passphrase-file <path>
//  2. MALDEV_MGR_PASSPHRASE_FILE
//  3. MALDEV_MGR_PASSPHRASE
//  4. interactive terminal prompt (only if stdin is a TTY)
func resolvePassphrase(f cliFlags) (string, error) {
	if f.PassphraseFile != "" {
		return readPassphraseFile(f.PassphraseFile)
	}
	if path := os.Getenv("MALDEV_MGR_PASSPHRASE_FILE"); path != "" {
		return readPassphraseFile(path)
	}
	if env := os.Getenv("MALDEV_MGR_PASSPHRASE"); env != "" {
		return env, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("no passphrase source available (set MALDEV_MGR_PASSPHRASE or run interactively)")
	}
	fmt.Print("Passphrase: ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read passphrase: %w", err)
	}
	return string(b), nil
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

// wipeString zeroes the underlying string value pointed to by p. Go strings
// are immutable values; this replaces with a null-filled string of the same
// length as a best-effort wipe before the GC reclaims the original allocation.
func wipeString(p *string) {
	if p == nil || *p == "" {
		return
	}
	*p = strings.Repeat("\x00", len(*p))
}
