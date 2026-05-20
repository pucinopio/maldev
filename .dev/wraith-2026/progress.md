---
project: WRAITH primitive-research — live tracker
status: in-progress
opened: 2026-05-20
last_reviewed: 2026-05-20
reflects_commit: 93a98c5
---

# WRAITH primitive-research progress

Bump `last_reviewed` + `reflects_commit` on every commit that touches
a row below. The session ordering is defined in
[`roadmap.md`](roadmap.md); detailed briefs live in `items/`.

## Sessions

| Session | Items   | State    | Closed on | Notes                                |
|---------|---------|----------|-----------|--------------------------------------|
| S1      | M28, M6 | `[ ]`    |           |                                      |
| S2      | M2      | `[ ]`    |           | depends on S1 patterns (none hard)   |
| S3      | M7      | `[ ]`    |           | depends on S2 (server side)          |
| S4      | M11     | `[ ]`    |           | independent; can interleave with S5  |
| S5      | M10     | `[ ]`    |           | depends on S2 (SMB stack reuse)      |

## Milestones

| Code | Package                       | State    | Closed on | Reflects commit |
|------|-------------------------------|----------|-----------|-----------------|
| M28  | `c2/transport/chunked`        | `[x]`    | 2026-05-20 |                |
| M6   | `c2/pivot/portrelay`          | `[x]`    | 2026-05-20 | 93a98c5        |
| M2   | `c2/listener/smbpipe`         | `[ ]`    |           |                 |
| M7   | `c2/pivot/smbpipe`            | `[ ]`    |           |                 |
| M11  | `lateral/wmiexec`             | `[ ]`    |           |                 |
| M10  | `lateral/svcexec`             | `[ ]`    |           |                 |

## Library pins (status)

| Library                  | Pinned SHA | Added in commit |
|--------------------------|------------|-----------------|
| `hirochachacha/go-smb2`  |            |                 |
| `oiweiwei/go-msrpc`      |            |                 |
| `go-ole/go-ole`          |            |                 |

## State legend

- `[ ]` — not started
- `[~]` — in-flight (one or more commits, gate not yet green)
- `[x]` — closed (all gate criteria green per `roadmap.md`)
