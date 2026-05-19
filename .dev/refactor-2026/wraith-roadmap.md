---
status: planning — architectural decision confirmed (sibling module)
opened: 2026-05-19
owner: oioio-space
scope: WRAITH C2 framework — orchestration layer ONLY
companion: maldev-primitives-roadmap.md (every primitive lives in maldev, this file plans only what orchestrates them)
sibling_decision: 2026-05-19 — separate module github.com/oioio-space/wraith
maldev_pin: v0.156.0+ (track maldev minor bumps as we add primitives)
---

# WRAITH roadmap — gap closure from maldev v0.156.0

**Scope clarified 2026-05-19**: this file plans the orchestration
layer only. **All listeners, forwarders, lateral movement
primitives, Kerberos primitives, hollowing, browser creds, recon
primitives, lure generators, format emitters** live in maldev —
see `maldev-primitives-roadmap.md`. WRAITH consumes them.

> **Read this file first** when resuming WRAITH work on another
> machine or after a session break. It is the canonical view of
> what's done, what's in flight, and what comes next. Update the
> `Phase status` table + the front-matter `reflects_commit` on
> every commit that advances a phase.

## Architectural decision (pending operator sign-off)

**Recommendation: sibling module, not in-maldev.**

  - `github.com/oioio-space/maldev`  — implant + post-ex library
                                       (current; v0.156.0)
  - `github.com/oioio-space/wraith`  — C2 framework, depends on
                                       maldev as `require` (new)

Rationale: a teamserver pulls heavy server-side deps (gRPC, SQLite,
ACME, WebSocket, RBAC). An implant that imports maldev should never
inherit them transitively — Go's dead-code elimination is good but
not magic, and even an unused `database/sql` symbol surface adds
detectable artefacts to the implant binary.

**Fallback** if you insist on one module: `wraith/` subdir with
`//go:build wraith_server` on every file — strict discipline, more
friction.

Mark the decision in the front-matter before starting Phase 1.

## Tracking & discipline (cross-machine)

  - **This file is the resume-after-break canonical view.** Every
    commit that closes a Phase / Step row updates the table AND
    bumps `reflects_commit`. Hard-link this file's path into any
    new machine's `.gitignore` exclusions if needed.
  - **Per-commit checklist (CLAUDE.md):**
    1. `/simplify` skill run on every Go-modifying commit (3-agent
       review: reuse / quality / efficiency).
    2. Tech md + CHANGELOG updated in the SAME commit as the code.
    3. `go build ./...` + targeted test run before commit.
    4. VM E2E for every Caller-aware / Win32-touching path —
       matrix-tested via `testutil.CallerResolverMatrix` where the
       feature crosses a syscall surface.
    5. Pre-commit hook checklist: no `ignore/`, no credentials, no
       stuttering / Get-prefix naming.
    6. SEMVER tag at each Phase close (`wraith-v0.X.0`).
  - **Skill discipline:** brainstorming before any new feature
    design (per superpowers); TDD where the surface is testable;
    requesting-code-review before merge.
  - **Cross-machine resume protocol:** the next session opens this
    file, reads the `Phase status` table, picks the highest-pri
    🟡 in-flight row OR the topmost 🟦 queued row, continues.

## Phase status

> ✅ = closed, 🟡 = in flight, 🟦 = queued, ❌ = wontfix

| Phase | Status | Commit | Scope |
|---|---|---|---|
| 0 — Architectural decision (sibling vs in-maldev) | 🟡 | — | Operator decision; this file's front-matter records it |
| 1 — Wraith module scaffold + maldev pinning | 🟦 | — | `go mod init`, vendor maldev v0.156.0, CI bootstrap |
| 2 — Data layer (P0) | 🟦 | — | bbolt + modernc/sqlite + AGE vault + IoC tracker |
| 3 — Team server core (P0) | 🟦 | — | mTLS, gRPC, RBAC, audit log, sessions |
| 4 — Listeners (P0) | 🟦 | — | HTTPS / DNS / SMB / WebSocket / TCP + failover + ACME |
| 5 — Beacon protocol (P0) | 🟦 | — | Staged loader, multi-channel, ECDH+AES-GCM, task queue |
| 6 — Lateral movement (P1) | 🟦 | — | PsExec / WMI / WinRM / DCOM / SMB pipe |
| 7 — Kerberos completion (P1) | 🟦 | — | Kerberoast / AS-REP / Silver ticket / DCSync |
| 8 — Process hollowing (P1) | 🟦 | — | The one missing inject technique |
| 9 — Browser cred harvest (P1) | 🟦 | — | Chrome SQLite + DPAPI Local State |
| 10 — Pivoting (P1) | 🟦 | — | SOCKS5 + port-forward + beacon-over-beacon SMB |
| 11 — Recon server-side (P2) | 🟦 | — | TCP scan / fingerprint / LDAP / BloodHound / CVE |
| 12 — Initial access infra (P2) | 🟦 | — | Phishing / lure gen / sandbox detonation |
| 13 — Scenario engine (P3) | 🟦 | — | YAML DSL / step executor / MITRE Navigator export |
| 14 — Reporting (P3) | 🟦 | — | Sigma gen / markdown / timeline / IoC checklist |
| 15 — TUI (P3) | 🟦 | — | Bubbletea + grpc client |

## Library research — fork / adapt / vendor decisions

Per operator directive: *cherche les packages déjà performants, fork
si nécessaire — pas de solution de lâche*. The list below is the
result of cross-referencing the Go ecosystem for each gap.

### Data layer (Phase 2)

| Need | Library | Status |
|---|---|---|
| Pure-Go SQLite | `modernc.org/sqlite` | adopt as-is (CGO-free, prod-grade) |
| KV store | `go.etcd.io/bbolt` | adopt as-is |
| Vault encryption | `filippo.io/age` | adopt as-is (FQDN format, recipients) |
| Migrations | `pressly/goose` | adopt as-is |

### Team server core (Phase 3)

| Need | Library | Status |
|---|---|---|
| gRPC + reflection | `grpc/grpc-go` + `bufbuild/protovalidate-go` | adopt |
| mTLS plumbing | stdlib `crypto/tls` + `caddyserver/certmagic` (ACME) | adopt |
| RBAC policy | `casbin/casbin/v2` | adopt — battle-tested |
| Audit log (append-only) | `tink-crypto/tink-go` HKDF + bbolt | compose |
| TOTP | `pquerna/otp` | adopt |
| WebAuthn / yubikey | `go-webauthn/webauthn` | adopt |
| Session presence | bbolt + heartbeats | compose |
| Chat bus | `nats-io/nats.go` (embedded server) OR pure WebSocket fanout | evaluate |

### Listeners (Phase 4)

| Need | Library | Status |
|---|---|---|
| DNS server | `miekg/dns` | adopt (de facto std) |
| SMB named pipe | `hirochachacha/go-smb2` + custom server | **needs fork** — go-smb2 is client-side; need server-side named pipe listener. May port from Impacket's SMBServer. |
| WebSocket | `coder/websocket` (ex-nhooyr) | adopt (modern, simple) |
| HTTPS (Malleable C2) | maldev `c2/transport` already has uTLS + JA3 + Malleable profiles | extend |
| ACME cert mgmt | `caddyserver/certmagic` | adopt |
| CDN fronting / redirector | custom HTTP layer + `caddyserver/caddy` plugin | compose |

### Beacon protocol (Phase 5)

| Need | Library | Status |
|---|---|---|
| ECDH | `crypto/ecdh` (stdlib) | adopt |
| AES-GCM | already in maldev `crypto` | reuse |
| Multi-channel router | custom; depends on transport choices | compose |
| Task queue | `riverqueue/river` (Postgres-backed) OR `vmihailenco/taskq` (Redis) OR custom over bbolt | evaluate — probably custom over bbolt for offline ops |
| Staged loader | maldev `pe/srdi` + `pe/packer` + custom stage-0 | extend |

### Lateral movement (Phase 6)

**This is the hardest gap.** Most Go projects approximating
Impacket are partial. The honest options:

| Technique | Source | Status |
|---|---|---|
| SMB client + PsExec-style | `bishopfox/sliver`'s `lateral/sliverpsexec` (BSD, well-tested) | **FORK + adapt** — strip Sliver-specific glue, fold into wraith |
| WMI exec | `microsoft/wmi` Go package + `go-ole/go-ole` | adopt + custom dispatcher |
| WinRM | `masterzen/winrm` + kerberos wrapper | adopt |
| DCOM (MMC20.Application / ShellWindows) | port from C# `SharpDCOM` / `Impacket dcomexec.py` | **port effort** — 1-2 weeks |
| LDAP relay (NTLM relay) | `dirkjanm/krbrelayx` patterns | port effort |

**Recommended fork strategy:** maintain `wraith/lateral/` as the
target namespace. Each technique gets its own sub-package; vendor
upstream where MIT/BSD permits, port-from-C# elsewhere.

### Kerberos completion (Phase 7)

| Need | Library | Status |
|---|---|---|
| Core krb5 types | maldev `internal/krb5` (existing) | extend |
| AS-REQ / TGS-REQ wire format | `jcmturner/gokrb5/v8` | adopt (most complete Go krb5) |
| Kerberoast (TGS-REP) | gokrb5 + custom hashcat-format output | compose |
| AS-REP roast (no preauth) | gokrb5 + custom output | compose |
| DCSync (DRSUAPI) | **NOTHING in Go ecosystem.** dirkjanm's secretsdump.py is the reference. | **PORT effort** — 2-3 weeks, requires DRSUAPI RPC encoder/decoder. Skip on first pass. |
| Silver ticket | maldev `goldenticket.Forge` is generalizable | extend |
| LsaCallAuthenticationPackage (PtT) | custom Win32 via maldev `win/api` | extend |

### Process hollowing (Phase 8)

The only meaningful inject gap. `inject/` already has the building
blocks (`AllocRemoteWithCaller`, `CreateRemoteThreadWithCaller`,
`memory_helpers`, etc.); a vanilla hollowing implementation is:

  1. `CreateProcessW(target, SUSPENDED)`
  2. `NtUnmapViewOfSection` on the target's image base
  3. `VirtualAllocEx` at the unmapped base
  4. `WriteProcessMemory` for headers + each section
  5. `GetThreadContext` → patch RIP → `SetThreadContext` → `ResumeThread`

~200 LOC. **Build inside maldev** — it belongs to `inject/`.

### Browser cred harvest (Phase 9)

| Component | Library | Status |
|---|---|---|
| Chromium SQLite read | `modernc.org/sqlite` | adopt |
| Local State JSON + DPAPI v10 key | maldev `credentials/sekurlsa/dpapi.go` extended | **extend** — current DPAPI is LSASS-scoped; needs CryptUnprotectData wrapper for non-LSASS contexts |
| Firefox profile parsing | port from `gourlaysama/firefox_decrypt` (Python, BSD) | port effort |
| Edge / Brave (Chromium-based) | same as Chrome | reuse |

**Build inside maldev** — `credentials/browser/` makes architectural
sense.

### Pivoting (Phase 10)

| Need | Library | Status |
|---|---|---|
| SOCKS5 server (reverse) | `nicocha30/ligolo-ng` (GPLv3 — **forking concern**) OR `armon/go-socks5` (MIT) | **fork ligolo-ng or build from armon** |
| Port-forward | custom over SOCKS5 | compose |
| Beacon-over-beacon SMB pipe | custom; use go-smb2 client + custom server | compose |

**GPLv3 caveat:** ligolo-ng's license is incompatible with MIT.
Don't fork — implement the pivoting model fresh on top of
`armon/go-socks5`.

### Recon server-side (Phase 11)

| Need | Library | Status |
|---|---|---|
| TCP SYN scanner | `github.com/google/gopacket` (BSD) + raw socket — **not to be confused with mandiant/gopacket, which is the impacket-in-Go reimplementation, used for lateral movement (M10/M11/M13/M14) not packet capture** | adopt |
| Service fingerprinter | `projectdiscovery/nuclei` recipes OR custom banner-grab | evaluate |
| LDAP enumerator | `go-ldap/ldap/v3` | adopt |
| BloodHound collector | port `BloodHoundAD/SharpHound`'s collector format from C# | **port effort** — JSON spec is public |
| DNS enumerator | `miekg/dns` + custom wordlist | compose |
| CVE matcher | `quay/clair` API OR custom NVD pull | evaluate |

### Initial access (Phase 12)

| Need | Library | Status |
|---|---|---|
| SMTP + templates | stdlib `net/smtp` + `html/template` | compose |
| HTML smuggling | custom JS payload + maldev `crypto` | compose |
| Office macro generation | custom (VBA template + maldev `crypto`) | compose |
| LNK / HTA | maldev `persistence/lnk` already has the LNK side | extend |
| Sandbox detonation | spin up `microsoft/avmlbox` OR custom VirusTotal API client | evaluate |

### Scenario engine (Phase 13)

| Need | Library | Status |
|---|---|---|
| YAML parser | `goccy/go-yaml` (faster + better errors than gopkg.in/yaml.v3) | adopt |
| Step executor + DSL | custom | build |
| MITRE Navigator JSON | schema-driven custom | build |
| Approval gate | custom (websocket → operator) | build |

### Reporting (Phase 14)

| Need | Library | Status |
|---|---|---|
| Sigma rule output | `bradleyjkemp/sigma-go` for parsing — generation is custom | compose |
| Markdown reports | `goldmark` rendering | compose |
| Timeline | custom over IoC store | build |

### TUI (Phase 15)

| Need | Library |
|---|---|
| Framework | `charmbracelet/bubbletea` |
| Widgets | `charmbracelet/bubbles` |
| Styling | `charmbracelet/lipgloss` |
| File browser | `charmbracelet/bubbles/filepicker` |

## Composability principles (carried from maldev)

Every wraith package must:

  1. **Accept an optional `*wsyscall.Caller`** where Win32 is
     touched — same convention as maldev.
  2. **Accept an optional `stealthopen.Opener`** where files are
     read — same convention as maldev.
  3. **Expose primitives, not god functions.** A `wraith/lateral/
     wmi.Exec(target, command, caller)` beats a `wraith.RunWMI(...)`
     mega-API.
  4. **Sentinel errors at package boundaries**, no PIDs / paths /
     symbol names leaked in error strings (per maldev
     `feedback_code_quality.md`).
  5. **One technique = one file = one tech md page** — same shape
     as `docs/techniques/<area>/<name>.md`.
  6. **Caller-resolver matrix test** for every Win32-touching
     primitive — sourced from `testutil.CallerResolverMatrix`.

## Priority recommendation

  - **P0 — must land first** (foundational, blocks the rest):
    Phases 1 → 5. Data layer + team server core + listeners +
    beacon protocol. Until you can spawn a beacon and talk to it,
    nothing else matters.
  - **P1 — high operational value**: Phases 6 → 10. Lateral +
    Kerberos + hollowing + browser + pivoting. The "what does the
    operator actually do once they have a beacon" surface.
  - **P2 — pre-engagement infra**: Phases 11 → 12. Recon + initial
    access. Worth less than P1 because most ops walk in with a
    target list and a phish lure already prepared.
  - **P3 — orchestration & polish**: Phases 13 → 15. Scenario
    engine + reporting + TUI. The "user-facing C2" layer once the
    primitives are solid.

## Next concrete step

Operator confirms (or overrides) the architectural decision:

  - [ ] sibling module `github.com/oioio-space/wraith`
  - [ ] subdir `wraith/` in maldev with build tag

Then Phase 1 (scaffold) starts. Estimated effort to reach a usable
P0 stack: 3-4 weeks of focused work given the library research above.

## Open follow-ups

  - SEMVER strategy across the two modules: tag wraith independently
    of maldev. Each maldev minor bump warrants a wraith `go get -u`
    cycle.
  - Naming bikeshed: `WRAITH` is a strong codename. The Go module
    path probably stays lowercase per convention.
  - GPLv3 contamination: confirm we don't fork any GPL'd code into
    wraith. `ligolo-ng` and a few BloodHound bits are flagged above.
