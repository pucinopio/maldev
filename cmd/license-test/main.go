// cmd/license-test exercises the full license pipeline against an in-process
// HTTP server. Used as an end-to-end smoke test for releases.
package main

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oioio-space/maldev/license"
	"github.com/oioio-space/maldev/license/revoke"
	"github.com/oioio-space/maldev/license/server"
)

func main() {
	pub, priv, err := license.GenerateKey()
	must(err)

	revStore := server.FileStore(filepath.Join(os.TempDir(), "license-test-rev.json"))
	licStore := server.StaticLicenseStore{}

	mux := http.NewServeMux()
	mux.Handle("/revoked.pem", server.NewRevocationHandler(server.RevocationOptions{
		PrivateKey: priv, KeyID: "k1", Store: revStore, ValidFor: time.Hour, AdminToken: "ADM",
	}))
	mux.Handle("/heartbeat", server.NewHeartbeatHandler(server.HeartbeatOptions{
		PrivateKey: priv, KeyID: "k1", Store: licStore, ValidFor: time.Hour,
	}))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Five profiles exercising the major option families.
	passwordBinding, err := license.BindPassword("hunter2")
	must(err)
	profiles := []license.IssueOptions{
		{Subject: "plain", NotAfter: time.Now().Add(time.Hour)},
		{Subject: "audience", NotAfter: time.Now().Add(time.Hour), Audience: []string{"rshell"}},
		{Subject: "machine", NotAfter: time.Now().Add(time.Hour), Bindings: []license.Binding{license.BindMachineIDs("aa", "bb")}},
		{Subject: "password", NotAfter: time.Now().Add(time.Hour), Bindings: []license.Binding{passwordBinding}},
		{Subject: "identity", NotAfter: time.Now().Add(time.Hour), IdentitySHA256: license.HashIdentity([]byte("seed-XYZ"))},
	}
	issued := make([][]byte, len(profiles))
	for i, p := range profiles {
		p.PrivateKey = priv
		p.KeyID = "k1"
		data, err := license.Issue(p)
		must(err)
		issued[i] = data
		lic, _ := license.Inspect(data)
		licStore[lic.ID] = server.StatusActive
	}

	trusted := license.Trusted{Keys: map[string]ed25519.PublicKey{"k1": pub}}

	type verifCase struct {
		idx  int
		opts []license.VerifyOption
	}
	cases := []verifCase{
		{0, nil},
		{1, []license.VerifyOption{license.WithAudience("rshell")}},
		{2, []license.VerifyOption{license.WithMachineID([]byte("aa"))}},
		{3, []license.VerifyOption{license.WithPassword("hunter2")}},
		{4, []license.VerifyOption{license.WithBinaryPinning(), license.WithIdentityBytes([]byte("seed-XYZ"))}},
	}
	for _, c := range cases {
		if _, err := license.Verify(issued[c.idx], trusted, c.opts...); err != nil {
			log.Fatalf("license-test: profile %d verify failed: %v", c.idx, err)
		}
	}

	// Revoke profile 0 and confirm Verify rejects.
	info, _ := license.Inspect(issued[0])
	body := fmt.Sprintf(`{"add":[%q]}`, info.ID)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/revoked.pem", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer ADM")
	resp, err := http.DefaultClient.Do(req)
	must(err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("license-test: revoke POST status=%d", resp.StatusCode)
	}

	_, err = license.Verify(issued[0], trusted,
		license.WithRevocation(revoke.HTTPSource(srv.URL+"/revoked.pem", nil), time.Hour, ""),
		license.WithContext(context.Background()),
	)
	if err == nil {
		log.Fatal("license-test: revocation did not reject")
	}

	log.Print("license-test: PASS")
}

func must(err error) {
	if err != nil {
		log.Fatalf("license-test: %v", err)
	}
}
