// Tutorial 03 — verifier client that fetches the CRL from a
// running revocation server before deciding whether to accept the
// licence. Same code path a deployed binary follows: bundle the
// issuer public key, pass the manager's HTTPS URL, refresh on a
// schedule, fall back to the on-disk cache if the manager is
// unreachable.
//
// Build:
//
//	go build -o /tmp/license-check-3 \
//	  ./examples/license-manager/tutorials/03-revocation-server/client
//
// Run:
//
//	/tmp/license-check-3 \
//	    --license  /etc/myapp/license.pem \
//	    --issuer-pub /etc/myapp/issuer.pub \
//	    --crl-url   http://localhost:8443/revoked.pem \
//	    --crl-cache /var/cache/myapp/crl.pem
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	licensekg "github.com/oioio-space/maldev/license"
	"github.com/oioio-space/maldev/license/revoke"
)

func main() {
	licPath := flag.String("license", "", "path to the licence PEM")
	pubPath := flag.String("issuer-pub", "", "path to the issuer public-key PEM")
	crlURL := flag.String("crl-url", "", "URL of the manager's GET /revoked.pem endpoint")
	crlCache := flag.String("crl-cache", "", "(optional) on-disk CRL cache path — survives manager outages")
	refresh := flag.Duration("crl-refresh", time.Hour, "how often to re-fetch the CRL")
	flag.Parse()

	if *licPath == "" || *pubPath == "" || *crlURL == "" {
		fmt.Fprintln(os.Stderr, "usage: license-check-3 --license <p> --issuer-pub <p> --crl-url <url> [--crl-cache <p>] [--crl-refresh <d>]")
		os.Exit(2)
	}

	licPEM, err := os.ReadFile(*licPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read licence: %v\n", err)
		os.Exit(1)
	}
	pubPEM, err := os.ReadFile(*pubPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read issuer pub: %v\n", err)
		os.Exit(1)
	}
	pub, kid, err := licensekg.ParsePublicKey(pubPEM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse issuer pub: %v\n", err)
		os.Exit(1)
	}
	trusted := licensekg.Trusted{Keys: licensekg.SingleKey(kid, pub)}

	// HTTPSource fetches /revoked.pem; if the optional cachePath is
	// passed, license.Verify keeps a copy on disk and replays it
	// when the manager is unreachable. The Sequence number stamped
	// into every CRL prevents stale-cache downgrade attacks.
	src := revoke.HTTPSource(*crlURL, &http.Client{Timeout: 5 * time.Second})
	opts := []licensekg.VerifyOption{
		licensekg.WithRevocation(src, *refresh, *crlCache),
	}

	v, err := licensekg.Verify(licPEM, trusted, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fail] verify: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[ok] licence verified — not on CRL")
	fmt.Printf("     subject:  %s\n", v.Subject)
	fmt.Printf("     issuer:   %s (key-id %s)\n", v.Issuer, kid)
	fmt.Printf("     crl_url:  %s\n", *crlURL)
}
