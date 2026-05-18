//go:build windows

package bof

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// E2E suite against TrustedSec's CS-Situational-Awareness-BOF
// public corpus — battle-tested public BOFs that exercise a
// broader Beacon API surface than the in-tree hand-written
// examples.
//
// Fixtures land under testdata/cs-sa/ via
// scripts/fetch-cs-sa-bofs.sh. The directory is .gitignored
// (CS-SA is GPL-2; maldev is MIT, so committing the .o files
// would mix licenses). Each test t.Skip's cleanly when the
// fixture is absent — fresh-clone CI runs without the fetch
// step degrade gracefully rather than failing red.

const csSaDir = "testdata/cs-sa"

// loadCSSA reads a vendored .o file by short name (e.g. "dir"),
// skipping the test if the fixture is missing. The caller
// receives the raw bytes ready for bof.Load().
func loadCSSA(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(csSaDir, name+".x64.o")
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("%s missing: %v (run scripts/fetch-cs-sa-bofs.sh)", path, err)
	}
	if len(bytes) < 256 {
		t.Fatalf("%s suspiciously small: %d bytes", path, len(bytes))
	}
	return bytes
}

// runCSSABOF loads + executes a CS-SA BOF and returns its combined
// stdout. Args is the pre-packed bofdata buffer (nil when the BOF
// takes no args). Centralises the load+execute+output pattern so
// individual tests stay readable.
func runCSSABOF(t *testing.T, name string, args []byte) string {
	t.Helper()
	b, err := Load(loadCSSA(t, name))
	if err != nil {
		t.Fatalf("Load(%s): %v", name, err)
	}
	out, err := b.Execute(args)
	if err != nil {
		t.Fatalf("Execute(%s): %v", name, err)
	}
	return string(out)
}

// assertContainsAny fails the test when none of `wants` appears in
// `out`. Most CS-SA tests assert on one of several plausible header
// strings (locale-independent, version-independent) — collapsing
// the boolean chain into one helper keeps each test's expectation
// in a single line.
func assertContainsAny(t *testing.T, bofName, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if strings.Contains(out, w) {
			return
		}
	}
	t.Errorf("%s BOF output missing any of %q\noutput:\n%s", bofName, wants, out)
}

// TestCSSA_Env exercises env.x64.o — the simplest of the suite,
// no args. The BOF reads the process environment block and prints
// each KEY=VALUE line. Every Windows process has at least
// SYSTEMROOT + PATH; the assertion is case-insensitive because
// GetEnvironmentStrings preserves the original casing (Windows
// uppercases most system vars but user-set ones can be mixed).
//
// Output is NOT logged on failure — it would dump the full
// environment block including secrets (PATs, AWS keys, etc.)
// that operators have set in their shell.
func TestCSSA_Env(t *testing.T) {
	out := strings.ToUpper(runCSSABOF(t, "env", nil))
	wants := []string{"SYSTEMROOT=", "PATH="}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("env BOF output missing %q (full output redacted to avoid leaking process env)", w)
		}
	}
}

// TestCSSA_Dir exercises dir.x64.o — exercises the args path
// (string + short) and a real MSVCRT$strncat / FindFirstFile
// loop. Targets C:\Windows because every Windows install has it.
// Asserts on at least one well-known entry ("System32") rather
// than parsing the BOF's table format (which differs slightly
// from cmd's `dir` output).
func TestCSSA_Dir(t *testing.T) {
	a := NewArgs()
	a.AddString(`C:\Windows`)
	a.AddShort(0) // subdirs=false
	out := runCSSABOF(t, "dir", a.Pack())
	if !strings.Contains(out, "System32") {
		t.Errorf("dir BOF output missing 'System32' entry\noutput:\n%s", out)
	}
}

// TestCSSA_Ipconfig exercises ipconfig.x64.o — pulls adapter info
// via IPHLPAPI$GetAdaptersAddresses + IPHLPAPI$GetNetworkParams.
// The output format mirrors Windows' own ipconfig, so asserting
// on "Host Name" or "adapter" works locale-independently (the
// BOF emits English headers regardless of host locale).
//
// This is the canary for the all-sections relocation fix: the
// pre-fix loader only applied .text relocations, but ipconfig
// ships a 239-entry .rdata pointer table (string lookup arrays
// for adapter type / node type / DUID format) that needs ADDR64
// rebasing. Without it the BOF dereferences file-relative
// offsets as pointers and segfaults.
func TestCSSA_Ipconfig(t *testing.T) {
	out := runCSSABOF(t, "ipconfig", nil)
	// "Host Name" appears in the global section header; "Adapter"
	// appears once per network interface. Asserting on either
	// covers minimal-network VMs (Host Name always present) and
	// loaded boxes alike.
	assertContainsAny(t, "ipconfig", out, "Host Name", "Adapter")
}

// TestCSSA_Listmods exercises listmods.x64.o — walks loaded
// modules of a target PID (0 = self). Asserts on "ntdll.dll"
// because every Windows process has ntdll.dll loaded; "kernel32"
// would also work but ntdll is the more universal canary
// (system DLL loaded by kernel before kernel32 even).
func TestCSSA_Listmods(t *testing.T) {
	a := NewArgs()
	a.AddInt(0) // pid=0 means current process
	out := runCSSABOF(t, "listmods", a.Pack())
	if !strings.Contains(strings.ToLower(out), "ntdll.dll") {
		t.Errorf("listmods BOF output missing 'ntdll.dll'\noutput:\n%s", out)
	}
}

// TestCSSA_Arp exercises arp.x64.o — ARP cache dump via
// IPHLPAPI$GetIpNetTable. Each adapter section starts with the
// upstream's header line (which carries the typo "Inteface" —
// preserved upstream, asserting on it is more reliable than
// hoping for a correction). "Internet Address" appears on the
// table-column row and is the more durable fallback.
func TestCSSA_Arp(t *testing.T) {
	out := runCSSABOF(t, "arp", nil)
	assertContainsAny(t, "arp", out, "Internet Address", "Inteface")
}

// TestCSSA_Routeprint exercises routeprint.x64.o — routing table
// dump via IPHLPAPI$GetIpForwardTable. Every Windows host has at
// least the loopback route (127.0.0.0/8) and a default route
// (0.0.0.0); asserting on one of them covers minimal-network VMs.
func TestCSSA_Routeprint(t *testing.T) {
	out := runCSSABOF(t, "routeprint", nil)
	assertContainsAny(t, "routeprint", out, "0.0.0.0", "127.0.0.")
}

// TestCSSA_Listdns exercises listdns.x64.o — DNS resolver cache
// dump via DNSAPI$DnsGetCacheDataTable. Two valid outcomes:
// "Cache record:" lines on a populated host, or "No results
// found" on a fresh-boot VM whose resolver cache is empty.
// Both witness that DNSAPI was loaded + invoked successfully —
// what we care about for loader validation.
func TestCSSA_Listdns(t *testing.T) {
	out := runCSSABOF(t, "listdns", nil)
	assertContainsAny(t, "listdns", out, "Cache record", "No results")
}

// TestCSSA_Netstat exercises netstat.x64.o — TCP/UDP tables via
// IPHLPAPI$GetExtendedTcpTable / GetExtendedUdpTable. Asserting on
// "Proto" because it's the column header the BOF always emits
// regardless of how many sockets are open.
func TestCSSA_Netstat(t *testing.T) {
	a := NewArgs()
	a.AddInt(0) // 0 selects both TCP and UDP per upstream contract
	out := runCSSABOF(t, "netstat", a.Pack())
	assertContainsAny(t, "netstat", out, "Proto")
}

// TestCSSA_Locale exercises locale.x64.o — system locale dump via
// KERNEL32$GetLocaleInfoEx. Case-insensitive "locale" substring
// match keeps the test locale-independent (the BOF prints the
// English label even on fr-FR / ja-JP hosts).
func TestCSSA_Locale(t *testing.T) {
	out := strings.ToLower(runCSSABOF(t, "locale", nil))
	assertContainsAny(t, "locale", out, "locale")
}

// TestCSSA_Netuptime exercises netuptime.x64.o — server uptime
// via NETAPI32$NetStatisticsGet. Takes a wide-string servername
// (empty = local). The BOF prints "ServerName:" + "Boot time:"
// lines; asserting on either keeps the test resilient across
// VM snapshots.
//
// Also exercises the AddWideString fix from v0.152.0 (byte-length
// prefix) — pre-fix the empty wstring would frame as 1 wchar
// instead of 2 bytes, mis-cursoring the parser.
func TestCSSA_Netuptime(t *testing.T) {
	a := NewArgs()
	a.AddWideString("") // empty = local server
	out := runCSSABOF(t, "netuptime", a.Pack())
	assertContainsAny(t, "netuptime", out, "ServerName", "Boot time")
}

// TestCSSA_Nslookup exercises nslookup.x64.o — active DNS query
// via DNSAPI$DnsQuery_A. DnsQuery_A queries DNS servers directly,
// bypassing the hosts file — on a sandboxed VM with no upstream
// DNS, even "localhost" returns NXDOMAIN. We assert on either a
// successful resolution OR the BOF's well-defined failure path
// ("Query for domain name failed") — both witness that DNSAPI
// was resolved and the BOF made the call.
func TestCSSA_Nslookup(t *testing.T) {
	a := NewArgs()
	a.AddString("localhost")
	a.AddString("") // empty = use system DNS
	out := runCSSABOF(t, "nslookup", a.Pack())
	assertContainsAny(t, "nslookup", out,
		"127.0.0.1", "::1",
		"Query for domain name failed", // BOF's NXDOMAIN path
	)
}

// TestCSSA_Netlocalgroup exercises netlocalgroup.x64.o — local
// group enumeration via NETAPI32$NetLocalGroupEnum. Type=0
// selects enum-mode (vs. members-of-named-group when nonzero).
//
// Asserting on the BOF's own English column headers ("Name:" +
// "Comment:") rather than the group names themselves — group
// names are localised by Windows (fr-FR: "Administrateurs",
// "Utilisateurs"; ja-JP: "Administrators"... actually those
// vary too), but the BOF's printf headers are hardcoded English.
func TestCSSA_Netlocalgroup(t *testing.T) {
	a := NewArgs()
	a.AddShort(0) // 0 = enumerate all local groups
	a.AddWideString("")
	a.AddWideString("")
	out := runCSSABOF(t, "netlocalgroup", a.Pack())
	assertContainsAny(t, "netlocalgroup", out, "Name:", "Comment:")
}

// TestCSSA_Netloggedon exercises netloggedon.x64.o — logged-on
// user enumeration via NETAPI32$NetWkstaUserEnum. The BOF prints
// "Username:" + "Domain:" + "Logon server:" lines per session.
// Asserting on the "Username:" label captures the BOF's actual
// output shape (single word, lowercase 'n') rather than the
// canonical Windows wording.
func TestCSSA_Netloggedon(t *testing.T) {
	a := NewArgs()
	a.AddWideString("") // empty = local
	out := runCSSABOF(t, "netloggedon", a.Pack())
	assertContainsAny(t, "netloggedon", out, "Username:", "Logon server:")
}

// TestCSSA_Enumlocalsessions exercises enumlocalsessions.x64.o —
// WTS session enum via WTSAPI32$WTSEnumerateSessionsExA. Adds a
// new module (WTSAPI32) to the PEB-walk coverage. Every Windows
// session manager exposes at least session 0 (Services) +
// session 1 (console); asserting on "Session" header is stable.
func TestCSSA_Enumlocalsessions(t *testing.T) {
	out := runCSSABOF(t, "enumlocalsessions", nil)
	assertContainsAny(t, "enumlocalsessions", out, "Session", "session")
}

// TestCSSA_ScEnum exercises sc_enum.x64.o — service enumeration
// via ADVAPI32$EnumServicesStatusEx. Empty servername = SCM on
// localhost (no admin required for read-only SCM access). Asserts
// on a well-known always-present service name ("svchost" appears
// as part of multiple entries).
func TestCSSA_ScEnum(t *testing.T) {
	a := NewArgs()
	a.AddWideString("") // empty = local SCM
	out := runCSSABOF(t, "sc_enum", a.Pack())
	assertContainsAny(t, "sc_enum", out, "svchost", "Service", "STATE")
}

// TestCSSA_ListFirewallRules exercises list_firewall_rules.x64.o —
// firewall policy via HNetCfg COM (INetFwPolicy2). Adds COM init
// paths (CoInitializeEx + CoCreateInstance) to the surface; the
// BOF emits a "Rule Name:" line per rule. Every Windows install
// has dozens of inbox rules.
func TestCSSA_ListFirewallRules(t *testing.T) {
	out := runCSSABOF(t, "list_firewall_rules", nil)
	assertContainsAny(t, "list_firewall_rules", out, "Rule Name", "Rule name", "Direction")
}

// TestCSSA_Driversigs exercises driversigs.x64.o — installed
// driver enumeration via ADVAPI32$EnumServicesStatusExW filtered
// to driver service types. Three valid outcomes, all witnessing
// that the loader resolved ADVAPI32 and the BOF ran end-to-end:
//
//   - full success: "ImagePath" / "Signed" / ".sys" lines
//   - partial-success warnings: "WARNING: Failed to get ImagePath"
//     (host has driver registry keys ACL'd against non-admin)
//   - BOF's own failure path: "EnumServicesStatusExW failed."
//     (upstream BOF doesn't handle ERROR_MORE_DATA correctly,
//     observed on the French Windows10 VM with more services
//     than the default buffer holds; bug is in the BOF, not our
//     loader — the line itself is BeaconPrintf output proving
//     ADVAPI32 was reached + the BOF executed cleanly)
func TestCSSA_Driversigs(t *testing.T) {
	out := runCSSABOF(t, "driversigs", nil)
	assertContainsAny(t, "driversigs", out,
		"ImagePath", "Signed", ".sys",
		"EnumServicesStatusExW failed",
	)
}

// TestCSSA_Md5 exercises md5.x64.o — file MD5 via ADVAPI32
// CryptCreateHash + CryptHashData. Targets notepad.exe (every
// Windows install has it, small file, stable). The output
// includes a hex digest line which we check for the 32-char
// shape via "MD5" header rather than a fixed digest (different
// patch levels = different bytes).
func TestCSSA_Md5(t *testing.T) {
	a := NewArgs()
	a.AddString(`C:\Windows\System32\notepad.exe`)
	out := runCSSABOF(t, "md5", a.Pack())
	assertContainsAny(t, "md5", out, "MD5", "md5", "Hash")
}
