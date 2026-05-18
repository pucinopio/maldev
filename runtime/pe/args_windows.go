//go:build windows

package pe

import (
	"strings"
	"syscall"
	"time"

	"github.com/oioio-space/maldev/runtime/bof"
)

// packArgs marshals an Options struct + PE bytes into the exact
// wire format the No-Consolation BOF entry point (source/entry.c
// go()) reads from its bofdata buffer.
//
// Field order — DO NOT REORDER — must match the BeaconData* call
// sequence on the C side. Documented in
// .dev/refactor-2026/goffloader-comparison.md (1.c.9 followup)
// and verified against fortra/No-Consolation @ main.
//
// The 26-field sequence (in order):
//
//	 1. pe_wname        wide string
//	 2. pe_name         string
//	 3. pe_wpath        wide string
//	 4. pe_bytes        bytes (length-prefixed)
//	 5. pe_path         string
//	 6. local           int
//	 7. timeout         int (seconds)
//	 8. headers         int
//	 9. cmdwline        wide string
//	10. cmdline         string
//	11. method          string
//	12. use_unicode     int
//	13. nooutput        int
//	14. alloc_console   int
//	15. close_handles   int
//	16. dont_save       int
//	17. list_pes        int
//	18. unload_pe       string
//	19. username        string
//	20. loadtime        string
//	21. link_to_peb     int
//	22. dont_unload     int
//	23. load_all_deps   int
//	24. load_all_deps_but   string
//	25. load_deps       string
//	26. search_paths    string
//	27. inthread        int
func packArgs(peBytes []byte, opt Options) []byte {
	a := bof.NewArgs()

	a.AddWideString(opt.Name)
	a.AddString(opt.Name)
	a.AddWideString(opt.Path)
	a.AddBytes(peBytes)
	a.AddString(opt.Path)

	a.AddInt(boolInt(opt.Local))
	a.AddInt(timeoutSeconds(opt.Timeout))
	a.AddInt(boolInt(opt.Headers))

	cmdline := joinArgs(opt.Args)
	a.AddWideString(cmdline)
	a.AddString(cmdline)
	a.AddString(opt.Method)

	a.AddInt(boolInt(opt.UseUnicode))
	a.AddInt(boolInt(opt.NoOutput))
	a.AddInt(boolInt(opt.AllocConsole))
	a.AddInt(boolInt(opt.CloseHandles))
	a.AddInt(boolInt(opt.DontSave))
	a.AddInt(boolInt(opt.ListPEs))

	a.AddString(opt.UnloadPE)
	a.AddString(opt.Username)
	a.AddString(opt.LoadTime)

	a.AddInt(boolInt(opt.LinkToPEB))
	a.AddInt(boolInt(opt.DontUnload))
	a.AddInt(boolInt(opt.LoadAllDeps))

	a.AddString(opt.LoadAllDepsBut)
	a.AddString(opt.LoadDeps)
	a.AddString(opt.SearchPaths)

	a.AddInt(boolInt(opt.InThread))

	return a.Pack()
}

// joinArgs reconstructs a single command-line string from the
// caller's tokens using syscall.EscapeArg per token. EscapeArg
// implements the full CommandLineToArgvW round-trip — including
// the backslash-before-quote rule that a naïve quoting loop gets
// wrong on inputs like `a\"b`.
func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = syscall.EscapeArg(a)
	}
	return strings.Join(parts, " ")
}

// boolInt maps a Go bool to the 0/1 int the BOF expects. Centralised
// to keep packArgs scannable and to make the conversion obvious to
// future readers reordering the wire fields.
func boolInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// timeoutSeconds rounds a Duration down to seconds, substituting
// defaultTimeout when the caller passed the zero value. Negative
// durations clamp to zero (the BOF treats 0 as "no timeout").
func timeoutSeconds(d time.Duration) int32 {
	if d == 0 {
		d = defaultTimeout
	}
	if d < 0 {
		return 0
	}
	return int32(d / time.Second)
}
