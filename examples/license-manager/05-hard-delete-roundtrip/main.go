// 05-hard-delete-roundtrip — runnable companion to README.md.
//
// Issues a licence, exports the PEM, hard-deletes the row, then
// re-imports the PEM and proves the same UUID lands in the DB
// without violating the unique constraint. Demonstrates the
// "export → delete → re-import" workflow operators use to move
// licences between manager instances.
//
// Build + run:
//
//	go run ./examples/license-manager/05-hard-delete-roundtrip
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

	iss, _ := svc.Issuer.Generate(ctx, "lab", "delete-demo", "operator")
	_ = svc.Issuer.SetActive(ctx, iss.ID, "operator")

	issued, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID: iss.ID, Subject: "dave@example.com",
		NotAfter: time.Now().Add(30 * 24 * time.Hour),
		Actor:    "operator",
	})
	if err != nil {
		return err
	}
	originalUUID := issued.Row.LicenseUUID
	pem := issued.PEM
	fmt.Fprintf(stderr, "[ok] issued (uuid: %s)\n", originalUUID)

	// Hard delete: License + Revocation + TOTPSecret cascade, audit
	// retained, unique license_uuid freed.
	if err := svc.License.Delete(ctx, issued.Row.ID, "operator"); err != nil {
		return fmt.Errorf("Delete: %w", err)
	}
	if _, err := svc.License.GetByUUID(ctx, originalUUID); err == nil {
		return fmt.Errorf("licence still queryable after Delete")
	}
	fmt.Fprintln(stderr, "[ok] hard-deleted (audit kept, uuid freed)")

	// Re-import the same PEM — would have failed pre-delete on the
	// unique license_uuid constraint.
	row, err := svc.License.Import(ctx, pem, "re-import", "operator")
	if err != nil {
		return fmt.Errorf("Import: %w", err)
	}
	if row.LicenseUUID != originalUUID {
		return fmt.Errorf("re-imported UUID = %q, want %q", row.LicenseUUID, originalUUID)
	}
	fmt.Fprintf(stderr, "[ok] re-imported with same uuid %s\n", row.LicenseUUID)

	_, err = stdout.Write(pem)
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
