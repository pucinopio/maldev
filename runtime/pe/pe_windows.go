//go:build windows

package pe

import (
	"context"
	"errors"
	"fmt"

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

// RunExecutable maps the supplied PE bytes into the current
// process and runs them via the embedded No-Consolation BOF.
// Output captured from the PE's stdout (and No-Consolation's
// own diagnostic prints) is returned as a Go string.
//
// peBytes is ignored when opt.Local is true — in that mode the
// BOF reads opt.Path from the on-disk file. Zero opt.Timeout
// substitutes 60 seconds; see Options for per-field semantics.
func RunExecutable(peBytes []byte, opt Options) (string, error) {
	loaderBytes, err := loadNoConsolation()
	if err != nil {
		return "", err
	}
	args := packArgs(peBytes, opt)
	res, err := bof.Run(context.Background(), bof.Spec{
		Bytes:  loaderBytes,
		Args:   args,
		Method: bof.KindCOFF,
	})
	if err != nil {
		return "", fmt.Errorf("runtime/pe: BOF dispatch: %w", err)
	}
	return string(res.Output), nil
}
