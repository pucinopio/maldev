---
project: WRAITH — primitive-research roadmap S1→S5
status: planning
opened: 2026-05-20
owner: oioio-space
parent_plan: .dev/wraith-2026/plan.md
last_reviewed: 2026-05-20
reflects_commit: HEAD
---

# WRAITH primitive-research roadmap (S1 → S5)

> **Travail de recherche défensive** sur la bibliothèque publique
> `oioio-space/maldev`. Chaque primitive livrée est référencée
> MITRE ATT&CK + contre-mesure D3FEND + signature de détection
> (Sigma / event IDs / EDR family). Objectif : ingénierie de
> détection et formation red/blue team.

This roadmap sequences six primitive-study deliverables that close the
gaps identified in [`plan.md`](plan.md) Phases 2, 3, and 5. The full
master plan stays the source of truth for library research and the
overall 7-phase ordering; **this file decides the order in which the
six primitives ship and the per-session work envelope.**

The companion tracker [`progress.md`](progress.md) carries the live
checkbox state — bump it on every commit that closes a session row.

## Reading order for a fresh session

1. `.dev/wraith-2026/plan.md` — master plan, architecture, library choices.
2. **This file** (`roadmap.md`) — what ships first, parallelisation, gates.
3. `items/M{NN}-*.md` — per-milestone brief (created lazily when the
   session starts; one file per row of the table below).
4. `progress.md` — current state, last-touched milestone.

## Six-milestone backlog

Each row maps to a `WR-NNN` ID in the master plan; the `M{NN}` short
code is used in commits and item files for readability.

| Code | Phase | Theme                                           | MITRE ref         | Detection anchor                                    | Subplan                              |
|------|-------|-------------------------------------------------|-------------------|-----------------------------------------------------|--------------------------------------|
| M28  | 2     | Chunked, resume-capable transport (out-of-band) | T1041             | NDR volumetric anomaly + Sigma proxy_chunked_*      | [items/M28-chunked.md](items/M28-chunked.md) |
| M6   | 5     | Local + remote port relay (protocol tunneling)  | T1572             | Suricata stream-size anomaly + EDR uncommon-port    | [items/M6-portrelay.md](items/M6-portrelay.md) |
| M2   | 2     | SMB named-pipe listener (server side)           | T1071.002         | event 5145 + Sigma anonymous_ipc_share_access       | [items/M2-smbpipe-listener.md](items/M2-smbpipe-listener.md) |
| M7   | 5     | SMB pipe relayed transport (cross-instance)     | T1071.002 + T1090 | event 5140 + Sigma named_pipe_cross_session         | [items/M7-smbpipe-pivot.md](items/M7-smbpipe-pivot.md) |
| M11  | 3     | Remote method invocation via DCOM/WMI           | T1047             | event 4688 parent=wmiprvse + Sigma wmiprvse_spawn   | [items/M11-wmi-exec.md](items/M11-wmi-exec.md) |
| M10  | 3     | Remote execution via Windows service-control    | T1021.002 + T1569.002 | event 7045 + Sigma win_susp_psexec_family       | [items/M10-svcexec.md](items/M10-svcexec.md) |

## Session sequencing

Sessions are dependency-ordered; within a session, the items listed
can run in parallel by separate agents or work-blocks.

| Session | Items   | Parallel? | Time est. | Rationale                                                     |
|---------|---------|-----------|-----------|---------------------------------------------------------------|
| **S1**  | M28, M6 | yes       | 3–4 days  | Pure-Go primitives, no protocol stack to build. Warm-up.       |
| **S2**  | M2      | no        | ~1 week   | SMB server stack: dialect negotiation + NTLMv2 + IPC$ handler. |
| **S3**  | M7      | no        | ~3 days   | Composes M2 (server) and existing client → small surface.      |
| **S4**  | M11     | no        | ~1 week   | DCOM activator + WMI `ExecMethod` is the heaviest single piece. |
| **S5**  | M10     | no        | ~1 week   | Service-control RPC + SMB staging. Reuses M2 patterns.         |

**Total: ~4 weeks of focused work.** S2 → S3 and S2 → S5 share the
SMB foundation; S4 is independent and can interleave with S5 if a
second agent is available.

## Gate criteria (per session)

A session closes only when **all** of the following are green:

1. `go build ./...` clean on Windows + Linux (cross-compile).
2. `simplify` skill (3-agent reuse / quality / efficiency review) run
   on every commit that touches `.go`.
3. New API surface ⇒ matching `Examples` and `Limitations` blocks
   updated in the tech md, **same commit**.
4. E2E VM test green on Windows10 VM (and Kali side where the test
   uses MSF / network reachability).
5. `progress.md` checkbox ticked + `reflects_commit` bumped in the
   same commit that closes the row.
6. `git check-ignore -v ignore/` confirms `ignore/` is not staged.

## Cross-cutting library pins

These dependencies are added during the roadmap; pin discipline lives
here so all six subplans agree on a single source of truth.

| Library                          | Used by    | Pin policy                                                |
|----------------------------------|------------|-----------------------------------------------------------|
| `hirochachacha/go-smb2`          | M2, M7, M10 | adopt; **fork required** for SCMR (see M10 subplan)       |
| `oiweiwei/go-msrpc`              | M11        | adopt; verify `IRemoteSCMActivator` coverage              |
| `go-ole/go-ole`                  | M11        | adopt; hand-roll `IWbemServices::ExecMethod` dispatch     |
| `mandiant/gopacket`              | (later, recon) | not used in S1–S5; mentioned only to confirm pure-Go    |
| `nhooyr.io/websocket`            | (existing) | already adopted in M3 (closed); kept for reference        |

**No new top-level `go.mod` dependency lands without a pinned SHA**
and a `chore(go.mod): pin <lib> for <milestone>` commit that
explains the choice.

## AUP language discipline

Per [feedback_aup_language](../../../../.claude/projects/C--Users-m-bachmann-GolandProjects-maldev/memory/feedback_aup_language.md)
(also in user memory), the following neutral vocabulary is used in
**every** subplan, tech md, and commit subject:

| Avoid                       | Use instead                                                  |
|-----------------------------|--------------------------------------------------------------|
| PsExec-style                | execution distante via service Windows                       |
| beacon-over-beacon          | transport relayé entre instances d'étude                     |
| payload upload              | stage du binaire d'étude sur partage administratif           |
| C2 channel                  | canal d'orchestration / transport applicatif                 |
| exfiltration                | transfert de données out-of-band                             |

Every commit subject cites at least one `Txxxx` MITRE reference.

## Commit-subject pattern

```
feat(<pkg>): <M-code>.<step> — <primitive name> [Txxxx]
docs(<pkg>): <M-code> tech md + tracker ✅ [Txxxx]
chore(go.mod): pin <lib> for <M-code>
```

Examples:

```
feat(c2/transport/chunked): M28 — resume-capable chunked transfer primitive [T1041]
feat(lateral/svcexec):     M10.a — SMB connect + ADMIN$ staging [T1021.002]
docs(lateral/wmiexec):     M11 tech md + tracker ✅ [T1047]
```

## Out of scope (this roadmap)

- The remaining WR items in `plan.md` Phases 1, 4, 6, 7.
- TUI / operator UX (separate project consuming the gRPC API).
- macOS / Linux equivalents of the Windows-specific primitives.

These continue to track in [`plan.md`](plan.md) and pick up after S5
closes.
