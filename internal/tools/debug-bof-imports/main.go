//go:build windows

// debug-bof-imports — diagnostic tool that opens a .o BOF and
// prints how each __imp_ symbol resolves through the same path
// runtime/bof uses (PEB-walk ROR13 first, LoadLibrary fallback).
// Intended for triaging "BOF crashes on entry" failures: the
// crash is usually a NULL pointer to call.
//
// usage: go run ./internal/tools/debug-bof-imports <bof.o>
package main

import (
	"debug/pe"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/oioio-space/maldev/hash"
	"github.com/oioio-space/maldev/win/api"
)

// bareSearch lists the DLLs probed for `__imp_<Func>` (no DLL$
// prefix) imports. Order matters — first hit wins. Mirrors
// runtime/bof.bareImportSearchOrder; future BOFs that pull from
// WININET / CRYPT32 / etc. need both lists extended in lockstep.
var bareSearch = []string{
	"KERNEL32.DLL", "ADVAPI32.dll", "USER32.dll",
	"WS2_32.dll", "OLE32.dll", "SHELL32.dll",
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: debug-bof-imports <.o file>")
		os.Exit(1)
	}
	f, err := pe.Open(os.Args[1])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer f.Close()

	seen := map[string]bool{}
	unresolved := 0
	total := 0
	for _, s := range f.Symbols {
		if !strings.HasPrefix(s.Name, "__imp_") {
			continue
		}
		if seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		total++
		fmt.Printf("%-50s ", s.Name)

		body := s.Name[6:]
		if idx := strings.IndexByte(body, '$'); idx > 0 {
			dll := strings.ToUpper(body[:idx])
			fn := body[idx+1:]
			if !strings.HasSuffix(dll, ".DLL") {
				dll += ".DLL"
			}
			if addr, e := api.ResolveByHash(hash.ROR13Module(dll), hash.ROR13(fn)); e == nil && addr != 0 {
				fmt.Printf("PEB:%s!%s -> 0x%x\n", dll, fn, addr)
				continue
			}
			h, lerr := syscall.LoadLibrary(dll)
			if lerr == nil && h != 0 {
				addr, _ := syscall.GetProcAddress(h, fn)
				if addr != 0 {
					fmt.Printf("LL:%s!%s -> 0x%x\n", dll, fn, addr)
					continue
				}
			}
			fmt.Printf("** UNRESOLVED dollar %s!%s\n", dll, fn)
			unresolved++
			continue
		}

		bare := body
		found := false
		for _, dll := range bareSearch {
			if addr, e := api.ResolveByHash(hash.ROR13Module(dll), hash.ROR13(bare)); e == nil && addr != 0 {
				fmt.Printf("BARE-PEB:%s!%s -> 0x%x\n", dll, bare, addr)
				found = true
				break
			}
		}
		if !found {
			for _, dll := range bareSearch {
				h, e := syscall.LoadLibrary(dll)
				if e != nil || h == 0 {
					continue
				}
				addr, _ := syscall.GetProcAddress(h, bare)
				if addr != 0 {
					fmt.Printf("BARE-LL:%s!%s -> 0x%x\n", dll, bare, addr)
					found = true
					break
				}
			}
		}
		if !found {
			fmt.Printf("** UNRESOLVED bare %s\n", bare)
			unresolved++
		}
	}
	fmt.Printf("\n%d / %d imports unresolved\n", unresolved, total)
	// Non-zero exit so CI / wrapper scripts can branch on unresolved
	// without parsing the textual summary. 2 keeps it distinct from
	// "argument error" (1) and "panic" (>= 130 on Windows).
	if unresolved > 0 {
		os.Exit(2)
	}
}
