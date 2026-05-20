---
milestone: M7
package: c2/pivot/smbpipe
mitre: T1071.002,T1090
status: planning
opened: 2026-05-20
parent_roadmap: .dev/wraith-2026/roadmap.md
last_reviewed: 2026-05-20
reflects_commit: HEAD
---

# M7: SMB named-pipe relayed transport

## Goal

Implement cross-instance relayed transport via SMB named pipes, enabling orchestration traffic to traverse from study instance B → A → outbound C2 transport on A. Compose M2 (server-side listener) with `hirochachacha/go-smb2` (client-side dialer) to satisfy the `c2/transport.Transport` interface. Foundational for orchestration of distributed study hosts over air-gapped network segments. Cited techniques: MITRE T1071.002 (application layer protocol — file transfer protocols) + T1090 (internal proxy — multiple hops).

## Package layout

```
c2/pivot/smbpipe/
├── dialer.go         — SMB named-pipe dialer, client-side tree-connect + pipe open
├── transport.go      — Transport contract implementation (Read, Write, Close)
├── relay.go          — bidirectional handler composing M2 listener + upstream Transport
├── framing.go        — SMB message frame boundary handling (read/write size limits)
├── doc.go            — package documentation, MITRE T1071.002 + T1090 references
└── pivot_test.go     — unit tests: Transport contract, relay message round-trip
```

## Public API

```go
package smbpipe

// Dialer connects to a named-pipe instance on a study host via SMB.
type Dialer struct { /* ... */ }

// New creates a dialer for the given host and credentials.
func New(host string, creds *Credentials) *Dialer

// Dial connects to a named pipe and returns a Transport.
// pipeName must match a name registered on the target listener (M2).
// Credentials are sent during NTLMv2 authentication.
func (d *Dialer) Dial(ctx context.Context, pipeName string) (c2.Transport, error)

// Transport satisfies c2.Transport: byte-stream delivery over SMB named pipe.
// Handles SMB frame boundaries transparently via internal framing.
type Transport struct { /* ... */ }

// Read reads up to len(p) bytes from the pipe.
func (t *Transport) Read(p []byte) (n int, err error)

// Write writes len(p) bytes to the pipe.
func (t *Transport) Write(p []byte) (n int, err error)

// Close closes the pipe and SMB session.
func (t *Transport) Close() error

// Relay returns a Handler for M2 listener that relays inbound pipe traffic
// to an upstream Transport. Allows study instance A to forward traffic from B.
func Relay(upstream c2.Transport) smbpipe.Handler
```

## Implementation steps

| Step | Commit subject | Success criterion |
|------|----------------|-------------------|
| 1 | `feat(c2/pivot/smbpipe): M7.a — Dialer client side + Transport contract impl [T1071.002]` | Dial() connects via hirochachacha/go-smb2; Read/Write pass c2/transport test harness contract; Close() cleans up session |
| 2 | `feat(c2/pivot/smbpipe): M7.b — Relay handler composing M2 listener with upstream [T1071.002+T1090]` | Relay(upstream) returns Handler; inbound pipe messages forwarded to upstream; upstream responses written back to inbound client |
| 3 | `docs(c2/smbpipe-pivot): M7 tech md + tracker ✅ [T1071.002+T1090]` | `docs/c2/smbpipe-pivot.md` with TL;DR, vocabulary, relay diagram, chained example, decision table; roadmap.md M7 checkbox ticked |

## Test plan

**Unit tests:**
- Transport contract: use existing `c2/transport` test harness to verify Dial/Read/Write/Close semantics.
- Relay handler: mock upstream Transport; simulate inbound connection; assert messages forwarded bidirectionally.
- Frame boundary handling: send message > 64 KiB; verify internal framing splits across SMB frames; receiver reconstructs original payload.
- NTLMv2 authentication failure: Dial with wrong password; expect auth error.

**VM E2E (Windows10 host-only lab 192.168.56.0/24):**
- Binary A (study instance A) starts M2 listener on 192.168.56.1:445 with pipe `\relayA` registered to Relay handler pointing to mock upstream transport (stub reading from a test channel).
- Binary B (study instance B) on 192.168.56.2 dials `192.168.56.1\relayA` with NTLMv2 admin creds.
- Send request from B → A → upstream; verify upstream receives request bytes intact.
- Upstream sends response back through relay; verify B receives response intact, with no data loss or corruption.
- Close B's connection; verify A's relay handler cleans up both inbound and upstream streams.
- Kali on 192.168.56.3 simulates upstream orchestrator receiving relayed traffic via custom test harness.

## Detection signatures

**Windows event 5140 (network share accessed):**
```yaml
title: Named-Pipe Access from Peer Study Instance
logsource:
  product: windows
  service: security
detection:
  selection:
    EventID: 5140
    ShareName: IPC$
    ClientAddress|startswith: 192.168.56.
    AccessMask: 0x001201B0
  condition: selection
```

**Windows event 4624 (logon — logon type 3, network):**
- Unusual logon source within study lab subnet; NTLMv2 authentication from peer IP.

**Sigma — cross-session named-pipe relay pattern:**
```yaml
title: Relay Named-Pipe Handler Active
logsource:
  product: windows
  service: security
detection:
  selection:
    EventID: 5145
    ShareName: IPC$
    RelativeTargetName: \relayA
    AccessLevel|all:
      - Read
      - Write
  filter:
    SourceAddress: 192.168.56.
  condition: selection and filter
```

## Limitations

- **Message framing overhead:** internal framing adds 8-byte header (4-byte length + 4-byte checksum) to each SMB message; receiver must parse and strip framing before passing to application. Adds ~1.6% overhead for typical C2 message sizes (5 KiB).
- **Transport contract assumes byte-stream:** SMB named pipes preserve message boundaries; framing layer normalizes to continuous stream to match c2/transport contract. Alternative: upstream Transport may need custom boundary awareness if it expects message framing.
- **Cleanup on abrupt disconnect:** if study instance B crashes mid-relay, A's handler may hold stale upstream connection briefly (cleanup timeout 30 seconds by default). No active keepalive heartbeat; Kali-side upstream must implement read timeout if relay stalls.
- **Latency stacking:** relay adds 1 hop (B → A → upstream); each SMB round-trip ~20–50 ms on lab network; latency compounds with multiple relays (B → A → C → upstream). Not suitable for low-latency interactive shells; suitable for asynchronous orchestration commands.
- **No SMB3 encryption:** inherits M2 limitation; traffic in SMB2 plaintext unless upstream Transport adds another encryption layer (TLS, etc.).
- **Single upstream per relay handler:** Relay(upstream) binds to one upstream Transport. Multiple inbound clients to same handler share single upstream connection; no connection pooling or per-client upstream selection.

## Dependencies

- **c2/listener/smbpipe (M2):** Listener, Handler contract, listener test utilities.
- **hirochachacha/go-smb2:** client-side SMB2 dialect, session setup, tree-connect, named-pipe open/read/write.
- **c2/transport:** Transport interface contract; used by Dial() return type and Relay() upstream parameter.
- **maldev internals:** `internal/log` for structured logging; `cleanup/memory` for session cleanup on Close().

## Closure gate

- [ ] M2 (smbpipe listener) merged and stable.
- [ ] Dial() connects to M2 listener; NTLMv2 authentication succeeds with valid creds.
- [ ] Transport (Read/Write/Close) passes c2/transport test harness unmodified.
- [ ] Relay() handler accepted and closes without data loss in 3-instance E2E test (B → A → upstream).
- [ ] Frame boundary handling verified: >64 KiB payload sent from B, received intact by upstream.
- [ ] Tech md written and merged; roadmap.md M7 checkbox ticked.
- [ ] No regressions in `go test ./...`.
