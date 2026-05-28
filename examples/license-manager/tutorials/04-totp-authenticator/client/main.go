// Tutorial 04 — verifier that requires a 6-digit TOTP code.
//
// Operator workflow: the TUI's TOTP tab generated a secret and showed
// it as a QR code; the licensee scanned the QR into their authenticator
// app. At runtime, this binary prompts for the current 6-digit code and
// passes it to license.Verify via WithTOTPCode. Wrong code → exit 1.
//
// Build:
//
//	go build -o /tmp/license-check-4 \
//	  ./examples/license-manager/tutorials/04-totp-authenticator/client
//
// Run:
//
//	/tmp/license-check-4 \
//	    --license  /etc/myapp/license.pem \
//	    --issuer-pub /etc/myapp/issuer.pub \
//	    --totp 123456
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
	totp := flag.String("totp", "", "6-digit code from the authenticator app")
	flag.Parse()

	if *licPath == "" || *pubPath == "" || *totp == "" {
		fmt.Fprintln(os.Stderr, "usage: license-check-4 --license <p> --issuer-pub <p> --totp <code>")
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

	v, err := licensekg.Verify(licPEM, trusted, licensekg.WithTOTPCode(*totp))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fail] verify: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[ok] licence verified — TOTP code accepted")
	fmt.Printf("     subject: %s\n", v.Subject)
	fmt.Printf("     issuer:  %s (key-id %s)\n", v.Issuer, kid)
}
