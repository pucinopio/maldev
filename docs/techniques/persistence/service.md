---
package: github.com/oioio-space/maldev/persistence/service
---

# Windows service persistence

[← persistence index](README.md) · [docs/index](../../index.md)

## TL;DR

Install a Windows service so the implant runs as `LocalSystem`
at every boot. Highest-trust persistence available; also the
loudest.

| Trait | Value |
|---|---|
| **Trigger** | Boot (or service start trigger) |
| **Privilege** | `LocalSystem` (highest non-kernel) |
| **Auto-restart on crash?** | Yes (configurable via SCM recovery actions) |
| **Admin required to install?** | Yes — `SeCreateServicePrivilege` or admin SCM access |
| **Telemetry signature** | System Event 7045 + Security Event 4697 every install |

What this DOES achieve:

- Survives reboots, user logoffs, AV cleanup sweeps that
  target user-scope artefacts (Run keys, StartUp folders).
- Runs as `LocalSystem` — full privilege, no UAC, can
  manipulate other services.
- Implements [`persistence.Mechanism`](https://pkg.go.dev/github.com/oioio-space/maldev/persistence)
  — composes via `InstallAll` for redundant persistence.

What this does NOT achieve:

- **Loudest persistence option** — every modern EDR alerts on
  service install. Pair with [`cleanup/service.Hide`](../cleanup/service.md)
  to remove from `services.msc` enumeration after install
  (still loud during install, quieter afterwards).
- **Doesn't bypass admin requirement** — you need to be admin
  to install. For non-admin persistence, see
  [`persistence/registry`](registry.md) (HKCU) or
  [`persistence/startup-folder`](startup-folder.md).
- **EDR remediation often targets services first** — defenders
  who notice see the service name + binary path, can stop +
  delete with one PowerShell command.
- **Service description is plaintext** — choose a name +
  description that blends with legitimate Windows services
  (e.g., "Windows Update Medic" variants), but ANY new
  service in `HKLM\SYSTEM\CurrentControlSet\Services` is
  inspectable.

## Primer

Services are the canonical Windows mechanism for "long-running
process started by the OS, restarted on failure, runs as
LocalSystem unless told otherwise". Once installed, the
implant survives reboots, user logoffs, and most cleanup
sweeps that target user-scope artefacts (Run keys, StartUp
folders).

Trade-off: SCM database changes are universally audited. Mature
EDR stacks correlate Event 7045 against the binary path
(user-writable = bad), the signer (unsigned = bad), and the
service description (suspicious keywords). Pair with
[`pe/masquerade`](../pe/masquerade.md) (svchost preset),
[`pe/cert`](../pe/certificate-theft.md), and a binary path
inside `%SystemRoot%\System32\` for the lowest-noise install
operationally available.

## How It Works

```mermaid
sequenceDiagram
    participant Caller
    participant SCM as "Service Control Manager"
    participant DB as "services.exe DB"
    participant Audit as "Event log"

    Caller->>SCM: OpenSCManager(SC_MANAGER_CREATE_SERVICE)
    Caller->>SCM: CreateService(name, binPath, type, startType)
    SCM->>DB: write service entry
    DB-->>Audit: System 7045 (service installed)
    DB-->>Audit: Security 4697 (service installed)
    Caller->>SCM: StartService (optional)
    Note over SCM: services.exe spawns binPath as LocalSystem
```

The implementation uses `golang.org/x/sys/windows/svc/mgr`
under the hood — the standard svc.mgr package — to keep
the SCM interaction contract well-tested and conventional.
`Mechanism.Install` chains `Install` + (optionally)
`StartService`; `Mechanism.Uninstall` is `StopService` +
`DeleteService` with cleanup-pause semantics.

## API → godoc

[`pkg.go.dev/github.com/oioio-space/maldev/persistence/service`](https://pkg.go.dev/github.com/oioio-space/maldev/persistence/service) is the authoritative
reference for every exported symbol. This page teaches the
*concepts*; the godoc is the *specification*.

## Examples

### Simple — install + start

```go
import "github.com/oioio-space/maldev/persistence/service"

err := service.Install(&service.Config{
    Name:        "WinUpdateNotifier",
    DisplayName: "Windows Update Notification Center",
    Description: "Provides update notifications.",
    BinPath:     `C:\ProgramData\Microsoft\winupdate.exe`,
    StartType:   service.StartAuto,
})
if err != nil {
    panic(err)
}
_ = service.Start("WinUpdateNotifier")
```

### Composed — Mechanism + InstallAll redundancy

Pair with a Run-key fallback so loss of either mechanism does
not lose persistence.

```go
import (
    "github.com/oioio-space/maldev/persistence"
    "github.com/oioio-space/maldev/persistence/registry"
    "github.com/oioio-space/maldev/persistence/service"
)

mechs := []persistence.Mechanism{
    service.Service(&service.Config{
        Name:      "WinUpdate",
        BinPath:   `C:\ProgramData\Microsoft\winupdate.exe`,
        StartType: service.StartAuto,
    }),
    registry.RunKey(registry.HiveLocalMachine, registry.KeyRun,
        "WinUpdateBackup",
        `C:\ProgramData\Microsoft\winupdate.exe`),
}
errs := persistence.InstallAll(mechs)
for _, e := range errs {
    if e != nil {
        // partial install — verify which fired
    }
}
```

### Advanced — masqueraded binary in System32

The full-stealth recipe: emit a binary that masquerades as a
real svchost service host, drop it under `System32`, install
under a plausible service name.

```go
// At build time:
//   import _ "github.com/oioio-space/maldev/pe/masquerade/preset/svchost"
//   go build -o svc-update.exe ./cmd/implant

// On target (assumes admin):
import (
    "io"
    "os"

    "github.com/oioio-space/maldev/persistence/service"
)

const target = `C:\Windows\System32\svc-update.exe`

src, _ := os.Open("svc-update.exe")
dst, _ := os.Create(target)
_, _ = io.Copy(dst, src)
_ = src.Close()
_ = dst.Close()

_ = service.Install(&service.Config{
    Name:        "SvcUpdate",
    DisplayName: "Service Update Helper",
    Description: "Coordinates background service updates.",
    BinPath:     target,
    StartType:   service.StartAuto,
})
```

See [`ExampleService`](../../../persistence/service/service_example_test.go).

### Advanced — service-account override

When `LocalSystem` is too noisy, pin the service to a built-in
low-priv principal (no password needed) or to a normal user
that already holds `SeServiceLogonRight`.

```go
// 1. Built-in NT AUTHORITY\NetworkService — no password.
//    Already holds SeServiceLogonRight.
_ = service.Install(&service.Config{
    Name:        "WinUpdateNetCheck",
    DisplayName: "Windows Update Network Check",
    BinPath:     `C:\ProgramData\Microsoft\winupdate.exe`,
    StartType:   service.StartAuto,
    Account:     `NT AUTHORITY\NetworkService`,
})

// 2. Domain account. Account MUST already hold
//    SeServiceLogonRight (granted via secedit / GPO / LsaAddAccountRights).
_ = service.Install(&service.Config{
    Name:      "WinUpdateContext",
    BinPath:   `C:\ProgramData\Microsoft\winupdate.exe`,
    StartType: service.StartManual,
    Account:   `CORP\svc-winupdate`,
    Password:  os.Getenv("MALDEV_SVC_PWD"),
})
```

## OPSEC & Detection

| Artefact | Where defenders look |
|---|---|
| System Event 7045 (service installed) | Universal; high-fidelity SIEM rule when correlated against unsigned binary or user-writable path |
| Security Event 4697 (service installed) | Audit log; same population as 7045 |
| `services.msc` / `sc query` listing | Operator review; service description is the human-readable fingerprint |
| `autoruns.exe` highlight | Sysinternals Autoruns flags unsigned services in red |
| `HKLM\SYSTEM\CurrentControlSet\Services\<Name>` registry write | Sysmon Event 13 (registry value set); forensic timeline |
| Service binary path under `%TEMP%`, `%APPDATA%`, `%PROGRAMDATA%` | Defender heuristic; legitimate services live under `Program Files` or `System32` |
| Service running as `LocalSystem` with outbound HTTPS to non-MS endpoint | Behavioural EDR — outbound profile mismatch with claimed identity |
| Service with empty `DisplayName` / `Description` | Defender heuristic — legitimate services document themselves |

**D3FEND counters:**

- [D3-PSA](https://d3fend.mitre.org/technique/d3f:ProcessSpawnAnalysis/)
  — services.exe spawning unsigned binaries.
- [D3-SICA](https://d3fend.mitre.org/technique/d3f:SystemConfigurationDatabaseAnalysis/)
  — SCM database registry monitoring.

**Hardening for the operator:**

- Pair with [`pe/masquerade/preset/svchost`](../pe/masquerade.md)
  so the binary's PE metadata matches a real Microsoft service
  host.
- Pair with [`pe/cert.Copy`](../pe/certificate-theft.md) to
  graft an Authenticode blob (passes presence checks).
- Drop the binary under `%SystemRoot%\System32\` (admin
  required) — services in `Program Files` or `System32` draw
  less default scrutiny than ones under `%PROGRAMDATA%`.
- Populate `DisplayName` + `Description` with text that
  matches the cloned identity.
- Avoid this technique on hosts with strict service-creation
  audit (Microsoft LAPS-protected, enterprise SOC-monitored).

## MITRE ATT&CK

| T-ID | Name | Sub-coverage | D3FEND counter |
|---|---|---|---|
| [T1543.003](https://attack.mitre.org/techniques/T1543/003/) | Create or Modify System Process: Windows Service | full | D3-PSA, D3-SICA |

## Limitations

- **Admin required.** SCM `CreateService` needs
  `SC_MANAGER_CREATE_SERVICE` which is admin-gated.
- **Service binary contract.** The launched binary must
  implement the SCM control protocol (respond to
  `ServiceMain` start, `SERVICE_CONTROL_STOP` etc.) or it
  will be killed within ~30 s. Implants that don't implement
  the contract should run as `StartManual` + a separate
  trigger, or wrap the implant binary with the
  `golang.org/x/sys/windows/svc` runner.
- **Service-account override is one-shot.** `Config.Account` +
  `Config.Password` propagate through to `mgr.CreateService` so
  non-LocalSystem services install fine. Pair with
  [`GrantSeServiceLogonRight(account)`](https://pkg.go.dev/github.com/oioio-space/maldev/persistence/service#GrantSeServiceLogonRight)
  for user-account services where the principal doesn't already
  hold the right. Built-in `NT AUTHORITY\NetworkService` /
  `LocalService` need neither the grant nor a password.
- **Boot/System start types.** `StartBoot` / `StartSystem`
  are kernel-driver-only; userland binaries with these
  start types are rejected by SCM.
- **Pre-Vista compatibility.** Some legacy options
  (interactive desktop, etc.) are not exposed.

## See also

- [`pe/masquerade`](../pe/masquerade.md) — clone svchost
  identity for the service binary.
- [`pe/cert`](../pe/certificate-theft.md) — graft
  Authenticode signature.
- [`persistence/registry`](registry.md) — sibling lower-noise
  persistence to pair as a fallback.
- [`persistence/scheduler`](task-scheduler.md) — sibling
  lower-noise SYSTEM-scope persistence.
- [`cleanup`](../cleanup/README.md) — remove the service
  post-op.
- [Operator path](../../by-role/operator.md).
- [Detection eng path](../../by-role/detection-eng.md).
