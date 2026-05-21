# TUI Snapshot Workflow

This page explains how to capture PNG screenshots of every license-manager TUI
screen and how to use those screenshots to iterate on the visual design.

## Prerequisites

Install [charmbracelet/freeze](https://github.com/charmbracelet/freeze):

```bash
go install github.com/charmbracelet/freeze@latest
```

Freeze requires a working font on the system.
[JetBrains Mono](https://www.jetbrains.com/legalnotice/fonts/) is the default;
install it or change `--font.family` in `scripts/tui-snap.sh`.

## Capturing snapshots

### Single view

```bash
make tui-snap VIEW=dashboard
# writes  ignore/snapshots/dashboard.png
```

### All nine views

```bash
make tui-snap-all
# writes  ignore/snapshots/{dashboard,licenses,...,settings}.png
```

Output files land in `ignore/snapshots/` which is git-ignored.

### Custom size or seed

The underlying script accepts positional arguments:

```bash
bash scripts/tui-snap.sh dashboard 160 48 scripts/tui-snap-seeds/dashboard.json
#                        <view>    <w> <h> <seed>
```

## Reference image

`ignore/01-dashboard.png` is the target design.
After running `make tui-snap VIEW=dashboard`, open both files side by side and
compare layout, colour, and content.

## Iterating on style

1. Edit `internal/manager/tui/screen_dashboard.go` — layout, tile labels,
   box titles, column weights.
2. Edit `internal/manager/tui/theme.go` — colours, border styles, pill styles.
3. Re-run `make tui-snap VIEW=dashboard`.
4. Repeat until `ignore/snapshots/dashboard.png` matches the reference.

## Seed files

`scripts/tui-snap-seeds/<view>.json` controls what data the snapshot tool
injects via `cmds.DashboardSnapshotMsg`. Fields:

| Field | Description |
|---|---|
| `active` / `revoked` / `expired` / `expiring_soon` / `superseded` | Counter values shown in the tile row |
| `active_key_id` / `active_key_name` / `active_key_fingerprint` | Active issuer key shown in the left-column box |
| `servers[].name/on/url/requests` | HTTP server status rows |
| `audit[].at/kind/target/actor/note` | Recent audit events (up to 5) |

## tui-snap binary flags

```
-view  dashboard|licenses|issuers|recipients|identities|revocation|servers|audit|settings
       |onboarding|onboarding-step<0-3>|passphrase
       |wizard|wizard-step<1-8>
       |overlay-confirm|overlay-confirm-danger|overlay-error|overlay-quit|overlay-quit-servers
       |overlay-revoke|overlay-input|overlay-qr|overlay-filepicker|overlay-help
-width  144   terminal width in cells
-height 44    terminal height in cells
-seed   path  seed JSON file (optional, dashboard only)
-keys   "1 d / esc"   space-separated key presses to send after seed
-mouse  "x,y[,left|right]"  single mouse click after layout
```

Output is raw ANSI text on stdout; pipe to `freeze` for PNG.

### Onboarding sub-steps

```bash
bash scripts/tui-snap.sh onboarding          # step 0 — welcome banner
bash scripts/tui-snap.sh onboarding-step1    # step 1 — passphrase (shows étape 2/4)
bash scripts/tui-snap.sh onboarding-step2    # step 2 — issuer     (shows étape 3/4)
bash scripts/tui-snap.sh onboarding-step3    # step 3 — first lic  (shows étape 4/4)
```

### Wizard sub-steps

```bash
bash scripts/tui-snap.sh wizard              # step 1 — Identity (default)
bash scripts/tui-snap.sh wizard-step5        # step 5 — Validity
bash scripts/tui-snap.sh wizard-step8        # step 8 — Review & Issue
```

## Interactive testing

`internal/manager/tui/teatest_smoke_test.go` drives a real `tea.Program` loop
using `github.com/charmbracelet/x/exp/teatest`. These tests confirm that:

- The dashboard renders and seeds correctly end-to-end.
- All nine tab keys navigate to their respective screens.
- A mouse click on the Active tile dispatches `SwitchToLicensesMsg`.

Run them with:

```bash
go test ./internal/manager/tui/... -run TestTeatest -v
```
