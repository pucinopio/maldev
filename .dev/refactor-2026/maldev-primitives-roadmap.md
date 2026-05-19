---
status: planning — ready for execution
opened: 2026-05-19
owner: oioio-space
scope: maldev — missing primitives for downstream consumers (WRAITH and any other C2 framework)
companion: wraith-roadmap.md (orchestration layer lives there, NOT here)
sibling_decision: confirmed — teamserver goes to a separate module
---

# maldev primitives roadmap — gap closure

> **Read first** when resuming on another machine. Every commit
> closing a row bumps `reflects_commit` in this file's front-matter
> and ticks the matching `Phase status` cell.

This file covers ONLY what lives in `github.com/oioio-space/maldev`:
implant primitives, post-exploitation, listeners (as connection
types), forwarders, file/protocol parsers, format emitters. The
teamserver orchestration (RBAC, audit log, gRPC API, sessions,
scenario engine, reporting UI) lives in the sibling module —
see `wraith-roadmap.md`.

## Scope split — maldev vs wraith

| Lives in maldev (primitive) | Lives in wraith (orchestration) |
|---|---|
| DNS / SMB / WebSocket / TCP listener types | Listener pool + failover manager + kill-switch |
| AES-GCM / ECDH wrappers | Key rotation policy + operator-facing key mgmt |
| SOCKS5 + port-forward + beacon-pipe transport | Pivot graph state + operator-facing route picker |
| PsExec / WMI / WinRM / DCOM dispatchers | Lateral-movement playbook + creds pickers |
| Kerberoast / AS-REP roast / Silver / PtT primitives | Cred vault + ticket store + replay UI |
| TCP SYN scanner / LDAP enumerator / BloodHound collector | Recon job runner + result UI |
| Office macro / HTA / HTML smuggling / LNK lure generators | Phishing engine + delivery infra |
| Sigma rule AST + MITRE Navigator JSON emitter | Report builder + timeline + operator notes |
| Chunked transfer protocol (wire format) | Task queue dispatcher + retry + priority |

If a primitive is genuinely operator-facing (HTML smuggling, lure
gen, etc.), it's still a maldev primitive — the wraith layer just
adds orchestration on top.

## Priority levels

  - **P0** — foundational primitives (listeners, pivoting,
    hollowing) — block downstream consumers.
  - **P1** — high operational value (lateral movement, Kerberos
    completion, browser creds, NTDS).
  - **P2** — pre-engagement (recon, initial-access lure gen).
  - **P3** — format emitters (sigma, navigator) — defer until a
    consumer needs them.

## Phase status

> ✅ closed · 🟡 in flight · 🟦 queued · ❌ wontfix

### P0 — Communication primitives

| ID | Status | Commit | Scope | Package target |
|---|---|---|---|---|
| M1  | 🟦 | — | DNS listener (TXT/A/CNAME server-side, slow-exfil capable) | `c2/transport` extension OR new `c2/listener/dns` |
| M2  | 🟦 | — | SMB named pipe server primitive | `c2/listener/smbpipe` — needs server-side work on top of `hirochachacha/go-smb2` (client-only) or port from Impacket |
| M3  | 🟦 | — | WebSocket listener | `c2/listener/ws` via `coder/websocket` |
| M4  | 🟦 | — | Multi-channel router (`Transport` interface chain w/ fallback + backoff) | `c2/transport.Router` |
| M28 | 🟦 | — | Chunked file transfer protocol (resume-capable) | `c2/transport/chunked` |

### P0 — Pivoting primitives

| ID | Status | Commit | Scope | Package target |
|---|---|---|---|---|
| M5  | 🟦 | — | SOCKS5 server primitive (forward + reverse) | `c2/pivot/socks5` — adopt `armon/go-socks5` (MIT). NOT ligolo-ng (GPLv3). |
| M6  | 🟦 | — | Port-forward (local + remote) | `c2/pivot/portforward` |
| M7  | 🟦 | — | Beacon-over-beacon SMB pipe transport | `c2/pivot/smbpipe` (depends on M2) |

### P0 — Inject completion

| ID | Status | Commit | Scope | Package target |
|---|---|---|---|---|
| M8  | 🟦 | — | Process hollowing (CreateProcessW SUSPEND + NtUnmapView + WriteProcessMemory + SetContext + Resume) | `inject/hollow_windows.go` (~200 LOC) |

### P1 — Post-exploitation primitives

| ID | Status | Commit | Scope | Package target |
|---|---|---|---|---|
| M9   | 🟦 | — | Browser credential harvest (Chrome SQLite + Local State + DPAPI v10 master key) | `credentials/browser/` — `modernc.org/sqlite` + extend maldev DPAPI from sekurlsa scope to non-LSASS |
| M10  | 🟦 | — | Lateral PsExec-style SMB | `lateral/psexec` — adapt `mandiant/gopacket` SMB layer (impacket-in-Go) |
| M11  | 🟦 | — | Lateral WMI exec | `lateral/wmi` — adapt `mandiant/gopacket` wmiexec (impacket-in-Go) instead of go-ole port |
| M12  | 🟦 | — | Lateral WinRM | `lateral/winrm` — `masterzen/winrm` + Kerberos auth wrapper (out of mandiant/gopacket scope) |
| M13  | 🟦 | — | Lateral DCOM (MMC20.Application / ShellWindows / ExcelDDE) | `lateral/dcom` — check `mandiant/gopacket` dcomexec first; fall back to SharpDCOM port if absent |
| M14  | 🟦 | — | NTDS dump (DIT parser + Esent + krbtgt extraction) | `credentials/ntds` — adapt `mandiant/gopacket` secretsdump (massive win vs hand-porting from Python) |

### P1 — Kerberos completion

| ID | Status | Commit | Scope | Package target |
|---|---|---|---|---|
| M15  | 🟦 | — | Kerberoast (TGS-REP + hashcat-format output) | `credentials/kerberoast` — `jcmturner/gokrb5/v8` |
| M16  | 🟦 | — | AS-REP roast (no preauth) | `credentials/asreproast` — gokrb5 |
| M17  | 🟦 | — | Silver ticket | extend `credentials/goldenticket` — same crypto core, different service principal |
| M18  | 🟦 | — | Pass-the-Ticket (LsaCallAuthenticationPackage) | `credentials/pth` extension OR new `credentials/ptt` |
| M19  | ❌ | — | DCSync (DRSGetNCChanges via DRSUAPI RPC) | wontfix this iteration — no Go ecosystem support, 2-3 weeks port from `dirkjanm/krbrelayx` |

### P2 — Recon server-side primitives

| ID | Status | Commit | Scope | Package target |
|---|---|---|---|---|
| M20  | 🟦 | — | TCP SYN scanner (stealth, rate-limited) | `recon/scan/syn` — `github.com/mandiant/gopacket` (active fork, BSD) + raw socket |
| M21  | 🟦 | — | Service fingerprinter (banner grab + version detect) | `recon/scan/fingerprint` — custom + nmap-probe corpus |
| M22  | 🟦 | — | LDAP / AD enumerator (users / groups / GPOs / SPNs / delegation) | `recon/ldap` — `go-ldap/ldap/v3` |
| M23  | 🟦 | — | BloodHound-compatible collector (JSON SharpHound format) | `recon/bloodhound` — port C# collector format |
| M24  | 🟦 | — | DNS enumerator (subdomain / record types) | `recon/dnsenum` — `miekg/dns` |
| M25  | 🟦 | — | CVE matcher against discovered services | `recon/cve` — NVD pull or Clair API |

### P2 — Initial access primitives

| ID | Status | Commit | Scope | Package target |
|---|---|---|---|---|
| M26  | 🟦 | — | Office macro generator (VBA template injection) | `initialaccess/office` |
| M27  | 🟦 | — | HTA generator | `initialaccess/hta` |
| M29  | 🟦 | — | HTML smuggling primitive (JS blob + decryptor) | `initialaccess/htmlsmuggling` |
| M30  | 🟦 | — | LNK lure (target arg + IconLocation) | extend `persistence/lnk` |

### P3 — Format emitters

| ID | Status | Commit | Scope | Package target |
|---|---|---|---|---|
| M31  | 🟦 | — | Sigma rule AST + YAML emitter | `reporting/sigma` — use `bradleyjkemp/sigma-go` for AST only, generation is custom |
| M32  | 🟦 | — | MITRE Navigator JSON emitter | `reporting/navigator` — schema is public, custom encoder |

## Library research — fork / adapt / build

Per operator directive *"cherche les packages déjà performants, fork
si nécessaire, pas de solution de lâche"*. Per-row decisions:

| Need | Library | License | Decision |
|---|---|---|---|
| DNS server | `miekg/dns` | BSD | adopt as-is |
| WebSocket server | `coder/websocket` | ISC | adopt as-is |
| SMB client | `hirochachacha/go-smb2` | BSD | adopt as-is for client |
| SMB server | nothing mature in Go | — | **port from Impacket SMBServer (~1-2 weeks)** |
| SOCKS5 | `armon/go-socks5` | MIT | adopt as-is |
| WMI / COM | `microsoft/wmi`, `go-ole/go-ole` | MIT | adopt |
| WinRM | `masterzen/winrm` | MIT | adopt + custom Kerberos wrapper |
| DCOM | nothing in Go | — | **port from SharpDCOM C# (~2 weeks)** |
| LDAP | `go-ldap/ldap/v3` | MIT | adopt |
| Kerberos | `jcmturner/gokrb5/v8` | Apache-2 | adopt (best Go krb5) |
| BloodHound | nothing in Go | — | **port C# collector JSON format** |
| TCP raw / SYN | `github.com/google/gopacket` (upstream, BSD) OR a Mandiant-maintained mirror — **NOT confused with mandiant/gopacket below, which is something else** | BSD | adopt |
| Impacket-class primitives (WMI exec, SMB client, secretsdump, etc.) | **`github.com/mandiant/gopacket`** — a Go reimplementation of Python Impacket (WMI, SMB, DCERPC, krb5). Despite the package name colliding with google/gopacket, **this is unrelated** — Mandiant's gopacket is the impacket-in-Go effort | BSD-class (verify on add) | **adopt** — covers M10 / M11 / M13 / M14 baselines |
| SQLite (Chrome creds) | `modernc.org/sqlite` | BSD-3 | adopt (CGO-free) |
| YAML (sigma) | `goccy/go-yaml` | MIT | adopt |
| Sigma AST | `bradleyjkemp/sigma-go` | MIT | adopt for AST, custom for emission |
| Sliver lateral | `bishopfox/sliver` | GPL-3 ⚠️ | **NO FORK** — license-incompatible. Reimplement on top of go-smb2 |
| ligolo-ng | `nicocha30/ligolo-ng` | AGPL-3 ⚠️ | **NO FORK** — reimplement on `armon/go-socks5` |

Reality check on the "no laziness" directive: **Sliver and ligolo
both have license problems for an MIT-licensed maldev**. We can
study their architecture freely but can't fork the code. The
effort estimates assume reimplementation.

**Major library find (operator-flagged 2026-05-19)**:
`github.com/mandiant/gopacket` is a **Go reimplementation of
Impacket** — covers WMI exec, SMB client (incl. PsExec patterns),
DCERPC, krb5 primitives, and secretsdump-style credential
extraction. The package name collides confusingly with
`github.com/google/gopacket` (packet capture, unrelated). Adopt
mandiant/gopacket for M10 / M11 / M13 / M14 baselines instead of
the Python-port effort estimates that previously sat in those
rows. Effort drops from "1-3 weeks per technique" to "1-2 weeks
of integration + maldev-side wrapping for Caller / Opener
plumbing".

## Composability principles (carried from current maldev)

Every primitive must:

  1. Accept an optional `*wsyscall.Caller` where Win32 is touched.
  2. Accept an optional `stealthopen.Opener` where files are read.
  3. Expose primitives, not god functions.
  4. Use sentinel errors at package boundaries (no PIDs / paths /
     symbol names leaked).
  5. Have a doc.go with MITRE ATT&CK ID + detection level + Example
     pointer.
  6. Have a tech md page under `docs/techniques/<area>/<name>.md`.
  7. Have a `*_example_test.go` covering Simple / Composed /
     Advanced / Complex tiers.
  8. Have a `Caller × SSN-resolver` matrix test sourced from
     `testutil.CallerResolverMatrix` for every Win32-touching path.

## Recommended execution order

Sequential constraints surface a natural priority:

  1. **M8 (process hollowing)** — small (~200 LOC), self-contained,
     immediate operator value, slots into existing `inject/` package.
     **Start here.**
  2. **M5 (SOCKS5)** — small (~150 LOC of glue around `armon/go-socks5`),
     unlocks all later pivoting work.
  3. **M4 (multi-channel router)** — pure Go composition, no new
     deps, prerequisite for M1 / M3 listener pooling.
  4. **M1 (DNS listener)** + **M3 (WebSocket listener)** — both
     straightforward, ~1 day each.
  5. **M2 (SMB server)** — heavier, port effort. Don't block on it
     for M5/M6/M4.
  6. **M9 (browser creds)** — DPAPI extension + SQLite read. ~3 days.
  7. **M15-M18 (Kerberos completion)** — gokrb5 adoption + custom
     emitters. ~1 week.
  8. **M10-M13 (lateral movement)** — heaviest single block. ~2-3
     weeks. Tackle in order: WMI (M11) → WinRM (M12) → DCOM (M13) →
     PsExec (M10). DCOM last because it's the heaviest port.
  9. **M14 (NTDS)** — heaviest single primitive. Defer until P1 is
     otherwise complete.
  10. P2 + P3 — pre-engagement and format emitters. Pull when a
      WRAITH consumer asks.

## Tracking & discipline

Same conventions as `progress.md` + the maldev v0.156.0 session:

  - Every Go-modifying commit: `/simplify` skill (3-agent review)
    BEFORE the commit lands. Reviewer findings folded in same
    commit or flagged as follow-ups.
  - Every primitive: tech md page in `docs/techniques/<area>/<name>.md`
    + doc.go in the package + `Example*` godoc + matrix test.
  - Every commit closing a row: bump this file's `reflects_commit`
    + tick `Phase status` cell.
  - SEMVER: maldev minor bump (`v0.X+1.0`) on each phase close
    where new exported API surface lands; patch bump
    (`v0.X.Y+1`) for internal refactors / doc-only changes.
  - VM E2E for every Win32-touching primitive on every commit that
    might affect it. CS-SA regression check stays mandatory.
  - Cross-machine resume: open this file FIRST. The combination
    of `reflects_commit` + the `Status` table is enough to pick
    up where the previous session stopped.

## Open questions

  - **WRAITH module name**: is `github.com/oioio-space/wraith` final
    or are we keeping a working title?
  - **NTDS dump (M14)**: do we want it in maldev or in wraith? My
    take is maldev — it's a primitive (parse DIT, extract krbtgt),
    not orchestration. But it imports a *lot* (Esent + parsers).
  - **DCSync (M19, wontfix)**: confirm wontfix or schedule for a
    dedicated 2-3 week chantier. Honest call given Go ecosystem
    state.
  - **Listener pool location**: I propose maldev only ships
    individual listener types (`c2/listener/dns`, etc.). The pool
    that picks one + fails over to another is wraith. Confirm.

## Next concrete step

Operator confirms scope split (this file + wraith-roadmap). Then
M8 (process hollowing) starts as the first commit in this roadmap.
~200 LOC, slots into `inject/`, established Caller convention,
matrix-testable. Half a day end-to-end including E2E + doc.
