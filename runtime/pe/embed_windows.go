//go:build windows && !pe_noconsolation

package pe

// loadNoConsolation is the default-build variant: ships no
// loader bytes. Callers get ErrLoaderMissing.
//
// Build with `-tags=pe_noconsolation` and supply the .o under
// runtime/pe/internal/noconsolation/ (produced by
// tools/no-consolation-build.sh) to override this symbol with
// the embedding variant in embed_pe_noconsolation_windows.go.
//
// Why split: the No-Consolation .o is ~63 KB of mingw-compiled
// COFF and isn't published as a release artefact upstream. The
// build tag keeps the default `go build` reproducible (no
// missing-asset failures) while letting operators opt into the
// embedded loader once they've built it themselves.
func loadNoConsolation() ([]byte, error) {
	return nil, ErrLoaderMissing
}
