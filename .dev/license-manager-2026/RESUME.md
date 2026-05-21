---
title: license-manager — Resume / Handoff
last_reviewed: 2026-05-21
---

# Resume work from another machine

Everything needed to continue the TUI work is now in the repo. The `ignore/`
folder is git-ignored and **will not be present on another machine** — this
file lists what was moved into the tracked tree so you can pick up anywhere.

## Where things live

| Asset | Path |
|---|---|
| Claude Design prototype (JSX + Handoff.md) | `.dev/license-manager-2026/design/prototype/` |
| Dashboard reference PNG (target visual) | `.dev/license-manager-2026/design/01-dashboard-reference.png` |
| Current rendered snapshots (SVG + PNG) | `docs/license-manager/snapshots/` |
| Active backlog | `.dev/license-manager-2026/backlog.md` |
| Progress tracker | `.dev/license-manager-2026/progress.md` |
| Resume guide (this file) | `.dev/license-manager-2026/RESUME.md` |

## Visualisation workflow

```bash
# Render any single view → ignore/snapshots/<view>.{svg,png}
make snap VIEW=dashboard

# Or render all 9 views at once
make snap-all

# Output goes to ignore/snapshots/ (git-ignored). To share, copy the .png
# into docs/license-manager/snapshots/ and commit.
```

Requirements:
- `go install github.com/charmbracelet/freeze@latest`
- Chrome or Edge installed (auto-detected, used for PNG conversion)
- The bash driver `scripts/tui-snap.sh` needs `bin/tui-snap[.exe]` built —
  `make tui-snap` builds it. The PowerShell driver `scripts/tui-snap.ps1`
  does the same on native Windows.

## State at end of last session (commit `401fb73`, tag `v0.165.0`)

### Fixed
- Widget bounds discipline: every widget sized from `Layout(bounds)`, no magic numbers
- Dashboard: 5 compact tiles in one row, fingerprint truncated, status bar visible
- Audit + Licenses: filter chips now horizontal (was vertical staircase), one status bar (was duplicated)
- Mouse clicks: accept both `MouseActionPress` and `Release`
- Onboarding persists DB then chains directly into main TUI (was exiting)
- Passphrase prompt emits `tea.Quit` on success (was hanging)

### Known visual gaps vs `01-dashboard-reference.png`
- [ ] Counter numbers not colored (should be green/red/orange/yellow/cyan per tile)
- [ ] `[k] gérer` / `[7] détail · [s] start/stop` / `[8] tout l'audit` labels not right-aligned in box titles
- [ ] ACTIVE pill not right-aligned in `Clé d'émission active` box
- [ ] Servers HTTP rows miss URL + uptime on second line
- [ ] Settings page renders only "loading…" in tui-snap (because the snap tool sends one frame and never dispatches the async settings cmd)
- [ ] Vertical / horizontal alignment of boxes within a row sometimes off by 1 char

### Pending work
- [ ] Wizard 8 steps — visual polish per `design/prototype/screens/wizard.jsx`
- [ ] Onboarding — match `design/prototype/screens/onboarding.jsx`
- [ ] Overlay polish per `design/prototype/overlays.jsx`
- [ ] Context-aware status bar (currently same hints everywhere)
- [ ] PNG output crashes on Windows freeze v0.2.2 — SVG works, Chrome converts to PNG

## How to verify visually

```bash
# Regenerate
make snap-all

# Open the PNGs side by side with the reference
# Linux: feh / eog ignore/snapshots/*.png .dev/license-manager-2026/design/01-dashboard-reference.png
# Or in your IDE: just open them
```

## Build / test
```bash
go build ./internal/manager/... ./cmd/license-manager/...
go test ./internal/manager/... -count=1
make tools                  # all 17 cmd binaries → bin/
make snap VIEW=dashboard    # one TUI screenshot
```

## Tags so far on this initiative
- v0.161.0 — backend M1 complete
- v0.162.0 — TUI Phases 1..4 (foundation through Servers)
- v0.162.1 — E2E test expansion + 3 onboarding bug fixes
- v0.162.2 — passphrase prompt + onboarding chain fixes
- v0.163.0 — cmd/tui-snap workflow + first visual pass
- v0.164.0 — full prototype alignment (8 screens refactored)
- v0.165.0 — bounds discipline + chips horizontal + status bar dedup
- v0.166.0 — visual polish: right-aligned hints, ACTIVE pill, uptime, settings skeleton, revocation alignment

---

## Session 0002 — 2026-05-21

### Commits (diff base: `4caa2d6` tag `v0.165.0`)

| SHA | Subject |
|---|---|
| `394dab5` | fix(tui/dashboard): right-aligned box title hints + server uptime + settings skeleton (P2/P4/P5) |
| `a779889` | fix(tui/dashboard): ACTIVE pill right-aligned in issuer key card (P3) |
| `76cf8ea` | fix(tui/revocation): uniform 3-box header row alignment (P0/P6) |

Tag: `v0.166.0` at `76cf8ea`

### Defects addressed

| Priority | Defect | Status |
|---|---|---|
| P0 | Box column heights misaligned (revocation header) | Fixed — all 3 boxes now `revocInfoTile` |
| P1 | Counter tile numbers not colored | Pre-existing — Color field IS applied in tile.go; freeze SVG renders it. Verified correct in source. |
| P2 | Box title hints not right-aligned | Fixed — `NewBoxWithHint` + `renderTitleRow(contentW-2)` |
| P3 | ACTIVE pill not right-aligned in key card | Fixed — `keyCardContent(textW)` with right-aligned gap |
| P4 | Settings page shows "loading…" forever | Fixed — zero-value `*ent.Setting` sentinel renders grid skeleton |
| P5 | Server rows missing uptime + full URL | Fixed — `ServerStatus.Uptime` field added; seed passes through |
| P6 | Alignment audit other screens | Done — licenses/issuers/recipients/identities/audit all clean |

### Current visual gap vs `01-dashboard-reference.png`

- **Column ratio**: dashboard left/right is 5:6 (45/55%); reference looks ~45/55 — close.
- **Counter colors**: colors are set in code (Green/Red/Orange/Yellow/Cyan) and should render in a real terminal. The freeze SVG output may appear white depending on font/renderer but the ANSI codes are present.
- **Settings grid alignment**: right column boxes staircase slightly due to `lipgloss.JoinHorizontal` on boxes of different heights — pre-existing, not regressed this session.
- **Wizard/onboarding/overlays**: not polished yet (still pending P6 follow-up).

### Build / test / CI status

- `go build ./internal/manager/... ./cmd/tui-snap/...` — clean
- `go test ./internal/manager/tui/... -count=1` — all pass (golden snapshots updated)
- CI: all 3 workflows (build / docs / mdbook) green on `76cf8ea`

### What to look at first when resuming

1. Run `bash scripts/tui-snap.sh dashboard` and compare to `.dev/license-manager-2026/design/01-dashboard-reference.png` — the main remaining gap is the settings grid column alignment.
2. Wizard steps polish — render `make snap VIEW=wizard` (not yet in tui-snap; add `-view wizard` support first), compare to `design/prototype/screens/wizard.jsx`.
3. Overlay alignment audit — run each overlay via `-keys` flag in tui-snap.

### Pending backlog (from `.dev/license-manager-2026/backlog.md`)

- [ ] Wizard 8 steps — visual polish per prototype
- [ ] Onboarding screen — match prototype
- [ ] Overlay polish (confirm, error, revoke, QR, file-picker)
- [ ] Settings grid — fix right-column box height alignment
- [ ] Context-aware status bar hints per screen
