// 01-issue-basic — runnable companion to examples/license-manager/README.md.
//
// Smallest possible round-trip: boot an in-memory license-manager,
// generate one issuer, issue one licence, verify it through the
// standalone license/ package, print the PEM to stdout.
//
// Build + run:
//
//	go run ./examples/license-manager/01-issue-basic
//
// Tested by main_test.go against the same in-memory pipeline.
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
	licensekg "github.com/oioio-space/maldev/license"
)

func main() {
	if err := run(context.Background(), os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "[err] %v\n", err)
		os.Exit(1)
	}
}

// run drives the example end-to-end. stdout receives the PEM only;
// stderr receives the human-readable status lines so the example
// can be piped (`example > file.pem`) without manual parsing.
// Exposed so main_test.go can re-use it.
func run(ctx context.Context, stdout, stderr io.Writer) error {
	// ── 1. Boot an in-memory manager ─────────────────────────────────
	// Production callers resolve the passphrase via the cascade
	// described in docs/license-manager/concepts.md. An example uses
	// a fixed deterministic value.
	st, err := store.New(ctx, ":memory:")
	if err != nil {
		return fmt.Errorf("store.New: %w", err)
	}
	defer st.Close()

	salt := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	kek := crypto.DeriveFromPassphrase("demo-passphrase", salt)
	canary, err := crypto.NewCanary(kek)
	if err != nil {
		return fmt.Errorf("crypto.NewCanary: %w", err)
	}
	if err := st.EnsureSingletons(ctx, salt[:], canary); err != nil {
		return fmt.Errorf("EnsureSingletons: %w", err)
	}
	svc := service.New(st, kek)
	defer svc.Close()
	fmt.Fprintln(stderr, "[ok] services up (in-memory SQLite, KEK derived)")

	// ── 2. Create the first issuer ────────────────────────────────────
	// In a real workflow the operator does this once via the wizard.
	// IssuerService.Generate creates the Ed25519 pair, wraps the
	// private key under the KEK, and writes an audit event.
	iss, err := svc.Issuer.Generate(ctx, "lab", "demo-2026-q2", "demo-operator")
	if err != nil {
		return fmt.Errorf("Issuer.Generate: %w", err)
	}
	if err := svc.Issuer.SetActive(ctx, iss.ID, "demo-operator"); err != nil {
		return fmt.Errorf("Issuer.SetActive: %w", err)
	}
	fmt.Fprintf(stderr, "[ok] issuer %q created (key-id: %s)\n", iss.Name, iss.KeyID)

	// ── 3. Issue a licence ────────────────────────────────────────────
	// Subject, audience, validity window — the bare minimum the
	// license/ package needs to produce a verifiable PEM. No bindings:
	// any binary that trusts this issuer's public key will accept it.
	issued, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID:     iss.ID,
		Subject:      "alice@example.com",
		AudienceList: []string{"demo"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
		Features:     []string{"basic"},
		Actor:        "demo-operator",
	})
	if err != nil {
		return fmt.Errorf("License.Issue: %w", err)
	}
	fmt.Fprintf(stderr, "[ok] licence issued for subject %q (uuid: %s)\n",
		issued.Row.Subject, issued.Row.LicenseUUID)

	// ── 4. Self-verify the round-trip ────────────────────────────────
	// Export the issuer's public key the same way the TUI's [E] action
	// would, then hand it to license.Verify alongside the PEM. This
	// catches any drift between Issue's wire format and Verify's
	// expectations — exactly what a binary in the field would do.
	pubPEM, err := svc.Issuer.ExportPublic(ctx, iss.ID)
	if err != nil {
		return fmt.Errorf("Issuer.ExportPublic: %w", err)
	}
	pub, kid, err := licensekg.ParsePublicKey(pubPEM)
	if err != nil {
		return fmt.Errorf("ParsePublicKey: %w", err)
	}
	trusted := licensekg.Trusted{Keys: licensekg.SingleKey(kid, pub)}
	if _, err := licensekg.Verify(issued.PEM, trusted); err != nil {
		return fmt.Errorf("Verify: %w", err)
	}
	fmt.Fprintln(stderr, "[ok] verify round-trip green")

	// ── 5. Emit the PEM on stdout ────────────────────────────────────
	if _, err := stdout.Write(issued.PEM); err != nil {
		return fmt.Errorf("write PEM: %w", err)
	}
	return nil
}
