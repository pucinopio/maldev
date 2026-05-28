// Tutorial 01 — verifier client.
//
// A real program a licensee would run. Loads a licence PEM and an
// issuer public-key PEM from disk, calls license.Verify, prints the
// verdict + the licence's subject / audience / features. Exits with
// code 0 on success, 1 on any verification error.
//
// Build:
//
//	go build -o /tmp/license-check ./examples/license-manager/tutorials/01-issue-and-verify/client
//
// Run:
//
//	/tmp/license-check \
//	    --license  /etc/myapp/license.pem \
//	    --issuer-pub /etc/myapp/issuer.pub
//
// Optional flags for bound licences:
//
//	--password <typed-by-user>
//	--machine  <hostid override>   (default: hostid.Composite())
//	--totp     <6-digit code>
package main

import (
	"flag"
	"fmt"
	"os"

	licensekg "github.com/oioio-space/maldev/license"
)

func main() {
	licPath := flag.String("license", "", "path to the licence PEM (signed)")
	pubPath := flag.String("issuer-pub", "", "path to the issuer public-key PEM")
	password := flag.String("password", "", "password evidence for a password-bound licence")
	machine := flag.String("machine", "", "machine-id evidence (empty → hostid.Composite())")
	totp := flag.String("totp", "", "6-digit TOTP code for a TOTP-bound licence")
	flag.Parse()

	if *licPath == "" || *pubPath == "" {
		fmt.Fprintln(os.Stderr, "usage: license-check --license <path> --issuer-pub <path> [--password ...] [--machine ...] [--totp ...]")
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

	opts := []licensekg.VerifyOption{}
	if *password != "" {
		opts = append(opts, licensekg.WithPassword(*password))
	}
	if *machine != "" {
		opts = append(opts, licensekg.WithMachineID([]byte(*machine)))
	}
	if *totp != "" {
		opts = append(opts, licensekg.WithTOTPCode(*totp))
	}

	v, err := licensekg.Verify(licPEM, trusted, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fail] verify: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[ok] licence verified")
	fmt.Printf("     subject:  %s\n", v.Subject)
	fmt.Printf("     issuer:   %s (key-id %s)\n", v.Issuer, kid)
	fmt.Printf("     features: %v\n", v.Features)
	fmt.Printf("     audience: %v\n", v.Audience)
	fmt.Printf("     not_after: %s\n", v.NotAfter.Format("2006-01-02"))
}
