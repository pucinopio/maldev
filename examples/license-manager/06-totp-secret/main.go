// 06-totp-secret — runnable companion to README.md.
//
// Generates a standalone TOTP secret (not tied to a licence) the
// way the TUI's [n] action on the TOTP tab does, renders the
// otpauth URI + the ASCII half-block QR a phone authenticator
// can scan, and computes the current 6-digit code so the operator
// can sanity-check the setup before handing the QR to the
// licensee.
//
// Build + run:
//
//	go run ./examples/license-manager/06-totp-secret
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/license/totp"
)

func main() {
	if err := run(context.Background(), os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "[err] %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, stdout, stderr io.Writer) error {
	svc, cleanup, err := bootInMemory(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	fmt.Fprintln(stderr, "[ok] services up")

	// Generate the secret + persist the row (encrypted under the KEK).
	row, secret, err := svc.TOTP.Generate(ctx, "alice@example.com")
	if err != nil {
		return fmt.Errorf("TOTP.Generate: %w", err)
	}
	fmt.Fprintf(stderr, "[ok] secret created (id: %s)\n", row.ID)

	// Fetch the decrypted view (otpauth URI + QR).
	view, err := svc.TOTP.ByID(ctx, row.ID, "license-manager")
	if err != nil {
		return fmt.Errorf("TOTP.ByID: %w", err)
	}
	fmt.Fprintln(stderr, "[ok] view materialised (URI + QR)")

	// Round-trip: compute the current code from the secret and verify
	// it matches the issuer-side helper.
	code, err := totp.Code(secret, time.Now())
	if err != nil {
		return fmt.Errorf("totp.Code: %w", err)
	}
	if len(code) != 6 {
		return fmt.Errorf("code length = %d, want 6", len(code))
	}
	fmt.Fprintf(stderr, "[ok] 6-digit code for now: %s\n", code)

	// Print: URI + ASCII QR (operators paste the QR into the licensee
	// onboarding doc when no terminal is available).
	fmt.Fprintln(stdout, view.OtpauthURI)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, view.QRImageASCII)
	return nil
}

func bootInMemory(ctx context.Context) (*service.Services, func(), error) {
	st, err := store.New(ctx, ":memory:")
	if err != nil {
		return nil, nil, err
	}
	salt := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	kek := crypto.DeriveFromPassphrase("demo", salt)
	canary, err := crypto.NewCanary(kek)
	if err != nil {
		st.Close()
		return nil, nil, err
	}
	if err := st.EnsureSingletons(ctx, salt[:], canary); err != nil {
		st.Close()
		return nil, nil, err
	}
	return service.New(st, kek), func() { st.Close() }, nil
}
