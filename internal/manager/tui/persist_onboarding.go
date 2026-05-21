package tui

import (
	"context"
	"fmt"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store"
)

// PersistOnboarding writes the initial DB state collected by the onboarding
// wizard.  It:
//
//  1. Opens (creates) the SQLite store at dbPath and runs schema migrations.
//  2. Derives the KEK from the supplied passphrase with a fresh random salt.
//  3. Generates a canary and writes the Setting singleton.
//  4. Creates the first Issuer (Ed25519 keypair, encrypted under the KEK).
//
// The function is intentionally pure of bubbletea — main.go calls it after
// the tea.Program exits, and the test suite calls it directly without a TTY.
func PersistOnboarding(ctx context.Context, dbPath string, msg OnboardingDoneMsg) error {
	st, err := store.New(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("persist onboarding: open store: %w", err)
	}

	salt, err := crypto.GenerateSalt()
	if err != nil {
		_ = st.Close()
		return fmt.Errorf("persist onboarding: generate salt: %w", err)
	}

	kek := crypto.DeriveFromPassphrase(msg.Passphrase, salt)

	canary, err := crypto.NewCanary(kek)
	if err != nil {
		kek.Wipe()
		_ = st.Close()
		return fmt.Errorf("persist onboarding: create canary: %w", err)
	}

	if err := st.EnsureSingletons(ctx, salt[:], canary); err != nil {
		kek.Wipe()
		_ = st.Close()
		return fmt.Errorf("persist onboarding: singletons: %w", err)
	}

	// svc.Close() wipes the KEK and closes st — it owns both from here on.
	svc := service.New(st, kek)

	iss, err := svc.Issuer.Generate(ctx, msg.IssuerName, msg.IssuerKeyID, "operator")
	if err != nil {
		_ = svc.Close()
		return fmt.Errorf("persist onboarding: create issuer: %w", err)
	}

	if err := svc.Issuer.SetActive(ctx, iss.ID, "operator"); err != nil {
		_ = svc.Close()
		return fmt.Errorf("persist onboarding: set active issuer: %w", err)
	}

	return svc.Close()
}
