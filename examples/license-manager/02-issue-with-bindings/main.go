// 02-issue-with-bindings — runnable companion to README.md.
//
// Issues a licence whose verification requires three pieces of
// evidence the licensed binary must collect at runtime:
//
//   1. machine-id — the host's hostid.Composite() must match one
//      of the values stamped into the licence at issue time.
//   2. password   — the operator types a passphrase the licence
//      bound through an Argon2id derivation. We round-trip
//      "hunter2" through the issue + verify chain.
//   3. TOTP       — a 30-second one-time code generated from a
//      shared secret the manager handed back at issue time.
//
// Verification fails if ANY of the three is missing or wrong.
//
// Build + run:
//
//	go run ./examples/license-manager/02-issue-with-bindings
//
// Tested by main_test.go.
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
	"github.com/oioio-space/maldev/license/totp"
)

func main() {
	if err := run(context.Background(), os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "[err] %v\n", err)
		os.Exit(1)
	}
}

// run boots an in-memory manager, issues a licence with three
// bindings, and verifies it against synthetic operator evidence.
// Exposed so main_test.go re-uses the same code path.
func run(ctx context.Context, stdout, stderr io.Writer) error {
	// ── Boot ─────────────────────────────────────────────────────────
	svc, cleanup, err := bootInMemory(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	fmt.Fprintln(stderr, "[ok] services up")

	iss, _ := svc.Issuer.Generate(ctx, "lab", "bindings-demo", "operator")
	_ = svc.Issuer.SetActive(ctx, iss.ID, "operator")

	// ── Issue with 3 bindings ────────────────────────────────────────
	// The wizard's TOTP step seeds a fresh base32 secret and stamps a
	// commitment hash into the licence. The plaintext secret is
	// returned to the caller via IssuedLicense.TOTPs[i].Secret so the
	// operator can hand it to the licensee (typically as a QR code).
	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "alice@example.com",
		AudienceList: []string{"demo"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
		Bindings: []service.BindingSpec{
			{Type: "machine", Values: []string{"host-alpha", "host-beta"}},
			{Type: "password", Values: []string{"hunter2"}},
			{Type: "totp"},
		},
		Actor: "operator",
	})
	if err != nil {
		return fmt.Errorf("Issue: %w", err)
	}
	if len(out.TOTPs) != 1 {
		return fmt.Errorf("expected 1 TOTP provisioning, got %d", len(out.TOTPs))
	}
	totpSecret := out.TOTPs[0].Secret
	fmt.Fprintf(stderr, "[ok] licence issued (uuid: %s)\n", out.Row.LicenseUUID)
	fmt.Fprintf(stderr, "[ok] TOTP secret returned out-of-band: %s\n", totpSecret)

	// ── Build the verifier ──────────────────────────────────────────
	pubPEM, _ := svc.Issuer.ExportPublic(ctx, iss.ID)
	pub, kid, _ := licensekg.ParsePublicKey(pubPEM)
	trusted := licensekg.Trusted{Keys: licensekg.SingleKey(kid, pub)}

	// ── Verify with FULL evidence — should succeed ───────────────────
	code, err := totp.Code(totpSecret, time.Now())
	if err != nil {
		return fmt.Errorf("totp.Code: %w", err)
	}
	v, err := licensekg.Verify(out.PEM, trusted,
		licensekg.WithMachineID([]byte("host-alpha")),
		licensekg.WithPassword("hunter2"),
		licensekg.WithTOTPCode(code),
	)
	if err != nil {
		return fmt.Errorf("Verify (full evidence): %w", err)
	}
	fmt.Fprintf(stderr, "[ok] verify GREEN with full evidence (features: %v)\n", v.Features)

	// ── Verify with MISSING evidence — should fail ──────────────────
	// Demonstrates the "all bindings must be satisfied" invariant: drop
	// the password and the verifier rejects the licence even though
	// the signature is valid.
	if _, err := licensekg.Verify(out.PEM, trusted,
		licensekg.WithMachineID([]byte("host-alpha")),
		licensekg.WithTOTPCode(code),
	); err == nil {
		return fmt.Errorf("Verify accepted licence WITHOUT password — binding skipped")
	}
	fmt.Fprintln(stderr, "[ok] verify RED when password evidence missing (expected)")

	// ── Verify with WRONG machine — should fail ─────────────────────
	if _, err := licensekg.Verify(out.PEM, trusted,
		licensekg.WithMachineID([]byte("host-charlie")),
		licensekg.WithPassword("hunter2"),
		licensekg.WithTOTPCode(code),
	); err == nil {
		return fmt.Errorf("Verify accepted licence on wrong machine")
	}
	fmt.Fprintln(stderr, "[ok] verify RED when machine doesn't match (expected)")

	// PEM goes to stdout so the operator can save it.
	_, err = stdout.Write(out.PEM)
	return err
}

// bootInMemory spins up an in-memory store + KEK + services. Returned
// cleanup must be called before exit so the SQLite handle closes.
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
	svc := service.New(st, kek)
	return svc, func() { svc.Close(); st.Close() }, nil
}
