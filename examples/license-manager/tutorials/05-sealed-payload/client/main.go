// Tutorial 05 — verifier that decrypts a sealed payload after the
// licence check passes.
//
// The TUI's Recipients screen generated an X25519 keypair; the issuer
// stamped a payload sealed to that recipient into the licence. The
// licensee ships the recipient's PRIVATE key with the binary (or
// embeds it in a sealed section). At verify time, the binary calls
// license.Verify, then seal.Open on Verified.SealedPayload.
//
// Build:
//
//	go build -o /tmp/license-check-5 \
//	  ./examples/license-manager/tutorials/05-sealed-payload/client
//
// Run:
//
//	/tmp/license-check-5 \
//	    --license  /etc/myapp/license.pem \
//	    --issuer-pub /etc/myapp/issuer.pub \
//	    --recipient-priv /etc/myapp/recipient.x25519
package main

import (
	"flag"
	"fmt"
	"os"

	licensekg "github.com/oioio-space/maldev/license"
	"github.com/oioio-space/maldev/license/seal"
)

func main() {
	licPath := flag.String("license", "", "path to the licence PEM")
	pubPath := flag.String("issuer-pub", "", "path to the issuer public-key PEM")
	privPath := flag.String("recipient-priv", "", "path to the recipient X25519 private key (32 raw bytes)")
	flag.Parse()

	if *licPath == "" || *pubPath == "" || *privPath == "" {
		fmt.Fprintln(os.Stderr, "usage: license-check-5 --license <p> --issuer-pub <p> --recipient-priv <p>")
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
	priv, err := os.ReadFile(*privPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read recipient priv: %v\n", err)
		os.Exit(1)
	}
	pub, kid, err := licensekg.ParsePublicKey(pubPEM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse issuer pub: %v\n", err)
		os.Exit(1)
	}
	trusted := licensekg.Trusted{Keys: licensekg.SingleKey(kid, pub)}

	v, err := licensekg.Verify(licPEM, trusted)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fail] verify: %v\n", err)
		os.Exit(1)
	}
	if len(v.SealedPayload) == 0 {
		fmt.Fprintln(os.Stderr, "[fail] licence has no sealed payload")
		os.Exit(1)
	}
	plain, err := seal.Open(priv, v.SealedPayload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[fail] open sealed payload: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[ok] licence verified — sealed payload decrypted")
	fmt.Printf("     subject: %s\n", v.Subject)
	fmt.Printf("     payload: %s\n", string(plain))
}
