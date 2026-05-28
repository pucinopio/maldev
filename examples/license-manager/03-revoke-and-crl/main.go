// 03-revoke-and-crl — runnable companion to README.md.
//
// Issues a licence, revokes it with a reason, signs + publishes
// the CRL, and verifies a binary armed with the CRL would reject
// the revoked licence.
//
// Build + run:
//
//	go run ./examples/license-manager/03-revoke-and-crl
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
	"github.com/oioio-space/maldev/license/revoke"
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

	iss, _ := svc.Issuer.Generate(ctx, "lab", "crl-demo", "operator")
	_ = svc.Issuer.SetActive(ctx, iss.ID, "operator")

	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "bob@example.com",
		AudienceList: []string{"demo"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
		Actor:        "operator",
	})
	if err != nil {
		return fmt.Errorf("Issue: %w", err)
	}
	fmt.Fprintf(stderr, "[ok] licence issued (uuid: %s)\n", out.Row.LicenseUUID)

	// ── Revoke ───────────────────────────────────────────────────────
	// Marks status=revoked AND inserts a Revocation row carrying the
	// reason + revoked_at + actor — all in one transaction with the
	// matching audit event.
	if err := svc.Revoke.Revoke(ctx, out.Row.ID, "key compromise", "operator"); err != nil {
		return fmt.Errorf("Revoke: %w", err)
	}
	fmt.Fprintln(stderr, "[ok] licence marked revoked (reason: key compromise)")

	// ── Publish the signed CRL ───────────────────────────────────────
	// The revocation server's HTTP handler calls this on every GET
	// /revoked.pem so the CRL is always fresh. The signed PEM is
	// cached for validFor/2 — repeated GETs during a quiet period
	// reuse it.
	crlPEM, err := svc.Revoke.PublishSignedList(ctx, 7*24*time.Hour)
	if err != nil {
		return fmt.Errorf("PublishSignedList: %w", err)
	}
	fmt.Fprintf(stderr, "[ok] CRL signed + published (%d bytes)\n", len(crlPEM))

	// ── Verify a binary armed with the CRL rejects the licence ──────
	// This is exactly the chain a deployed binary runs: pull the CRL
	// from the manager's revocation server, parse it, pass it to
	// licensekg.Verify which then checks the licence's UUID isn't on
	// the list.
	pubPEM, _ := svc.Issuer.ExportPublic(ctx, iss.ID)
	pub, kid, _ := licensekg.ParsePublicKey(pubPEM)
	trusted := licensekg.Trusted{Keys: licensekg.SingleKey(kid, pub)}

	// Sanity-check the CRL parses standalone (operator could ship it
	// as a static file via `revoke.FileSource`).
	if _, err := revoke.VerifyBytes(crlPEM, pub, kid); err != nil {
		return fmt.Errorf("revoke.VerifyBytes: %w", err)
	}

	// Hand it to Verify via EmbedSource — same chain a binary using
	// revoke.HTTPSource("https://manager:8443/revoked.pem") follows
	// at runtime.
	if _, err := licensekg.Verify(out.PEM, trusted,
		licensekg.WithRevocation(revoke.EmbedSource(crlPEM), time.Hour, ""),
	); err == nil {
		return fmt.Errorf("Verify accepted a revoked licence — CRL not consulted")
	}
	fmt.Fprintln(stderr, "[ok] verify RED with CRL consulted (revocation enforced)")

	// Without the CRL the licence still verifies — the manager's CRL
	// is the source of truth, not the licence row itself.
	if _, err := licensekg.Verify(out.PEM, trusted); err != nil {
		return fmt.Errorf("Verify without CRL must succeed (signature still valid): %w", err)
	}
	fmt.Fprintln(stderr, "[ok] verify GREEN without CRL (signature still valid)")

	_, err = stdout.Write(crlPEM)
	return err
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
