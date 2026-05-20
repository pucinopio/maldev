---
milestone: M28
package: c2/transport/chunked
mitre: T1041
status: planning
opened: 2026-05-20
parent_roadmap: .dev/wraith-2026/roadmap.md
last_reviewed: 2026-05-20
reflects_commit: HEAD
---

## Goal

Resume-capable chunk-framing primitive for transferring large research artifacts over an existing `c2/transport.Transport` interface. Composes with M3 WebSocket and future HTTPS/DNS transports. Pure Go, no new external dependency. Implements **T1041: Out-of-band data transfer over an application channel**.

## Package layout

```
c2/transport/chunked/
  doc.go           — MITRE T1041, detection level, overview
  frame.go         — FrameCodec, 4-byte chunk-ID + length + CRC32
  resume.go        — ResumeToken serialization, resume state
  sender.go        — Sender: Send(ctx, io.Reader, ResumeToken) (ResumeToken, error)
  receiver.go      — Receiver: Recv(ctx, io.Writer, ResumeToken) (ResumeToken, error)
  chunked_test.go  — unit tests: codec round-trip, corruption rejection, resume state
```

## Public API

```go
// Sender writes large blobs as frames over a Transport.
type Sender struct {
  t     Transport  // underlying channel
  chunk int        // configurable chunk size, default 8KB
}

func NewSender(t Transport, chunkSize int) *Sender

func (s *Sender) Send(ctx context.Context, r io.Reader, resume ResumeToken) (ResumeToken, error)

// Receiver reads frames and reconstructs into an io.Writer.
type Receiver struct {
  t Transport
}

func NewReceiver(t Transport) *Receiver

func (r *Receiver) Recv(ctx context.Context, w io.Writer, resume ResumeToken) (ResumeToken, error)

// ResumeToken captures chunk ID + offset for reconnect.
type ResumeToken struct {
  ChunkID    uint32
  ByteOffset uint32
}

func (rt ResumeToken) Bytes() []byte
func (rt *ResumeToken) UnmarshalBinary(b []byte) error
```

## Implementation steps

| Step | Commit subject | Success criterion |
|------|----------------|-------------------|
| 1 | `feat(c2/transport/chunked): M28.a — frame codec + ResumeToken [T1041]` | FrameCodec encodes/decodes 4-byte header + payload + CRC32; ResumeToken round-trips via Bytes()/UnmarshalBinary(); unit tests pass |
| 2 | `feat(c2/transport/chunked): M28.b — Sender/Receiver with resume on reconnect [T1041]` | Send() reads io.Reader in chunks, writes frames; Recv() reconstructs into io.Writer; resume from ResumeToken skips delivered chunks; unit tests pass |
| 3 | `docs(c2/chunked): M28 tech md + tracker ✅ [T1041]` | `docs/techniques/c2/chunked-transfer.md` written with Detection section, Examples, Limitations; roadmap.md M28 ticked |

## Test plan

**Unit tests (`chunked_test.go`):**
- Frame codec: encode/decode round-trip for payloads 0–64KB.
- Corruption rejection: CRC32 mismatch causes frame discard, retry path triggered.
- ResumeToken: serialization and exact offset reconstruction.
- Context cancellation: Send/Recv return context.Canceled if ctx expires mid-frame.

**VM E2E:**
- Relay 100MB binary via M3 WebSocket (chunked Sender → WS → Receiver on test harness).
- Force disconnect at chunk 50 (simulate network interruption).
- Resume from ResumeToken, validate hash equality both sides (SHA256).
- Measure time + bandwidth (should show no unnecessary retransmit).

## Detection signatures

**Sigma rule stub (`proxy_chunked_transfer_anomalous_size.yml`):**
```yaml
title: Chunked Transfer Anomaly
detection:
  selection:
    http.content_encoding: chunked
    http.response_size: "> 100000000"  # >100MB in single session
  condition: selection
```

**Suricata EVE stream anomaly:**
- Alert on stream with `chunks_total > 1000` and consistent `bytes_per_chunk` (signature of deliberately framed transfer).

**Windows event ID:** None (application-layer; EDR visibility depends on Transport encryption state).

## Limitations

- **No encryption at this layer** — delegated to Transport (M3 WebSocket provides TLS).
- **CRC32 integrity only** — not authenticity; assume Transport handles transport-layer integrity.
- **Resume requires both-end agreement** — ResumeToken must be sent out-of-band or re-embedded in connection handshake (no automatic re-ACK).
- **Single-threaded Send/Recv** — no concurrent sender+receiver on same Transport (frame interleaving undefined).
- **Chunk size fixed at construction** — no mid-transfer negotiation (requires new Sender/Receiver pair).

## Dependencies

**External:** `io`, `crypto/crc32` (stdlib).

**Internal:** `c2/transport.Transport` interface, `testutil.CallerMethods`, `testutil.WindowsSearchableCanary` (E2E validation).

## Closure gate

- `go build ./c2/transport/chunked` and `go test ./c2/transport/chunked -v` pass.
- VM E2E test (100MB resume) passes on Windows10 + Kali.
- `docs/techniques/c2/chunked-transfer.md` includes Examples, Limitations, Sigma stub.
- `.dev/wraith-2026/roadmap.md` M28 row marked complete.
- No new external Go dependencies.
- All exported symbols documented in `doc.go` with MITRE T1041 citation.
