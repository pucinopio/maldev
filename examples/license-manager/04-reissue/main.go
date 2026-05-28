// 04-reissue — runnable companion to README.md.
//
// Re-issues an existing licence, demonstrating the supersession
// chain: the new licence inherits bindings/audience/features from
// the original; the operator overrides only the fields they want
// to change (here: extend NotAfter by 90 days, add a feature).
// The original is marked superseded — verify still works for both
// PEMs, but a chain walk shows the lineage.
//
// Build + run:
//
//	go run ./examples/license-manager/04-reissue
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store"
	licenseent "github.com/oioio-space/maldev/internal/manager/store/ent/license"
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

	iss, _ := svc.Issuer.Generate(ctx, "lab", "reissue-demo", "operator")
	_ = svc.Issuer.SetActive(ctx, iss.ID, "operator")

	// Original licence: 30 days, no features.
	orig, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "carol@example.com",
		AudienceList: []string{"demo"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(30 * 24 * time.Hour),
		Actor:        "operator",
	})
	if err != nil {
		return fmt.Errorf("Issue: %w", err)
	}
	fmt.Fprintf(stderr, "[ok] original issued (uuid: %s)\n", orig.Row.LicenseUUID)

	// Re-issue: extend by 90 days and add a feature. Empty fields in
	// ReIssueOptions inherit from the original — only the explicit
	// overrides change.
	reissued, err := svc.License.ReIssue(ctx, orig.Row.ID, service.ReIssueOptions{
		NotAfter: time.Now().Add(120 * 24 * time.Hour),
		Features: []string{"extended"},
		Payload:  json.RawMessage(`{"tier":"gold"}`),
		Actor:    "operator",
	})
	if err != nil {
		return fmt.Errorf("ReIssue: %w", err)
	}
	fmt.Fprintf(stderr, "[ok] re-issued (uuid: %s)\n", reissued.Row.LicenseUUID)

	// ── Original is now superseded ──────────────────────────────────
	origRow, _ := svc.License.Get(ctx, orig.Row.ID)
	if origRow.Status != licenseent.StatusSuperseded {
		return fmt.Errorf("original status = %v, want superseded", origRow.Status)
	}
	fmt.Fprintln(stderr, "[ok] original marked superseded")

	// ── Chain walk: parents / this / successors ─────────────────────
	chain, err := svc.License.GetChain(ctx, reissued.Row.ID)
	if err != nil {
		return fmt.Errorf("GetChain: %w", err)
	}
	if len(chain.Parents) != 1 || chain.Parents[0].LicenseUUID != orig.Row.LicenseUUID {
		return fmt.Errorf("chain.Parents = %d, want 1 with original UUID", len(chain.Parents))
	}
	if chain.This.LicenseUUID != reissued.Row.LicenseUUID {
		return fmt.Errorf("chain.This UUID mismatch")
	}
	fmt.Fprintf(stderr, "[ok] chain walk: 1 parent → this licence → 0 successors\n")

	_, err = stdout.Write(reissued.PEM)
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
