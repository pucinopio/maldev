// Tutorial 02 — verifier client that collects three evidence
// pieces at startup: a machine id (from hostid.Composite()), a
// password (typed by the user, read from --password flag for the
// E2E demo), and a 6-digit TOTP code (read from --totp).
//
// Refuses to start unless ALL three match the bindings stamped
// into the licence at issue time.
//
// Build:
//
//	go build -o /tmp/license-check-2 \
//	  ./examples/license-manager/tutorials/02-bindings-and-verify/client
//
// Run (interactive — collects evidence from flags here for
// reproducibility; a real binary would prompt the user instead):
//
//	/tmp/license-check-2 \
//	    --license  /etc/myapp/license.pem \
//	    --issuer-pub /etc/myapp/issuer.pub \
//	    --machine  host-alpha \
//	    --password hunter2 \
//	    --totp     123456
package main

import (
	"flag"
	"fmt"
	"os"

	licensekg "github.com/oioio-space/maldev/license"
)

func main() {
	licPath := flag.String("license", "", "path to the licence PEM")
	pubPath := flag.String("issuer-pub", "", "path to the issuer public-key PEM")
	machine := flag.String("machine", "", "machine id evidence (in production: hostid.Composite())")
	password := flag.String("password", "", "password evidence")
	totpCode := flag.String("totp", "", "6-digit TOTP code")
	flag.Parse()

	if *licPath == "" || *pubPath == "" {
		fmt.Fprintln(os.Stderr, "usage: license-check-2 --license <p> --issuer-pub <p> [--machine ...] [--password ...] [--totp ...]")
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

	// Feed ALL three evidence options. Verify ANDs the stamped
	// bindings against the supplied evidence — missing or wrong
	// → fail.
	opts := []licensekg.VerifyOption{}
	if *machine != "" {
		opts = append(opts, licensekg.WithMachineID([]byte(*machine)))
	}
	if *password != "" {
		opts = append(opts, licensekg.WithPassword(*password))
	}
	if *totpCode != "" {
		opts = append(opts, licensekg.WithTOTPCode(*totpCode))
	}

	v, err := licensekg.Verify(licPEM, trusted, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fail] verify: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[ok] licence verified — all three bindings satisfied")
	fmt.Printf("     subject:  %s\n", v.Subject)
	fmt.Printf("     issuer:   %s (key-id %s)\n", v.Issuer, kid)
}
