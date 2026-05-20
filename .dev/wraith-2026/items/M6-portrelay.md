---
milestone: M6
package: c2/pivot/portrelay
mitre: T1572
status: planning
opened: 2026-05-20
parent_roadmap: .dev/wraith-2026/roadmap.md
last_reviewed: 2026-05-20
reflects_commit: HEAD
---

## Goal

Local and remote port relay primitive for connecting research lab segments and studying protocol tunneling. Pure stdlib implementation (`net` only). Both local and remote relays honor context cancellation and connection lifecycle cleanly. Implements **T1572: Protocol tunneling** — research framing focuses on TCP relay mechanics for lab orchestration.

## Package layout

```
c2/pivot/portrelay/
  doc.go           — MITRE T1572, detection level, overview
  local.go         — Local(ctx, listenAddr, dialAddr) error, TCP accept + forward
  remote.go        — Remote(ctx, conn Transport, dialAddr) error, framed requests from Transport
  relay.go         — internal relayConn, bidirectional copy + error propagation
  portrelay_test.go — unit tests: net.Pipe, context cancellation, connection lifecycle
```

## Public API

```go
// Local accepts connections on listenAddr and relays to dialAddr.
func Local(ctx context.Context, listenAddr, dialAddr string) error

// Remote accepts framed relay requests over an existing Transport connection
// and dials dialAddr locally, copying both directions.
func Remote(ctx context.Context, conn Transport, dialAddr string) error

// Transport represents a bidirectional frame channel (e.g., WebSocket, HTTP/2).
type Transport interface {
  Send(ctx context.Context, frame []byte) error
  Recv(ctx context.Context) ([]byte, error)
  Close() error
}

// internal relayConn handles bidirectional copy between two conns.
type relayConn struct {
  from, to net.Conn
  done     chan error
}

func (rc *relayConn) relay(ctx context.Context) error
```

## Implementation steps

| Step | Commit subject | Success criterion |
|------|----------------|-------------------|
| 1 | `feat(c2/pivot/portrelay): M6.a — local forwarder + connection lifecycle [T1572]` | Local() starts listener, accepts TCP, dials target, bidirectional copy; context cancellation closes listener + active connections; unit tests with net.Pipe pass |
| 2 | `feat(c2/pivot/portrelay): M6.b — remote forwarder with Transport framing [T1572]` | Remote() accepts framed dial requests over Transport, establishes outbound connection, relays payload; tracks conn per frame ID; unit tests pass |
| 3 | `docs(c2/portrelay): M6 tech md + tracker ✅ [T1572]` | `docs/techniques/c2/protocol-tunneling.md` written with Detection section, Examples, Limitations; roadmap.md M6 ticked |

## Test plan

**Unit tests (`portrelay_test.go`):**
- Local forwarder: listener on localhost, dial target via net.Pipe, bidirectional copy round-trip.
- Remote framing: Transport mock send/recv, frame codec for dial request + payload relay.
- Context cancellation: both Local() and Remote() return context.Canceled when ctx expires.
- Connection closure: active relays close cleanly on either side hangup; no goroutine leaks.
- Error propagation: read/write errors surface via return value, not panic.

**VM E2E:**
- Host: `portrelay.Local(":4445", "192.168.56.101:445")` (Windows10 SMB target).
- Kali client: `smbclient -L //127.0.0.1:4445` via relay.
- Validate share listing matches direct-connect to target; measure latency < 10ms overhead.

## Detection signatures

**Suricata EVE stream anomaly:**
```yaml
- Alert on `stream` with unusual source/dest port pair (e.g., local ephemeral → remote 445/139).
- Signature: `uncommon_outbound_port.yaml` — detect outbound SMB/LDAP/RPC not from server IPs.
```

**Windows MDE category:** `Uncommon outbound port` (EDR heuristic; T1572 commonly flagged here).

**Sigma rule stub (`network_connection_uncommon_port.yml`):**
```yaml
title: Uncommon Outbound Port
detection:
  selection:
    destination.port: [445, 139, 389, 3389]
    source.ip: "192.168.56.0/24"  # lab range, not production server
    destination.ip: "!192.168.56.0/24"  # outbound to external
  condition: selection
```

**Windows event ID:** 5156 (outbound connection audit) when AppLocker / netsh rules log unusual port pairs.

## Limitations

- **No transport encryption** — raw TCP copy; assume existing Transport (WebSocket) or network provides TLS.
- **Single-target dial** — no SOCKS dynamic routing (separate M-SOCKS milestone); both Local and Remote dial a fixed remote address.
- **No rate limiting** — full-speed relay; DoS risk if attacker sends large streams. Document user responsibility for flow control.
- **Bidirectional copy only** — no connection pooling, multiplexing, or session reuse across frames; new relay per connection.
- **No keepalive** — tcp.keepalive not set; idle connections may timeout per OS defaults.

## Dependencies

**External:** `net`, `context`, `io` (stdlib).

**Internal:** `c2/transport.Transport` interface (for Remote), `testutil.SpawnSacrificial` (E2E SMB target spawn).

## Closure gate

- `go build ./c2/pivot/portrelay` and `go test ./c2/pivot/portrelay -v` pass.
- VM E2E test (Kali smbclient via local + remote relay) passes on Windows10 target.
- `docs/techniques/c2/protocol-tunneling.md` includes Examples, Limitations, Sigma stub.
- `.dev/wraith-2026/roadmap.md` M6 row marked complete.
- No new external Go dependencies.
- All exported symbols documented in `doc.go` with MITRE T1572 citation + detection references.
