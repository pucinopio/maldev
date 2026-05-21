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
