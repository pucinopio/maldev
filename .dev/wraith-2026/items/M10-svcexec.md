---
milestone: M10
package: lateral/svcexec
mitre: T1021.002,T1569.002
status: planning
opened: 2026-05-20
parent_roadmap: .dev/wraith-2026/roadmap.md
last_reviewed: 2026-05-20
reflects_commit: HEAD
---

# M10 — Service-Based Remote Execution (T1021.002 + T1569.002)

## Goal

Primitive d'étude pour execution distante via service Windows: connexion SMB + stage du binaire d'étude sur `ADMIN$`, puis séquence d'API service-control-manager standard (`OpenSCManagerW` → `CreateServiceW` → `StartServiceW` → IO via pipe `IPC$` → `DeleteServiceW`). Reuses M2 SMB stack (`hirochachacha/go-smb2` fork) for the SMB side; uses SCMR over DCERPC for service operations. Research framing: MITRE T1021.002 (SMB / admin shares) and T1569.002 (system services — service execution).

## Package layout

```
lateral/svcexec/
  doc.go                    # T1021.002 + T1569.002 technique documentation
  executor.go               # Executor struct, Exec() entry point, KeepService flag
  staging.go                # SMB connection, binary stage to ADMIN$
  scmr_ops.go               # DCERPC SCMR sequence: OpenSCManager, CreateService, StartService, DeleteService
  pipe_io.go                # IPC$ named-pipe communication for output capture
  result.go                 # Result struct with Stdout, ServiceName, timestamps
  executor_windows.go       # Windows implementation
  executor_linux.go         # Stub (no Windows services on Linux)
  cleanup.go                # Guaranteed defer cleanup logic, error recovery
  _test.go                  # Unit tests: SCMR marshaling, ServiceName collision avoidance, cleanup-on-panic
```

## Public API

```go
package svcexec

// Executor manages remote service-based execution.
type Executor struct {
	KeepService bool // if true, do not delete service after execution (default false — always cleanup)
}

// Result holds execution output and metadata.
type Result struct {
	Stdout           string
	ServiceName      string
	ServiceCreatedAt time.Time
	ServiceDeletedAt time.Time
	ExitCode         uint32
}

// Exec stages a binary via SMB ADMIN$ share, creates + starts a service, captures output, then deletes the service.
func (e *Executor) Exec(ctx context.Context, host, username, password, binaryPath string, args []string) (*Result, error)
```

## Implementation steps

| Step | Commit Subject | Success Criterion |
|------|--------|-------------------|
| 1 | `feat(lateral/svcexec): M10.a — SMB connect + ADMIN$ staging (reuses M2 stack) [T1021.002]` | SMB connection established, binary successfully staged to `\\host\ADMIN$\staged.exe`, unit test verifies stage path, M2 SMB stack integration clean |
| 2 | `feat(lateral/svcexec): M10.b — SCMR sequence: CreateService → StartService → IO over IPC$ [T1569.002]` | SCMR NDR marshaling unit-tested, CreateServiceW accepts random ServiceName, StartServiceW dispatches, output captured via IPC$ named pipe, Result struct populated with ExitCode |
| 3 | `feat(lateral/svcexec): M10.c — guaranteed cleanup (defer DeleteService) + error recovery [T1569.002]` | Cleanup test confirms DeleteServiceW called even on panic; partial-failure scenarios (e.g., IPC$ read timeout) do not skip DeleteService; Result returned with best-effort data |
| 4 | `docs(lateral/svcexec): M10 tech md + tracker ✅ [T1021.002+T1569.002]` | Tech md Examples block shows typical Exec() usage + KeepService=true edge case, Limitations block documents cleanup discipline + event audit, .dev/wraith-2026/roadmap.md milestone checked ✅ |

## Test plan

### Unit
- SCMR parameter marshaling (NDR encoding of CreateServiceW arguments).
- ServiceName randomization and uniqueness assertion (no collision test helper).
- Cleanup-on-panic: defer DeleteService() executes even if Exec() panics mid-execution.
- IPC$ named-pipe read with timeout edge cases (partial reads, timeout, closed pipe).

### VM E2E
- **Setup:** Windows10 workgroup, local admin credentials, Sysmon/Defender exclusions per test docs.
- **Main scenario:** `Exec(ctx, "192.168.56.101", "Admin", "password", ".\\notepad-canary.exe", nil)`.
  - Verify event 7045 (service installed) in Event Viewer with auto-generated ServiceName.
  - Verify event 7036 (service stopped) after Exec() returns.
  - Verify event 4697 (security audit) if audit policy enabled.
  - Verify `sc query <ServiceName>` returns "not found" after Exec() completes.
  - Verify `Result.ServiceName` matches event 7045 entry.
- **Cleanup discipline test:** Inject failure in IPC$ pipe read, verify `sc query <ServiceName>` still returns "not found" (cleanup happened).
- Run from: host (native Win32) or Kali SSH (DCERPC over TCP).

## Detection signatures

| Detector | Signature | Event ID / Sigma |
|----------|-----------|------------------|
| Windows Event Log | Service installed | Event 7045 |
| Windows Event Log | Service stopped | Event 7036 |
| Windows Event Log | Security audit service install (if enabled) | Event 4697 |
| Sigma | PsExec family execution detection | `win_susp_psexec_family.yml` |
| MDE | PsExec family telemetry | MDE behavioral classification |
| Sysmon | Process creation under `svchost.exe` (parent chain analysis) | Event 1 (process create) |

## Limitations

- **Cleanup discipline mandatory:** `defer DeleteService()` must execute even on partial failure (IPC$ read timeout, ExitCode unrecoverable). Cleanup failure is logged but does NOT cause Exec() to return error; best-effort guarantee only. Document as critical safety principle.
- **SMB stage requires write to ADMIN$:** Caller must have local admin rights or SeBackup privilege. Document as prerequisite.
- **ServiceName visible in audit trail:** Randomized ServiceName (e.g., `svc_4a7b9e2c`) to minimize collisions, but entropy budget is modest (~32 bits). Document the visibility trade-off; detection via event 7045 is unavoidable.
- **SCMR over TCP only:** Named-pipe SCMR fallback (reachable via SMB IPC$ without DCERPC) deferred to M12. M10 uses DCERPC RPC only.
- **IPC$ output capture requires elevation or explicit share access:** Named pipe `\\host\IPC$` may require admin token on older Windows versions. Document as known limitation.

## Dependencies

- `lateral/smb2` (M2) for SMB connection and ADMIN$ staging.
- `oiweiwei/go-msrpc` for DCERPC SCMR RPC call marshaling.
- `internal/log` for structured logging.
- Windows Service Control Manager RPC interface (DCERPC) — standard OS API.

## Closure gate

- [ ] Exec() signature matches API sketch; Result struct complete with timestamps.
- [ ] SMB stage to ADMIN$ works; M2 stack integration verified.
- [ ] SCMR sequence (Open → Create → Start → IO → Delete) fully implemented and unit-tested.
- [ ] Cleanup-on-panic test confirms defer DeleteService() always fires.
- [ ] VM E2E: event 7045 (service install) + 7036 (service stop) observed on Windows10; service absent in `sc query` after Exec().
- [ ] Event audit event 4697 documented (conditional on audit policy).
- [ ] Tech md written with Examples + Limitations blocks; MITRE T1021.002 + T1569.002 + detection anchors documented.
- [ ] All tests (unit + VM E2E) passing; coverage >80% for `executor.go` + `scmr_ops.go` + `cleanup.go`.
- [ ] Roadmap.md M10 gate ✅ marked; no regressions in M1–M9 packages.
