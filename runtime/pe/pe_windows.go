//go:build windows

package pe

import (
	"errors"
	"fmt"
	"sync"

	"github.com/oioio-space/maldev/runtime/bof"
)

// ErrLoaderMissing is returned when the No-Consolation object
// file isn't embedded in the running binary. The default build
// ships without it; rebuild with -tags=pe_noconsolation after
// running tools/no-consolation-build.sh to make the loader
// available.
var ErrLoaderMissing = errors.New(
	"runtime/pe: No-Consolation .o not embedded — rebuild with -tags=pe_noconsolation",
)

// noConsolationCache holds the prepared No-Consolation BOF.
// Lazy-initialised on first RunExecutable, reused for every
// subsequent call. The PE bytes change per invocation (caller
// supplies them via Args) but the BOF code itself is the same
// .o on every run, so prepare()'s parse + alloc + reloc work
// pays off after the first call.
//
// persistent=true lets No-Consolation keep its LIBS_LOADED cache
// + handle-info struct across operator-chained calls. That's the
// upstream BOF's documented behaviour (BeaconAddValue with a
// fixed key recovers state in subsequent invocations); we honour
// it by not zeroing .data between Executes.
//
// Lock-free fast path via sync.Once; sentinel `loadErr` captures
// initialisation failure so repeated callers see the same error
// without re-paying the Load cost.
var (
	noConsolationOnce sync.Once
	noConsolationBOF  *bof.BOF
	noConsolationErr  error
)

// loadCachedNoConsolation returns the prepared No-Consolation BOF
// for this process. First call does the embed-fetch + bof.Load +
// SetPersistent(true); subsequent calls hit the cache.
func loadCachedNoConsolation() (*bof.BOF, error) {
	noConsolationOnce.Do(func() {
		loaderBytes, err := loadNoConsolation()
		if err != nil {
			noConsolationErr = err
			return
		}
		b, err := bof.Load(loaderBytes)
		if err != nil {
			noConsolationErr = fmt.Errorf("runtime/pe: bof.Load: %w", err)
			return
		}
		b.SetPersistent(true)
		noConsolationBOF = b
	})
	return noConsolationBOF, noConsolationErr
}

// RunExecutable maps the supplied PE bytes into the current
// process and runs them via the embedded No-Consolation BOF.
// Output captured from the PE's stdout (and No-Consolation's
// own diagnostic prints) is returned as a Go string.
//
// peBytes is ignored when opt.Local is true — in that mode the
// BOF reads opt.Path from the on-disk file. Zero opt.Timeout
// substitutes 60 seconds; see Options for per-field semantics.
//
// The underlying BOF is loaded once per process and reused —
// see loadCachedNoConsolation. Repeated RunExecutable calls
// skip the parse + alloc + reloc pass entirely.
func RunExecutable(peBytes []byte, opt Options) (string, error) {
	b, err := loadCachedNoConsolation()
	if err != nil {
		return "", err
	}
	out, err := b.Execute(packArgs(peBytes, opt))
	if err != nil {
		return "", fmt.Errorf("runtime/pe: Execute: %w", err)
	}
	return string(out), nil
}
