---
milestone: M2
package: c2/listener/smbpipe
mitre: T1071.002
status: planning
opened: 2026-05-20
parent_roadmap: .dev/wraith-2026/roadmap.md
last_reviewed: 2026-05-20
reflects_commit: HEAD
---

# M2: SMB named-pipe listener primitive

## Goal

Implement a server-side SMB named-pipe listener for orchestration traffic reception on study hosts. The primitive negotiates SMB2 dialect, performs NTLMv2 authentication, and exposes a registration API for handlers to bind a pipe name and receive `net.Conn`-like streams. Foundational for cross-instance relayed transport (M7). Cited technique: MITRE T1071.002 (application layer protocol — file transfer protocols; SMB named-pipe surface).

## Package layout

```
c2/listener/smbpipe/
├── listener.go       — Listener struct, dialect negotiation, tree-connect responder
├── auth.go           — NTLMv2 challenge/response, session setup
├── handler.go        — pipe registration API, conn dispatch
├── ipc.go            — IPC$ anonymous tree-connect logic
├── doc.go            — package documentation, MITRE T1071.002 reference
└── listener_test.go  — unit tests: dialect, auth round-trip, handler dispatch
```

## Public API

```go
package smbpipe

// Handler is called for each inbound pipe connection.
type Handler func(ctx context.Context, conn net.Conn) error

// Listener negotiates SMB2, performs NTLMv2 auth, and dispatches pipe connections.
type Listener struct { /* ... */ }

// New creates a listener bound to the given address.
func New(addr string) (*Listener, error)

// Register binds a pipe name to a handler. pipeName must be unique per listener.
func (l *Listener) Register(pipeName string, h Handler) error

// ListenAndServe blocks, negotiating SMB sessions and dispatching connections.
// Supports both anonymous IPC$ tree-connect and authenticated handlers.
func (l *Listener) ListenAndServe(ctx context.Context) error

// Shutdown gracefully closes all active sessions and listeners.
func (l *Listener) Shutdown(ctx context.Context) error
```

## Implementation steps

| Step | Commit subject | Success criterion |
|------|----------------|-------------------|
| 1 | `chore(go.mod): pin hirochachacha/go-smb2 + fork SHA for SMB server primitive` | go.mod references v1.x of go-smb2; fork branch documented in .dev comments if server-side support is missing |
| 2 | `feat(c2/listener/smbpipe): M2.a — server skeleton + anonymous tree-connect [T1071.002]` | TCP listener on 445, SMB2 negotiate response, anonymous IPC$ tree-connect accepted; hand-coded test dialect blob round-trip passes |
| 3 | `feat(c2/listener/smbpipe): M2.b — NTLMv2 session setup + IPC$ handler registry [T1071.002]` | Register() accepts pipe names; NTLMv2 challenge/response with fixed test vector passes; handler called on inbound connection with net.Conn contract satisfied |
| 4 | `docs(c2/smbpipe-listener): M2 tech md + tracker ✅ [T1071.002]` | `docs/c2/smbpipe-listener.md` with TL;DR, vocabulary, flow diagram, examples, decision table; roadmap.md M2 checkbox ticked |

## Test plan

**Unit tests:**
- Dialect negotiation: fixed SMB2 negotiate request/response vectors.
- NTLMv2 challenge/response: hardcoded user/pass/domain, verify server salt and response hash.
- Handler dispatch: Register(pipeName, h); simulate inbound connection; assert h called with net.Conn.
- Tree-connect responder: anonymous IPC$ accept, authenticated trees reject without valid credential.

**VM E2E (Windows10 host-only lab):**
- Start listener on 192.168.56.1:445 with two handlers registered: `\research-anon` (anonymous), `\research-auth` (admin-authenticated).
- `smbclient -N \\192.168.56.1\IPC$` — list pipes, connect to `\research-anon`, send/receive small payload.
- `smbclient -U WORKGROUP/Administrator \\192.168.56.1\IPC$` — connect to `\research-auth`, send/receive payload.
- Verify both succeed; unauthenticated access to `\research-auth` rejected with logon-failure NT status.

## Detection signatures

**Windows event 5145 (network share access):**
```yaml
title: Anonymous IPC$ Named-Pipe Access
logsource:
  product: windows
  service: security
detection:
  selection:
    EventID: 5145
    ShareName: IPC$
    RelativeTargetName|startswith: \research-
    AccessMask: 0x001201B0
  condition: selection
```

**Event 4624 (logon):**
- Logon type 3 (network logon) for NTLMv2 authentication attempts from unusual peer addresses.

## Limitations

- **SMB3 encryption deferred (M2.c stretch):** current implementation reads/writes plaintext SMB2; encryption support requires SMB3 dialect and cryptographic framing per RFC 3394.
- **NTLMv2 only:** Kerberos authentication deferred; no cross-domain trust support.
- **Single-bind port 445:** no SMB-over-QUIC or port redirection; assumes 445 available on study host.
- **go-smb2 fork burden:** `hirochachacha/go-smb2` is client-only. M2.a requires confirmation whether server-side support exists upstream; if not, fork scope: SMB2 dialect negotiator, session setup responder, tree-connect responder, IPC$ pipe dispatcher. Exact fork repo and branch flagged in go.mod.
- **Message framing not yet implemented:** pipe read/write size limits (64 KiB per SMB frame) not yet wrapped; application handlers must respect SMB buffer boundaries or add custom framing layer.

## Dependencies

- **hirochachacha/go-smb2:** client-side SMB2 dialect, crypto, message marshaling. Fork may be required for server-side support; master plan in `.dev/wraith-2026/plan.md` flags this decision point.
- **maldev internals:** `internal/log` for structured logging; `c2/transport` interface (not used in M2, but referenced in test harness for M7 integration).
- **Windows:** native NTLM hash computation via `golang.org/x/sys/windows` for auth vector generation; no external NTLM library.

## Closure gate

- [ ] go.mod updated; fork scope documented if upstream go-smb2 lacks server support.
- [ ] M2.a: TCP listener accepts negotiate, anonymous tree-connect passes unit test.
- [ ] M2.b: NTLMv2 round-trip with fixed vector passes; handler dispatch verified.
- [ ] VM E2E: `smbclient` connects to listener; both anonymous and authenticated pipes succeed.
- [ ] Tech md written and merged; roadmap M2 checkbox ticked.
- [ ] No regressions in `go test ./...`.
