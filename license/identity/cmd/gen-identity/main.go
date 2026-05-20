// gen-identity writes 32 random bytes to ./identity.bin if absent. Idempotent.
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	out := flag.String("out", "identity.bin", "destination path")
	force := flag.Bool("force", false, "overwrite if exists")
	flag.Parse()
	if _, err := os.Stat(*out); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "gen-identity: %s exists (use -force to overwrite)\n", *out)
		return
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		log.Fatalf("gen-identity: %v", err)
	}
	if err := os.WriteFile(*out, b[:], 0o644); err != nil {
		log.Fatalf("gen-identity: %v", err)
	}
}
