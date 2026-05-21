package manager_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	licensekg "github.com/oioio-space/maldev/license"
	"github.com/oioio-space/maldev/license/revoke"
	"github.com/oioio-space/maldev/license/totp"
)

// TestE2E_FullPipeline boots the full manager stack and walks through the
// typical operator flow: keypair generation, licence issuance with three
// kinds of binding, signature verification with all evidence, revocation,
// CRL publication via HTTP, and graceful shutdown.
func TestE2E_FullPipeline(t *testing.T) {
	ctx := context.Background()

	// Boot store + KEK + Services.
	st, err := store.New(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureSingletons(ctx, []byte("salt-16-byte-xxx"), []byte("canary")); err != nil {
		t.Fatal(err)
	}
	kek := crypto.DeriveFromPassphrase("test", [16]byte{1})
	svc := service.New(st, kek)
	defer svc.Close()

	// Generate + activate an issuer.
	iss, err := svc.Issuer.Generate(ctx, "lab", "k1", "op")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Issuer.SetActive(ctx, iss.ID, "op"); err != nil {
		t.Fatal(err)
	}

	// Create an identity for binary pinning.
	id, err := svc.Identity.Create(ctx, "rshell-v1", "op")
	if err != nil {
		t.Fatal(err)
	}

	// Issue a licence with three binding types + features.
	out, err := svc.License.Issue(ctx, service.IssueRequest{
		IssuerID:     iss.ID,
		Subject:      "alice@example.com",
		AudienceList: []string{"rshell"},
		Features:     []string{"export", "api"},
		IdentityID:   &id.ID,
		NotAfter:     time.Now().Add(7 * 24 * time.Hour),
		Bindings: []service.BindingSpec{
			{Type: "machine", Values: []string{"machine-1"}},
			{Type: "password", Values: []string{"hunter2"}},
			{Type: "totp"},
		},
		Actor: "op",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.TOTPs) != 1 {
		t.Fatalf("expected 1 TOTP provisioning, got %d", len(out.TOTPs))
	}

	// Verify the PEM with full evidence: license must accept.
	pubBytes, _ := svc.Issuer.ExportPublic(ctx, iss.ID)
	pub, kid, _ := licensekg.ParsePublicKey(pubBytes)
	trusted := licensekg.Trusted{Keys: licensekg.SingleKey(kid, pub)}

	code, _ := totp.Code(out.TOTPs[0].Secret, time.Now())
	v, err := licensekg.Verify(out.PEM, trusted,
		licensekg.WithAudience("rshell"),
		licensekg.WithMachineID([]byte("machine-1")),
		licensekg.WithPassword("hunter2"),
		licensekg.WithTOTPCode(code),
		licensekg.WithBinaryPinning(),
		licensekg.WithIdentityBytes(id.Bytes),
	)
	if err != nil {
		t.Fatalf("Verify with full evidence failed: %v", err)
	}
	if !v.HasFeature("export") {
		t.Fatal("export feature missing in verified licence")
	}

	// Revoke and confirm the row status updates.
	if err := svc.Revoke.Revoke(ctx, out.Row.ID, "test reason", "op"); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := svc.License.Get(ctx, out.Row.ID)
	if string(reloaded.Status) != "revoked" {
		t.Fatalf("status=%q want revoked", reloaded.Status)
	}

	// Start the revocation HTTP server on a free port.
	_, _ = svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdateOne) {
		u.SetRevocationListen("127.0.0.1:0")
	})
	revSrv := httpsrv.NewRevocationServer(svc.Revoke, svc.License, svc.Settings, svc.KEK)
	if err := revSrv.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer revSrv.Stop(2 * time.Second)

	addr := revSrv.Status().ListenAddr
	resp, err := http.Get("http://" + addr + "/revoked.pem")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	pem, _ := io.ReadAll(resp.Body)
	parsed, err := revoke.VerifyBytes(pem, pub, kid)
	if err != nil {
		t.Fatalf("CRL VerifyBytes: %v", err)
	}
	if len(parsed.Revoked) != 1 || parsed.Revoked[0] != out.Row.LicenseUUID {
		t.Fatalf("CRL doesn't contain the revoked licence: %+v", parsed.Revoked)
	}
	if len(parsed.Entries) != 1 || parsed.Entries[0].Reason != "test reason" {
		t.Fatalf("CRL entry metadata missing: %+v", parsed.Entries)
	}

	// Audit trail should contain every action.
	events, _ := svc.Audit.List(ctx, 50)
	if len(events) < 5 {
		t.Fatalf("expected at least 5 audit events, got %d", len(events))
	}
}
