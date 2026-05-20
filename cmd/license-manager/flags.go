package main

import "flag"

type cliFlags struct {
	DBPath         string
	PassphraseFile string
	NoTUI          bool
}

func parseFlags() cliFlags {
	var f cliFlags
	flag.StringVar(&f.DBPath, "db", "manager.db", "path to the SQLite store")
	flag.StringVar(&f.PassphraseFile, "passphrase-file", "", "file containing the passphrase")
	flag.BoolVar(&f.NoTUI, "no-tui", false, "boot without launching the TUI (smoke test)")
	flag.Parse()
	return f
}
