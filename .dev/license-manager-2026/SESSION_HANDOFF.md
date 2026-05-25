# License-manager TUI — session handoff

Snapshot of the work pushed during the 2026-05-22 and 2026-05-25 sessions
and what's still queued. All commits are on `master`, authored as
`oioio-space <oioio-space@users.noreply.github.com>`.

## 2026-05-25 — Architecture refactor (research-driven)

Research pass on Bubble Tea best practices (VHS, teatest, lipgloss v2,
stickers/bubblelayout, focus patterns) followed by a 7-commit refactor of
the existing TUI. Goals: eliminate magic numbers, prepare for golden-file
test growth, centralize theme conventions.

### Commits shipped

| SHA | Title |
|---|---|
| `dc200a0` | refactor(tui): adopt BoxedInner/BoxedWidth in licenses screen |
| `0752b83` | refactor(tui): adopt BoxedInner/BoxedWidth in list screens |
| `4f8e302` | feat(tui): BoxedInner/BoxedWidth helpers + pinning test |
| `d16713e` | refactor(tui): use shared theme styles in settings screen |
| `36e04b0` | refactor(tui): adopt ChromeRows constant in 4 sites |
| `e9b8ab0` | refactor(tui): rename PassphraseResult → PassphraseResultMsg |
| `7ce8e5a` | feat(tui): layout constants + overlay resize fix + msg dump |

### New infrastructure

- **`internal/manager/tui/layout_constants.go`** — single source of truth
  for chrome / box / modal frame metrics.
  - `ChromeRows = 4` (title + tabs + breadcrumb + statusbar)
  - `BoxFrame() (w, h int)` — BoxStyle border + padding overhead (4, 2)
  - `ModalFrame() (w, h int)` — Modal style overhead (6, 4)
  - `BoxedInner(total) int` — inner content width inside BoxStyle
  - `BoxedWidth(total) int` — value to pass to `BoxStyle.Width(...)` for
    a rendered width of `total` cells
  - `FrameOf(s lipgloss.Style) (w, h int)` — ad-hoc frame query
- **`internal/manager/tui/layout_constants_test.go`** — pins the resolved
  values so any future BoxStyle tweak surfaces immediately as a test
  failure instead of silently shifting every screen.
- **`internal/manager/tui/debug.go`** — opt-in `tea.Msg` dump gated by
  `LICENSE_MANAGER_TUI_DUMP=/path/to/log` env var. RFC3339Nano timestamp
  + `%#v` per message. Useful for diagnosing message-routing bugs without
  rebuilding.

### Bug fixed

- **Overlay resize bug** (`7ce8e5a`, app.go `updateOverlay`): when the
  terminal was resized while an overlay was open, the `tea.WindowSizeMsg`
  was never forwarded to the backing screens. Now `updateOverlay`
  broadcasts the resize to all sub-models before normal routing so the
  layout reflects the new size as soon as the overlay closes.

### Magic numbers eliminated

| Screen | Before | After |
|---|---|---|
| dashboard / settings / issuers / app.go | `m.height - 4`, local `chromeRows`, `m.hgt - (4 + ...)` | `ChromeRows` |
| identities / issuers / revocation / totp / audit (vp) / licenses | `m.width - 4` (BoxStyle inner) | `BoxedInner(m.width)` |
| identities / issuers / revocation / totp / licenses | `m.width - 2` (passed to `.Width()`) | `BoxedWidth(m.width)` |
| licenses | `m.hgt - 6` (chrome + box vert) | `m.hgt - ChromeRows - boxV` (boxV from `BoxFrame()`) |
| licenses | `m.width - 4 - 14` (box-inner minus label gutter) | `BoxedInner(m.width) - labelGutter` |

Values are arithmetically identical at runtime — all 21 golden files
remained valid without regeneration.

### Naming convention

- `PassphraseResult` → `PassphraseResultMsg` (the lone outlier in the
  `*Msg` convention).
- 14 `wizard.*Msg` types audited for privatization — **kept exported**
  because `wizard_test.go` (package `tui_test`) consumes them; the
  package lives under `internal/` so privatizing them is busy-work that
  breaks 20+ tests for zero external-API benefit.

### Settings styles centralized

`screen_settings.go` had two inline `lipgloss.NewStyle().Foreground(Palette.X)`
blocks that exactly matched the existing `GlowGreen` / `Mute` theme
vars. Replaced (4 sites). Other screens use Palette colors directly
through helpers — not generic enough to extract further without
introducing per-screen style names.

### Widget styles — kept as-is, with reason

`widgets/statusbar.go`, `widgets/tabbar.go`, `widgets/tile.go`,
`widgets/button.go` already use `sync.OnceValue` to cache style structs
built from `core.Colors` (populated by `theme.go init()`). Moving them
to `theme.go` would break the `widgets → core ← tui` cycle that was
deliberately set up; locality wins over centralization here.

### Deferred (the more invasive remaining items)

- **`focusStack` for screens.** The overlay pile (`m.overlays`) is a
  real stack; screens themselves have no `Focus()`/`Blur()` and routing
  is piloted by `m.active`. Adding explicit per-screen focus would be a
  large structural change and deserves a dedicated session with explicit
  architectural sign-off.
- **VHS regression tapes** (still queued from the previous session).
  Now feasible because the layout constants give stable cell widths.
- **Per-screen teatest tests with new goldens** for paths not covered
  by the existing 21 (overlays in particular).
- **lipgloss v2 migration** — `AdaptiveColor` → `lipgloss.LightDark()`,
  listen to `tea.BackgroundColorMsg`. Cosmetic; do it when bumping
  another dep.


## 2026-05-22 session — Commits shipped (newest first)

| SHA | Title |
|---|---|
| `98632f6` | fix(tui): propagate handler cmds + wizard popup frame + TOTP layout |
| `e39016c` | feat(tui/wizard): collect Subject + Audience in step 6 |
| `2687763` | fix(tui): half-block QR + list refresh + license detail clipping |
| `d76971d` | fix(tui): wizard sub-overlay routing + missing click/key handlers |
| `0a7c03a` | refactor(service): rename TOTPService.GetByID → ByID (no Get prefix) |
| `e30aa53` | feat(tui): TOTP top-level tab with full CRUD + QR export |
| `7923804` | feat(tui): persist settings + clickable server action bar |
| `1e8111f` | fix(tui): wizard polish + audit empty-state + list-screen overflow |
| `658f8f3` | fix(tui): server tick + missing key handlers across screens |
| `07f46de` | feat(tui): gradient progress bars via bubbles/progress |
| `13e29dd` | fix(tui/licenses): always render the bordered detail box |
| `07b411e` | feat(tui/servers): wire admin token regen end-to-end |
| `4fc62bb` | feat(tui): dashboard heatmap fed from real licence dates |
| `852458f` | fix(tui): quit overlay never quit + always fired with stopped servers |

`git log --oneline --author=oioio-space -25` to see the full list including
the prior batch from the same day.

## What was fixed (user-reported items, in the order they came in)

- ✅ Quit overlay misfiring + "quit anyway" not actually quitting (`852458f`).
- ✅ Dashboard GitHub-style 91-day heatmap of licence issuance/expiry +
  per-server request-rate sparkline (`4fc62bb`, sparkline shipped in
  `852458f`).
- ✅ Server admin-token regen end-to-end with KEK-wrapped persistence and
  one-shot cleartext display (`07b411e`).
- ✅ License detail card always renders even on empty selection (`13e29dd`).
- ✅ Gradient progress bars (wizard + onboarding strip + new
  `renderHealthBar` used in the licence "validity" row) (`07f46de`).
- ✅ Server `[e]` / `[a]` keys wired + server status tick actually fires
  from root Init (`658f8f3`).
- ✅ Wizard step 1 inputs (unit-tested OK — was a stale-binary issue) +
  step 4 file-picker race (`tea.Sequence` so pop happens before path msg
  fires) + step 5 ctrl+w/m/y/f shortcuts + step 6 enter/ctrl+s submit
  (`1e8111f`).
- ✅ Audit detail panel always renders with empty / row-selected /
  payload-open variants (`1e8111f`).
- ✅ Issuers / Recipients / Identities / Revocation overflow fix via shared
  `listTableHeight(hgt, width, intro)` helper that measures the actual
  wrapped intro height (`1e8111f`).
- ✅ Settings persistence: argon preset + ConfirmQuit + AutoStart toggles
  via `svc.Settings.Update`; clickable server action bar at Y=height-2
  (`7923804`).
- ✅ TOTP top-level tab with full CRUD: schema edge made optional,
  `service.TOTP.List/Generate/Delete/ByID`, new `screen_totp.go` with
  list table + always-visible QR box; wizard step 7 selector reads from
  the same shared pool; `0`-key shortcut for the 10th tab so Settings
  stays keyboard-reachable (`e30aa53`).
- ✅ Naming: renamed `GetByID` → `ByID` per the no-Get-prefix policy
  (`0a7c03a`).
- ✅ Wizard sub-overlay routing: `pushOverlayMsg` no longer swallowed by
  the active overlay's Update — root unconditionally intercepts it so
  the file picker, error overlays, etc. stack on top of the wizard
  (`d76971d`).
- ✅ Wizard navigation: explicit `ctrl+right` / `ctrl+n` next-step,
  `ctrl+left` / `ctrl+p` prev-step, `ctrl+x` discard alias (`d76971d`).
- ✅ Step 7 "Require TOTP" toggle + secret rows now clickable; review
  screen refreshes its state snapshot on every `initStep` so direct
  sidebar jumps display the accumulated data (`d76971d`).
- ✅ Probe sub-tab (T / H / L) clickable in the Servers screen
  (`d76971d`).
- ✅ Settings theme is a real persisted field now (enum
  neon/mono/nord-soft, schema migration regenerated) (`d76971d`).
- ✅ QR ASCII compact (half-block ▀▄█ — ~15 lines instead of ~29) —
  fixes both the TOTP screen QR and the license-issued popup's QR
  (`2687763`).
- ✅ License list refresh on issue/revoke: data-loaded msgs route to
  their owning model before the overlay short-circuit; revoke result
  capture via `pendingCmd` (`2687763`).
- ✅ License detail field truncation + clickable `[I/B/P/A/C]` tab strip
  (`2687763`).
- ✅ Revoke modal: suggestion chips wrap onto multiple rows instead of
  overflowing (`2687763`).
- ✅ Wizard Subject + Audience inputs in step 6 (`e39016c`).
- ✅ Identity "create new binary" + all other screens' input/confirm
  result cmds — five sites were silently dropping the returned `tea.Cmd`
  via `_, _ := … ` (`98632f6`).
- ✅ Wizard now renders inside a bordered Modal popup; mouse coords
  offset by the frame (`98632f6`).
- ✅ TOTP screen: 2-col layout only kicks in when total width ≥
  ~88 cells; narrower terminals stack so neither column overflows
  (`98632f6`).

## What is still queued / known to need follow-up

These are the items the user flagged on the last test pass that are NOT yet
covered by a pushed fix. Resume from these:

1. **Theme persistence has no runtime effect.** The Setting.Theme field
   persists, but the lipgloss palette is initialised at boot from a global
   `Palette` and never re-evaluated. To make the theme switch visible, the
   palette has to become a `ThemeID → palette` function called from each
   View that reads colours (or a global pointer swapped on
   `settingsSetThemeMsg`).
2. **vhs E2E test pass requested by the user.** `vhs` (v0.11.0) is
   installed at `~/go/bin/vhs`. No `.tape` files exist yet. The user
   asked specifically for vhs-driven regression tapes that exercise:
   - dashboard → wizard discard (`ctrl+x` / `ctrl+c` / esc paths)
   - full wizard create → review → emit flow, including the new
     Subject + Audience fields in step 6
   - licence revoke → confirm + list refresh
   - server tab key/click interactions including `[e]/[g]/[c]/[a]` chips
   - TOTP create → QR display → PNG export
   See `Makefile` for the existing `make license-manager` target; vhs
   tapes typically live under `tape/` or `docs/tapes/`.
3. **Wizard "back" via esc may collide with textinput.** `ctrl+left` /
   `ctrl+p` were added as an escape hatch but `esc` is still the natural
   reflex — the wizard parent catches `esc` before the step, which works
   today but breaks the expected "esc clears the active textinput". User
   may want this rebalanced.
4. **Step 3 paste mode hint** — `enter` on empty now skips (good), but the
   hint strip still implies enter "confirms". Update copy.
5. **Step 5 date picker affordance** — currently a textinput with
   shortcuts. User asked once for a "date picker"; not delivered. Could
   be a calendar grid widget bound to ↑/↓/←/→.
6. **License detail panel content tab keys (`[I/B/P/A/C]`)** are click +
   keyboard reachable, but the bodies for `B`/`P`/`A`/`C` may show empty
   placeholders depending on the licence record. Worth a pass.
7. **README + docs/mitre.md not updated** for the TOTP tab + new schema
   field. Pre-commit-check skill flags this but didn't block — fix
   alongside the next feature commit.

## How to resume

```bash
cd /home/mathieu/GolandProjects/maldev
git pull
git log --oneline --author=oioio-space -20    # session commits
make license-manager                          # rebuild bin/license-manager
go test ./internal/manager/tui/... -count=1   # green at session end
bin/license-manager                            # interactive smoke-test
```

Open this file (`.dev/license-manager-2026/SESSION_HANDOFF.md`) to see the
full punch list. Active in-memory tasks at session end: #26 completed
(Wizard Subject+Audience). All other tracked tasks (#17-#25) were closed
during the session.

## Notes / gotchas captured during the session

- **Cmd-dropping bug pattern.** Every `handle*Result` returns a
  `(model, tea.Cmd)` but the dispatcher historically did
  `updated, _ := …` and lost the cmd. Always assign to `m.pendingCmd`
  (drained by `updateOverlay`).
- **pushOverlayMsg under an overlay.** Root's `updateOverlay` now
  unconditionally intercepts `pushOverlayMsg` and appends — previously
  the active overlay caught it and dropped it, breaking
  wizard-from-overlay flows.
- **Data-load msgs across overlays.** `LicensesLoadedMsg` /
  `IssuersLoadedMsg` / `TOTPLoadedMsg` are routed to their owning model
  BEFORE the overlay short-circuit so lists stay fresh while a modal is
  on top.
- **Tab strip > 9 tabs.** Adding TOTP made 10 tabs; the tab strip + key
  handler now use `0` for position 10. Tests using `'9'` for Settings
  had to be updated.
- **Wizard popup frame offset.** When wrapped in `Modal`
  (`border(1) + padding(1,2)`), all mouse coords need `frameX=3,
  frameY=2` adjustment.
- **QR sizing.** `totp.QRImageASCIICompact` uses half-block characters
  for a ~half-height QR. Both `service.TOTP.Get/ByID` and the licence
  Issue path feed this variant; the old `QRImageASCII` is still
  exported but unused at runtime.
