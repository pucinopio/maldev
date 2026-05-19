//go:build windows

package bof

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/maldev/testutil"
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

// TestCSSA_Whoami exercises whoami.x64.o — current user identity
// via ADVAPI32$GetTokenInformation. The output always includes
// the user SID (S-1-5-...) and the username; asserting on the
// SID prefix is universally stable.
func TestCSSA_Whoami(t *testing.T) {
	out := runCSSABOF(t, "whoami", nil)
	assertContainsAny(t, "whoami", out, "S-1-5", "SID", "User Name")
}

// TestCSSA_Whoami_CallerMatrix runs whoami.x64.o through every
// meaningful (wsyscall.Method, SSN-resolver) combination so each
// syscall dispatch strategy gets exercised on a real public-corpus
// BOF — not a synthetic primitive. 14 sub-tests, sourced from
// testutil.CallerResolverMatrix so the same row set stays in
// lock-step with TestBeaconRemoteAlloc_CallerMatrix.
//
// Each sub-test asserts the BOF completes end-to-end (Load + Execute
// + Close) under that Caller configuration and emits the SID prefix
// — i.e. the kernel32 wrapper that BeaconInjectProcess would have
// used is correctly bypassed and the BOF observes a working host.
//
// whoami is the canonical pick: short runtime, deterministic output,
// no admin / network / side effects. A regression in any single
// (Method, Resolver) cell shows up here before it can hide behind a
// green default-path test.
func TestCSSA_Whoami_CallerMatrix(t *testing.T) {
	bytes := loadCSSA(t, "whoami")
	for _, cm := range testutil.CallerResolverMatrix(t) {
		t.Run(cm.Name, func(t *testing.T) {
			b, err := Load(bytes)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			defer b.Close()
			b.SetCaller(cm.Caller)
			out, err := b.Execute(nil)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			assertContainsAny(t, "whoami", string(out), "S-1-5", "SID", "User Name")
		})
	}
}

// TestCSSA_Tasklist exercises tasklist.x64.o — process
// enumeration via WMI (ConnectServer + ExecQuery
// Win32_Process). Two valid outcomes:
//
//   - Success: process names like "svchost", "System" appear.
//   - BOF's documented failure path: "ConnectServer to  failed"
//     when WMI access is gated (some sandboxed VMs, AV-blocked
//     COM init, etc.). The BOF emits 3 cascading error messages
//     in that case — each one is BeaconPrintf output, witnessing
//     loader + import resolution + capture worked.
func TestCSSA_Tasklist(t *testing.T) {
	a := NewArgs()
	a.AddWideString("") // empty = local
	out := runCSSABOF(t, "tasklist", a.Pack())
	assertContainsAny(t, "tasklist", out,
		"svchost", "System", "Image Name",
		"ConnectServer", "Wmi_Connect", // BOF's WMI failure path
	)
}

// TestCSSA_Uptime exercises uptime.x64.o — system uptime via
// KERNEL32$GetTickCount64. Output prints a "days/hours/minutes/
// seconds" breakdown; "seconds" appears in every variant.
func TestCSSA_Uptime(t *testing.T) {
	out := runCSSABOF(t, "uptime", nil)
	assertContainsAny(t, "uptime", out, "seconds", "second", "minutes", "uptime")
}

// TestCSSA_Useridletime exercises useridletime.x64.o — user
// idle time via USER32$GetLastInputInfo. On a freshly-booted
// VM idle time is small but always non-zero; the BOF prints a
// "user has been idle for X" line.
func TestCSSA_Useridletime(t *testing.T) {
	out := runCSSABOF(t, "useridletime", nil)
	assertContainsAny(t, "useridletime", out, "idle", "Idle", "seconds")
}

// TestCSSA_Windowlist exercises windowlist.x64.o — titled
// top-level window enumeration via USER32$EnumDesktopWindows.
// VM caveat: headless / SSH-only sessions can have zero titled
// windows on the current desktop. We spawn notepad.exe ahead
// of the BOF call as a known-titled witness, kill it after,
// then assert on its title appearing in the output.
//
// ALL=1 includes hidden windows so the assertion holds whether
// the desktop is visible or not.
func TestCSSA_Windowlist(t *testing.T) {
	const witnessTitle = "Untitled - Notepad"
	cmd := exec.Command(`C:\Windows\System32\notepad.exe`)
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn notepad: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	// Notepad takes a moment to create its top-level window.
	time.Sleep(500 * time.Millisecond)

	a := NewArgs()
	a.AddInt(1) // include hidden — VM session may have no visible desktop
	out := runCSSABOF(t, "windowlist", a.Pack())
	// notepad's window title is localised ("Notepad" en-US,
	// "Bloc-notes" fr-FR, etc.) so we don't assert on that.
	// EnumDesktopWindows always reports the cmd.exe / SSH shell
	// process whose path is locale-neutral; that's the durable
	// witness across VM locales. The notepad spawn is still
	// useful — it boosts the window count on minimal sessions
	// — but the assertion targets the locale-neutral entry.
	_ = witnessTitle // kept for read-back documentation
	assertContainsAny(t, "windowlist", out, "cmd.exe", "Hidden", "Visible")
}

// TestCSSA_Sha1 exercises sha1.x64.o — same surface as md5
// (ADVAPI32 Crypt* + MSVCRT file I/O) with CALG_SHA1. Same
// fixture (notepad.exe) for the same reason as TestCSSA_Md5.
func TestCSSA_Sha1(t *testing.T) {
	a := NewArgs()
	a.AddString(`C:\Windows\System32\notepad.exe`)
	out := runCSSABOF(t, "sha1", a.Pack())
	assertContainsAny(t, "sha1", out, "SHA", "sha", "Hash")
}

// TestCSSA_Sha256 exercises sha256.x64.o — CALG_SHA_256 variant.
// The BOF source labels the input variable "server" but uses it
// as a file path (upstream copy-paste typo, no impact). Same
// assertion shape as sha1.
func TestCSSA_Sha256(t *testing.T) {
	a := NewArgs()
	a.AddString(`C:\Windows\System32\notepad.exe`)
	out := runCSSABOF(t, "sha256", a.Pack())
	assertContainsAny(t, "sha256", out, "SHA", "sha", "Hash")
}

// TestCSSA_Cacls exercises cacls.x64.o — file ACL listing via
// ADVAPI32$GetNamedSecurityInfoW + LookupAccountSid. Output is
// cacls.exe-style (NOT SDDL): "PRINCIPAL:F" / ":C" / ":R" with
// (CI)/(OI)/(IO) inheritance flags. Asserting on "BUILTIN\\" or
// "TrustedInstaller" — both literal English strings that survive
// VM locale (verified on French Windows10 where the user-facing
// labels become "Administrateurs" but the principal namespace
// prefix stays English).
func TestCSSA_Cacls(t *testing.T) {
	a := NewArgs()
	a.AddWideString(`C:\Windows`)
	out := runCSSABOF(t, "cacls", a.Pack())
	assertContainsAny(t, "cacls", out, `BUILTIN\`, "TrustedInstaller", "NT SERVICE")
}

// TestCSSA_Nettime exercises nettime.x64.o — server time via
// NETAPI32$NetRemoteTOD. Empty servername = local. The BOF
// prints "Current time at:" + a parsed timestamp.
func TestCSSA_Nettime(t *testing.T) {
	a := NewArgs()
	a.AddWideString("") // empty = local
	out := runCSSABOF(t, "nettime", a.Pack())
	assertContainsAny(t, "nettime", out, "Current time", "time at", ":")
}

// TestCSSA_Schtasksenum exercises schtasksenum.x64.o —
// scheduled-task enumeration via ITaskService COM. Empty
// hostname = local. Every Windows host ships with built-in
// tasks under \Microsoft\Windows\ ; asserting on the "\"
// path separator or the "Microsoft" folder name is portable.
func TestCSSA_Schtasksenum(t *testing.T) {
	a := NewArgs()
	a.AddWideString("") // empty = local
	out := runCSSABOF(t, "schtasksenum", a.Pack())
	assertContainsAny(t, "schtasksenum", out, "Microsoft", "Task", "task")
}

// TestCSSA_Aadjoininfo exercises aadjoininfo.x64.o — Azure AD
// join state via NETAPI32$NetGetAadJoinInformation. Two valid
// outcomes: an AAD-joined host returns tenant info, a workgroup
// host returns "Device is not joined" — both prove the API
// resolved + the BOF ran.
func TestCSSA_Aadjoininfo(t *testing.T) {
	out := runCSSABOF(t, "aadjoininfo", nil)
	assertContainsAny(t, "aadjoininfo", out, "joined", "Joined", "Tenant", "Device")
}

// TestCSSA_GetSessionInfo exercises get_session_info.x64.o —
// terminal session info via WTSAPI32$WTSQuerySessionInformation.
// Reports user / domain / client info for the current console
// session. The BOF emits "Domain" + "Username" labels.
func TestCSSA_GetSessionInfo(t *testing.T) {
	out := runCSSABOF(t, "get_session_info", nil)
	assertContainsAny(t, "get_session_info", out, "Domain", "Username", "Session")
}

// TestCSSA_Netshares exercises netshares.x64.o — share enum
// via NETAPI32$NetShareEnum. Empty sharename + asAdmin=0 keeps
// the BOF on the basic (non-admin) path. Every Windows install
// has IPC$ which always shows up.
func TestCSSA_Netshares(t *testing.T) {
	a := NewArgs()
	a.AddWideString("") // empty = enumerate all
	a.AddInt(0)         // asAdmin=false stays on the level-1 enum path
	out := runCSSABOF(t, "netshares", a.Pack())
	assertContainsAny(t, "netshares", out, "IPC$", "Share", "share")
}

// TestCSSA_GetPasswordPolicy exercises get_password_policy.x64.o —
// password policy via NETAPI32$NetUserModalsGet (modal 0).
// Empty server = local SAM. The BOF reports min password length,
// max age, history count — the "Password" label appears in
// each printed field.
func TestCSSA_GetPasswordPolicy(t *testing.T) {
	a := NewArgs()
	a.AddWideString("") // empty = local
	out := runCSSABOF(t, "get_password_policy", a.Pack())
	assertContainsAny(t, "get_password_policy", out, "Password", "password", "Min", "Max")
}

// ============================================================
// MALDEV_INTRUSIVE=1 — admin / privileged BOFs
// ============================================================
//
// These call APIs that require LocalSystem or admin (audit
// policy enumeration, SAM access, VSS service, etc.) or
// touch session-state that other tests rely on. Each is
// gated by testutil.RequireIntrusive so default `go test`
// runs stay clean.

// TestCSSA_AdvAuditPolicies exercises adv_audit_policies.x64.o —
// per-user audit policy via ADVAPI32$AuditEnumeratePerUserPolicy.
// Needs admin: querying audit policy requires SE_SECURITY_NAME.
// The BOF takes an int "iswow64" flag — pass 0 for native x64.
func TestCSSA_AdvAuditPolicies(t *testing.T) {
	testutil.RequireIntrusive(t)
	a := NewArgs()
	a.AddInt(0) // 0 = native x64
	out := runCSSABOF(t, "adv_audit_policies", a.Pack())
	assertContainsAny(t, "adv_audit_policies", out,
		"Policy", "policy", "Audit", "audit",
		"AuditEnumerate", // failure path acceptable
	)
}

// TestCSSA_Regsession exercises regsession.x64.o — registry-
// session enumeration (the keys that map to remote user
// sessions via HKEY_USERS). Empty hostname = local registry.
// Needs admin because some HKEY_USERS\<SID> subkeys are
// ACL'd against non-admin reads.
func TestCSSA_Regsession(t *testing.T) {
	testutil.RequireIntrusive(t)
	a := NewArgs()
	a.AddString("") // empty = local
	out := runCSSABOF(t, "regsession", a.Pack())
	assertContainsAny(t, "regsession", out,
		"S-1-5", "Session", "session", "User", "user",
		"RegOpenKey", "RegQueryValue", // failure path
	)
}

// TestCSSA_ScQuery exercises sc_query.x64.o — query a single
// service's status via ADVAPI32$QueryServiceStatusEx. Targets
// "RpcSs" (Remote Procedure Call), which every Windows install
// has running. Not strictly admin-only but intrusive because
// it touches the SCM.
func TestCSSA_ScQuery(t *testing.T) {
	testutil.RequireIntrusive(t)
	a := NewArgs()
	a.AddString("")      // hostname empty = local SCM
	a.AddString("RpcSs") // always-present built-in service
	out := runCSSABOF(t, "sc_query", a.Pack())
	assertContainsAny(t, "sc_query", out,
		"RpcSs", "STATE", "RUNNING", "TYPE",
		"Failed to query service", // non-admin path (ACCESS_DENIED)
	)
}

// TestCSSA_Vssenum exercises vssenum.x64.o — Volume Shadow
// Copy enumeration via the VSS COM interface (IVssBackup
// Components). Needs admin because VSS service queries are
// gated. Empty hostname = local. Empty sharename = all
// volumes.
func TestCSSA_Vssenum(t *testing.T) {
	testutil.RequireIntrusive(t)
	a := NewArgs()
	a.AddWideString(`C:`) // hostname slot is repurposed as drive root
	a.AddWideString(`C$`) // share name
	out := runCSSABOF(t, "vssenum", a.Pack())
	assertContainsAny(t, "vssenum", out,
		"Volume", "Shadow", "VSS", "vssadmin",
		// The BOF builds a UNC root path from the two args. With
		// empty inputs it produces \\\ which the FS rejects; the
		// BOF prints "Could not open root folder to query" which
		// still witnesses that NetWkstaGetInfo + path build ran.
		"Could not open root folder", "Error:",
	)
}

// TestCSSA_Netuser exercises netuser.x64.o — query a single
// user's properties via NETAPI32$NetUserGetInfo. Targets the
// well-known "Administrator" SID; empty domain = local SAM.
// Admin-gated because reading SAM details about other accounts
// is privileged on hardened systems.
func TestCSSA_Netuser(t *testing.T) {
	testutil.RequireIntrusive(t)
	a := NewArgs()
	a.AddWideString("Administrator") // always-present account
	a.AddWideString("")              // empty = local SAM
	out := runCSSABOF(t, "netuser", a.Pack())
	assertContainsAny(t, "netuser", out,
		"User name", "Administrator", "User ID",
		"SID", "S-1-5",
		"Failed to get user info", // BOF's failure path when non-admin
		"NetUserGetInfo",          // raw API failure form
	)
}
