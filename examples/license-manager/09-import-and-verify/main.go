// 09-import-and-verify — runnable companion to README.md.
//
// Simulates moving a licence between two manager instances:
// instance A issues, exports the issuer's public key; instance B
// imports the PEM, verifies it via the standalone license/
// package, and proves the licence survives a manager handoff.
//
// Build + run:
//
//	go run ./examples/license-manager/09-import-and-verify
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"

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

func run(ctx context.Context, stdout, stderr io.Writer) error {
	// ── Instance A: issue ───────────────────────────────────────────
	svcA, cleanupA, err := bootInMemory(ctx)
	if err != nil {
		return err
	}
	defer cleanupA()
	iss, _ := svcA.Issuer.Generate(ctx, "lab-a", "handoff-demo", "operator")
	_ = svcA.Issuer.SetActive(ctx, iss.ID, "operator")
	out, err := svcA.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "eve@example.com",
		NotAfter: time.Now().Add(30 * 24 * time.Hour),
		Actor:    "operator",
	})
	if err != nil {
		return err
	}
	pubPEM, _ := svcA.Issuer.ExportPublic(ctx, iss.ID)
	pem := out.PEM
	originalUUID := out.Row.LicenseUUID
	fmt.Fprintf(stderr, "[ok] instance-A issued (uuid: %s)\n", originalUUID)

	// ── Instance B: import + verify ─────────────────────────────────
	// Instance B has a different KEK / DB, never saw the private key
	// that signed the licence. It needs only the issuer's PUBLIC key
	// to verify.
	svcB, cleanupB, err := bootInMemory(ctx)
	if err != nil {
		return err
	}
	defer cleanupB()
	// Import the issuer (public + private). For a public-key-only
	// verification we'd skip this and only feed pubPEM to Verify.
	if _, err := svcB.Issuer.Import(ctx, "lab-a", iss.KeyID, mustExportPriv(ctx, svcA, iss.ID), "operator"); err != nil {
		return fmt.Errorf("Issuer.Import: %w", err)
	}
	row, err := svcB.License.Import(ctx, pem, "handoff", "operator")
	if err != nil {
		return fmt.Errorf("License.Import: %w", err)
	}
	if row.LicenseUUID != originalUUID {
		return fmt.Errorf("imported UUID drift: got %s want %s", row.LicenseUUID, originalUUID)
	}
	fmt.Fprintf(stderr, "[ok] instance-B imported (uuid preserved: %s)\n", row.LicenseUUID)

	// Standalone verify with public key only — exact path a deployed
	// binary follows.
	pub, kid, _ := licensekg.ParsePublicKey(pubPEM)
	trusted := licensekg.Trusted{Keys: licensekg.SingleKey(kid, pub)}
	if _, err := licensekg.Verify(pem, trusted); err != nil {
		return fmt.Errorf("Verify: %w", err)
	}
	fmt.Fprintln(stderr, "[ok] verify GREEN with public key only")

	_, err = stdout.Write(pem)
	return err
}

func mustExportPriv(ctx context.Context, svc *service.Services, id uuid.UUID) []byte {
	b, err := svc.Issuer.ExportPrivate(ctx, id)
	if err != nil {
		panic(err)
	}
	return b
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
