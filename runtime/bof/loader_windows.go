//go:build windows

package bof

import (
	"context"
	"fmt"
	"sync"
)

// Kind labels a module-format family. Today only KindCOFF is wired;
// KindGoModule and KindGOF are reserved for slices 3 and 4 of the BOF
// loader revamp (.dev/refactor-2026/bof-loader-revamp-plan.md).
type Kind int

const (
	// KindUnknown is the zero value — Run treats it as "auto-detect
	// from the magic bytes". Loaders never register under it.
	KindUnknown Kind = iota
	// KindCOFF is the Cobalt-Strike-style x64 COFF object file. Magic
	// is the IMAGE_FILE_HEADER.Machine field (0x8664, little-endian).
	KindCOFF
	// KindGoModule will cover .o produced by go tool compile for the
	// goloader path (slice 3).
	KindGoModule
	// KindGOF will cover the maldev-private custom format (slice 4).
	KindGOF
)

func (k Kind) String() string {
	switch k {
	case KindCOFF:
		return "coff"
	case KindGoModule:
		return "gomod"
	case KindGOF:
		return "gof"
	default:
		return "unknown"
	}
}

// Loader is implemented by each module-format plug-in. Plug-ins
// register themselves via registerLoader during init; Run dispatches on
// Kind. Loaders are stateless (a fresh Runnable per Load call).
type Loader interface {
	Kind() Kind
	Load(bytes []byte) (Runnable, error)
}

// Runnable is the executable shape every loader returns. The COFF
// loader's *BOF already satisfies it.
type Runnable interface {
	Execute(args []byte) ([]byte, error)
	Errors() []byte
}

// Spec drives Run. Zero-value Method triggers magic-byte detection.
// SpawnTo / UserData are applied only when the loaded Runnable exposes
// the matching setter — loaders that don't honour them simply ignore.
type Spec struct {
	Bytes    []byte
	Args     []byte
	SpawnTo  string
	UserData []byte
	Method   Kind
}

// Result is what Run produces. Output is the BOF's stdout-equivalent
// (BeaconPrintf / BeaconOutput on the COFF path); Errors is what
// BeaconErrorD / DD / NA wrote.
type Result struct {
	Output []byte
	Errors []byte
}

// Run is the format-agnostic entry point. It picks a loader (explicit
// Spec.Method, else magic-byte sniff), loads the bytes, applies the
// optional per-call knobs, then executes.
//
// ctx is reserved for slice 3+ when goloader / .gof modules will
// honour cancellation — today it's accepted and forwarded as
// documentation of the future contract.
func Run(ctx context.Context, s Spec) (*Result, error) {
	_ = ctx
	kind := s.Method
	if kind == KindUnknown {
		kind = DetectKind(s.Bytes)
	}
	if kind == KindUnknown {
		return nil, fmt.Errorf("bof.Run: cannot auto-detect format (magic bytes do not match any registered loader)")
	}
	ldr, ok := loaderFor(kind)
	if !ok {
		return nil, fmt.Errorf("bof.Run: no loader registered for kind %s", kind)
	}
	r, err := ldr.Load(s.Bytes)
	if err != nil {
		return nil, fmt.Errorf("bof.Run: load (%s): %w", kind, err)
	}
	applySpecKnobs(r, s)
	out, err := r.Execute(s.Args)
	if err != nil {
		return nil, fmt.Errorf("bof.Run: execute (%s): %w", kind, err)
	}
	return &Result{Output: out, Errors: r.Errors()}, nil
}

// DetectKind sniffs the magic bytes. The COFF check reads the
// IMAGE_FILE_HEADER.Machine field at offset 0 (little-endian 0x8664).
// Future formats add cases here; the .gof loader will look for "GOF1"
// at offset 0, the Go-module loader for the Go .o header.
func DetectKind(b []byte) Kind {
	if len(b) >= 2 && b[0] == 0x64 && b[1] == 0x86 {
		return KindCOFF
	}
	return KindUnknown
}

// applySpecKnobs forwards per-call configuration to the loaded
// Runnable when it advertises the matching setter. Loaders that don't
// implement a particular setter silently no-op — Run never fails on a
// non-applicable knob.
func applySpecKnobs(r Runnable, s Spec) {
	type spawnSetter interface{ SetSpawnTo(string) }
	if s.SpawnTo != "" {
		if ss, ok := r.(spawnSetter); ok {
			ss.SetSpawnTo(s.SpawnTo)
		}
	}
	type userDataSetter interface{ SetUserData([]byte) }
	if len(s.UserData) > 0 {
		if us, ok := r.(userDataSetter); ok {
			us.SetUserData(s.UserData)
		}
	}
}

// loader registry. Plug-ins call registerLoader in init.
var (
	loaderRegistryMu sync.RWMutex
	loaderRegistry   = map[Kind]Loader{}
)

func registerLoader(l Loader) {
	loaderRegistryMu.Lock()
	defer loaderRegistryMu.Unlock()
	loaderRegistry[l.Kind()] = l
}

func loaderFor(k Kind) (Loader, bool) {
	loaderRegistryMu.RLock()
	defer loaderRegistryMu.RUnlock()
	l, ok := loaderRegistry[k]
	return l, ok
}

// coffLoader is the slice-2 plug-in adapting the existing
// Load + (*BOF).Execute path to the Loader interface.
type coffLoader struct{}

func (coffLoader) Kind() Kind { return KindCOFF }

func (coffLoader) Load(b []byte) (Runnable, error) {
	return Load(b)
}

func init() {
	registerLoader(coffLoader{})
}
