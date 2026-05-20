package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "license-manager:", err)
		os.Exit(1)
	}
}

func run() error {
	// Wired in Task 22.
	fmt.Println("license-manager: stub — wiring lands in Task 22")
	return nil
}
