//go:build windows && pe_noconsolation

package pe

import _ "embed"

//go:embed internal/noconsolation/NoConsolation.x64.o
var noConsolationX64 []byte

// loadNoConsolation returns the embedded No-Consolation .o bytes
// linked into the binary by the `pe_noconsolation` build tag.
// Produced by tools/no-consolation-build.sh, which pins
// fortra/No-Consolation @ a known commit and compiles with
// x86_64-w64-mingw32-gcc.
//
// Only the x64 variant is wired. A future pe_noconsolation_x86
// tag would add the 32-bit object for downlevel implants.
func loadNoConsolation() ([]byte, error) {
	return noConsolationX64, nil
}
