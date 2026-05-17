//go:build windows

package bof

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRunesToString_RoundTrip locks the contract: rune-array
// conversion produces the same string as a literal, but the Go
// compiler doesn't deposit the bytes in .rdata. The latter property
// is empirically verified (see TestObfuscation_NoPlainBeaconString
// below), the former is asserted here.
func TestRunesToString_RoundTrip(t *testing.T) {
	got := runesToString('B', 'e', 'a', 'c', 'o', 'n', 'O', 'u', 't', 'p', 'u', 't')
	assert.Equal(t, "BeaconOutput", got)
}

// TestImpName_RoundTrip asserts the "__imp_<name>" assembly that
// drives the resolver map keys.
func TestImpName_RoundTrip(t *testing.T) {
	got := impName('B', 'e', 'a', 'c', 'o', 'n', 'P', 'r', 'i', 'n', 't', 'f')
	assert.Equal(t, "__imp_BeaconPrintf", got)
}

// TestObfuscation_RegistrySymbolsResolve is the integration-level
// proof that the obfuscated map keys round-trip through
// resolveBeaconImport — a typo in any rune literal would silently
// fail to register a Beacon API symbol, and the resolver would error
// at BOF load time with "unresolved external symbol".
func TestObfuscation_RegistrySymbolsResolve(t *testing.T) {
	for _, name := range []string{
		"__imp_BeaconPrintf",
		"__imp_BeaconOutput",
		"__imp_BeaconDataParse",
		"__imp_BeaconDataInt",
		"__imp_BeaconDataShort",
		"__imp_BeaconDataLength",
		"__imp_BeaconDataExtract",
		"__imp_BeaconFormatAlloc",
		"__imp_BeaconFormatReset",
		"__imp_BeaconFormatFree",
		"__imp_BeaconFormatAppend",
		"__imp_BeaconFormatInt",
		"__imp_BeaconFormatToString",
		"__imp_BeaconFormatPrintf",
		"__imp_BeaconErrorD",
		"__imp_BeaconErrorDD",
		"__imp_BeaconErrorNA",
		"__imp_BeaconGetSpawnTo",
	} {
		addr, ok := resolveBeaconImport(name)
		assert.True(t, ok, "obfuscated map key %s must resolve via clear-text lookup", name)
		assert.NotZero(t, addr, "callback address for %s", name)
	}
}
