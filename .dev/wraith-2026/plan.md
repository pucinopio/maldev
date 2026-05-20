---
project: WRAITH — C2 framework on top of maldev
status: planning
opened: 2026-05-19
owner: oioio-space
parent_session: maldev v0.156.0 ship + helper audit
audit_source: .dev/refactor-2026/helper-audit-2026-05-19.md
---

# WRAITH gap-closure master plan

> **Read this file first** when starting any WRAITH work-item, on any
> machine. The companion tracker [`progress.md`](progress.md) carries
> the live checkbox state — bump it on every commit.

## Architecture decision: team server lives inside maldev

The team-server pieces land under:

  - `c2/teamserver/` — library: auth, RBAC, audit log, gRPC service
    handlers, beacon registry, task dispatch, operator session
    manager. Single sub-tree, exports a clean façade.
  - `cmd/wraith-teamserver/` — the runnable binary composing the
    library + a config-file driven server lifecycle.

**Justification:**

  - The `c2/` area is already maldev's "comms" scope (transport,
    shell, multicat, meterpreter). Adding the server side keeps the
    family coherent.
  - Single repo = single `vendor/`, single test-infra (`vmtest`,
    Kali helpers), single CHANGELOG.
  - Go tree-shaking guarantees implants that don't `import` the
    teamserver package never embed gRPC, sqlite, or the audit
    machinery. The library surface stays composable.

**Discipline:**

  - `c2/teamserver/` MUST NOT be imported from any other maldev
    package. Enforced by a `cmd/teamserver-import-lint` go-vet
    custom pass (WR-009) or a simple grep in `scripts/pre-commit`.
  - Heavy deps (gRPC, bbolt, age) declared inside `c2/teamserver/`
    only — never at top-level go.mod choices that other packages
    depend on.

If you disagree with this placement, surface the disagreement
BEFORE work-item WR-001 starts. Reversing it later means moving
hundreds of LOC across the package boundary.

## Phase ordering (priority = top-to-bottom)

The phases are **dependency-ordered**: each row's work-items
require completed rows above. Within a phase, items can be
done in parallel by different sessions.

| Phase | Theme                                              | Items     | Time est. |
|-------|----------------------------------------------------|-----------|-----------|
| 1     | Team server foundation                             | WR-001 .. WR-015 | 3–5 days  |
| 2     | Operational core (listeners, comms, task queue)    | WR-016 .. WR-030 | 4–6 days  |
| 3     | Lateral movement + post-ex depth                   | WR-040 .. WR-055 | 5–8 days  |
| 4     | Server-side recon                                  | WR-060 .. WR-075 | 3–5 days  |
| 5     | Pivoting                                           | WR-076 .. WR-080 | 2–3 days  |
| 6     | Scenario engine + reporting                        | WR-081 .. WR-095 | 3–4 days  |
| 7     | Initial access (phishing, lures, sandbox matrix)   | WR-096 .. WR-105 | 2–3 days  |

**Total rough estimate: ~25 days of focused engineering**, assuming
maldev's existing primitives are imported as-is for everything
implant-side. Time DOES NOT include the TUI itself — that's a
separate WRAITH-side project consuming the gRPC API.

## Library research — adopt vs fork

The user constraint "**pas de solution de lache ou de feneantise**"
applies here: when a Go library is close-but-incomplete, we fork
it under `vendor-forks/` rather than monkey-patch downstream.
Forks land in their own repos and are added as Go module replace
directives in `go.mod`.

### Phase 1 — Team server

| Need                              | Library                              | Status                                    |
|-----------------------------------|--------------------------------------|-------------------------------------------|
| gRPC + protobuf                   | `google.golang.org/grpc`             | stdlib-class, adopt                       |
| mTLS                              | `crypto/tls` (stdlib)                | adopt                                     |
| WebSocket events                  | `nhooyr.io/websocket`                | active, context-aware, preferred over gorilla |
| TOTP                              | `pquerna/otp`                        | stable, ~3k stars, adopt                  |
| Yubikey U2F (PIV)                 | `go-piv/piv-go`                      | adopt for PIV-based 2FA                   |
| Yubikey FIDO U2F                  | `flynn/u2f`                          | older, evaluate; possibly write minimal   |
| RBAC                              | hand-roll on stdlib                  | no good Go RBAC lib that isn't bloated    |
| Audit log (hash-chained append)   | hand-roll on `crypto/sha256` + bbolt | no lib worth pulling                      |
| Persistent store                  | `etcd-io/bbolt`                      | adopt (battle-tested, simple K/V)         |
| Encrypted vault                   | `filippo.io/age`                     | adopt — Filippo Valsorda's reference impl |

### Phase 2 — Listeners

| Need                              | Library                              | Status                                    |
|-----------------------------------|--------------------------------------|-------------------------------------------|
| HTTPS listener                    | `net/http` + `crypto/tls`            | stdlib                                    |
| DNS listener                      | `miekg/dns`                          | gold standard, used by Sliver, adopt      |
| SMB named pipe listener           | hand-roll on `windows.CreateNamedPipe` | no Go lib; direct Win32                 |
| WebSocket listener                | `nhooyr.io/websocket`                | adopt                                     |
| ACME certificate management       | `golang.org/x/crypto/acme/autocert`  | stdlib-class, adopt                       |
| CDN fronting                      | hand-roll on `net/http.Transport`    | SNI override + Host-header rewrite        |
| Failover orchestrator             | hand-roll                            | exponential-backoff, kill-switch, custom  |

### Phase 3 — Lateral movement + Kerberos

| Need                              | Library                              | Status                                    |
|-----------------------------------|--------------------------------------|-------------------------------------------|
| SMB protocol                      | `hirochachacha/go-smb2`              | active, modern; **FORK** to add SCMR (service-control RPC) for PsExec-style |
| MSRPC infrastructure              | `oiweiwei/go-msrpc`                  | adopt; covers DCOM + DRSUAPI partial      |
| WMI                               | `go-ole/go-ole` + manual COM         | adopt go-ole, hand-roll IWbemServices.ExecMethod dispatch |
| WinRM                             | `masterzen/winrm`                    | mature; **EVAL** kerberos auth; may fork  |
| Kerberos full stack               | `jcmturner/gokrb5/v8`                | adopt; combine with our `internal/krb5`   |
| Kerberoast (TGS-REP)              | extend `jcmturner/gokrb5`            | request SPN tickets, write Hashcat-format |
| AS-REP roast (no pre-auth)        | extend `jcmturner/gokrb5`            | AS-REQ without ENC-TIMESTAMP              |
| Silver ticket                     | extend `credentials/goldenticket`    | variant of Forge with service key         |
| DCSync (DRSUAPI)                  | `oiweiwei/go-msrpc` DRSUAPI          | **FORK** if missing GetNCChanges — verify |
| NTDS.dit parser (offline)         | port `impacket/esedb`                | **FORK** target — no good Go esedb        |
| VSS shadow copy (Windows)         | `go-ole/go-ole` + Win32_ShadowCopy   | hand-roll WMI call                        |
| Browser cred harvest (Chromium)   | `modernc.org/sqlite` + DPAPI wrapper | adopt sqlite; **FORK** mimikatz-style DPAPI master-key recovery if not in `credentials/sekurlsa/dpapi.go` |

### Phase 4 — Recon

| Need                              | Library                              | Status                                    |
|-----------------------------------|--------------------------------------|-------------------------------------------|
| TCP SYN scanner                   | `gopacket/gopacket`                  | Mandiant maintains the post-Google-deprecation fork; adopt |
| Service fingerprinter             | hand-roll on `net.Dial` + banner regex | nmap-probes port (subset) is the gold standard reference |
| LDAP/AD enumerator                | `go-ldap/ldap/v3`                    | adopt; supports NTLM bind                 |
| BloodHound collector              | hand-roll                            | follow SharpHound 5.x JSON schema; no Go port exists |
| DNS enumerator                    | `miekg/dns`                          | adopt (same lib as listener)              |
| CVE matcher                       | hand-roll on NVD JSON feed           | no lib needed — fetch + match version strings |

### Phase 5 — Pivoting

| Need                              | Library                              | Status                                    |
|-----------------------------------|--------------------------------------|-------------------------------------------|
| SOCKS5 server (in-implant)        | `armon/go-socks5`                    | adopt minimal; pin version                |
| Port-forward (local/remote)       | hand-roll on stdlib                  | tiny TCP relay                            |
| Beacon-over-beacon SMB pipe       | hand-roll                            | maldev's named-pipe primitive             |

### Phase 6 — Scenario + Reporting

| Need                              | Library                              | Status                                    |
|-----------------------------------|--------------------------------------|-------------------------------------------|
| YAML parser                       | `goccy/go-yaml`                      | faster than yaml.v3, full spec support    |
| Playbook DSL                      | hand-roll                            | no Go lib; spec lives in WRAITH itself    |
| MITRE Navigator JSON              | hand-roll                            | follow attack-navigator JSON schema       |
| Sigma rule parser                 | `bradleyjkemp/sigma-go`              | adopt for parse; gen is custom            |
| Markdown report builder           | `text/template` (stdlib)             | adopt                                     |

### Phase 7 — Initial access

| Need                              | Library                              | Status                                    |
|-----------------------------------|--------------------------------------|-------------------------------------------|
| SMTP phishing                     | `net/smtp` + `html/template`         | stdlib                                    |
| Office macro generator            | hand-roll OOXML zip + VBA stream     | no good Go lib; use `archive/zip`         |
| LNK builder                       | hand-roll binary format              | parsergeneric/go-lnk reads but doesn't write well |
| HTA                               | string templates                     | trivial                                   |
| HTML smuggling                    | hand-roll                            | base64 blob + JS reconstruction           |
| Sandbox detonation matrix         | VM orchestration scripts             | not a Go lib — operator infra             |

## Work-item IDs

Each work-item gets a unique `WR-NNN` ID. The full list lives in
[`progress.md`](progress.md). Commit messages MUST cite the ID in
the subject line:

  ```
  feat(c2/teamserver): WR-001 — gRPC service skeleton + mTLS handshake
  ```

This lets git-log filtering reproduce the project history per
work-item:

  ```bash
  git log --oneline --grep="WR-001"
  ```

## Per-commit discipline

Inherited verbatim from CLAUDE.md:

  1. `go build ./...` clean before commit
  2. `/simplify` reviewed (reuse / quality / efficiency)
  3. New exported symbol ⇒ matching tech md Example + Limitation
     bullet in the same commit
  4. E2E validation: every WR item ships with a VM-runnable test
     under `cmd/vmtest` or the package's own `*_test.go` matrix
  5. Caller-aware NtXxx primitives accept optional `*wsyscall.Caller`
     (kernel32-only exempt)
  6. No `Get` prefix, no stuttering, no `ignore/` staged
  7. Update [`progress.md`](progress.md) checkbox + front-matter
     `last_reviewed` / `reflects_commit` in the same commit that
     ships the work-item

## Cross-machine resumption

To pick up WRAITH work on another machine:

  1. `git pull origin master`
  2. Read this file (`.dev/wraith-2026/plan.md`)
  3. Read [`progress.md`](progress.md) — find the first row
     marked `[in-flight]` or the first unchecked `[ ]`
  4. Read the work-item's per-item brief if it exists at
     `.dev/wraith-2026/items/WR-NNN.md` (created lazily by
     whoever starts the item)
  5. Continue.

If `progress.md`'s `reflects_commit` differs from the current
`HEAD`, run `git log <reflects_commit>..HEAD --oneline` to see
what landed between the last tracker update and now — those
commits are the WRAITH state authoritatively, the tracker is
catching up.

## Out of scope (intentionally)

  - The WRAITH TUI itself (Bubble Tea / charmbracelet). That's a
    separate project consuming this library's gRPC API.
  - macOS / Linux beacon equivalents beyond what maldev already
    surfaces.
  - Cloud / SaaS-style hosted variant of the team server.

These can be tackled after the 7-phase plan ships.
