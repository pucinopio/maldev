# License-manager TUI — session handoff

Snapshot of the work pushed during the 2026-05-22, 2026-05-25 and 2026-05-27
sessions and what's still queued. All commits are on `master`, authored as
`oioio-space <oioio-space@users.noreply.github.com>`.

## 2026-05-27 — Operator defects 2.0, truth tables, visual E2E

Continues Session 7 (10 operator defects). This session shipped **22
distinct work items**, all green in CI (`go test ./internal/manager/tui/
-count=1` → ok in ~7.8s). Below is the delta — pick any row to resume.

### A. Licenses screen — operator-reported

| Item | Files | Status |
|---|---|---|
| Stress test 100 licences (rendering + scroll + N=100 visual) | `screen_stress_100_test.go`, `screen_stress_render_test.go` | ✅ |
| Import licence via filepicker `[i]` | `screen_licenses.go` (+ `licenseImportPickedMsg`/`licenseImportedMsg`) | ✅ |
| Filter chip click mapping (bordered AND compact modes) | `screen_licenses.go` (new `chipHitRow` type) | ✅ |
| Doc chaîne de succession | (conversational explanation) | ✅ |
| Export licence `[E]` via input overlay → ExportPEM | `screen_licenses.go`, `overlay_ids.go`, `app.go` | ✅ |

### B. Issuers screen

| Item | Files | Status |
|---|---|---|
| Stress test 100 issuers | `screen_stress_100_test.go` | ✅ |
| Detail panel open by default (`detail: true`) | `screen_issuers.go` | ✅ |
| Import issuer via filepicker `[i]` | `screen_issuers.go` (+ `issuerImportPickedMsg`) | ✅ |
| **`[K]` export private key — full flow** | `screen_issuers.go`, `app.go`, `service/issuer.go` (new `ExportPrivate`) | ✅ |
| Action chip alignment fix (`[x]` → HintKey w/red) | `screen_issuers.go` | ✅ |
| Extension `.priv` (was `.key`) | `screen_issuers.go` | ✅ |

### C. Revocation screen — Detail panel

| Item | Files | Status |
|---|---|---|
| Detail panel — was missing entirely | `screen_revocation.go` (`renderDetail`, `renderDetailLicenseContext`) | ✅ |
| Lazy-load underlying licence via `License.Get` | `loadDetailCmd`, `revocationLicenseLoadedMsg` | ✅ |
| Detail open by default + `[d]` toggle hint | `screen_revocation.go` | ✅ |

### D. All list screens — column auto-sizing

| Item | Files | Status |
|---|---|---|
| `fitColumns(t, w, mins, weights)` + bidirectional resize | `layout.go` | ✅ |
| `setAutoFitRows(t, w, weights, rows, cap)` — content-aware ideal widths | `layout.go` | ✅ |
| 7 screens migrated to `setAutoFitRows` | `screen_*.go` × 7 | ✅ |
| Stretch + shrink under min-sum (proportional w/ floor=4) | `layout.go` (`fitColumns` 3 phases) | ✅ |

### E. Vertical overflow — global audit

| Item | Files | Status |
|---|---|---|
| `ChromeRows` (4, Y-origin) vs `ContentReservedRows` (5, height-reservation) split | `layout_constants.go` | ✅ |
| Breadcrumb pre-truncate to avoid soft-wrap | `chrome.go` | ✅ |
| titleBar drops rightmost hints when overflow | `title_hints.go` | ✅ |
| `clipDetailBox` truncates middle, preserves bottom border | `layout.go` | ✅ |
| Per-screen `remaining` budget in `View()` | 5× `screen_*.go` | ✅ |
| `TestNoOverflow_FullMatrix` — 40 cases (5 sizes × 8 scenarios) | `screen_stress_render_test.go` | ✅ |

### F. PEM tab — scroll + navigation

| Item | Files | Status |
|---|---|---|
| `[P]` mouse-click loads PEM (case was missing) | `screen_licenses.go` (`licenseDetailTabClickMsg` case 2) | ✅ |
| `renderDetailPEM` purified (no SetContent in View) | `screen_licenses.go` | ✅ |
| **Multi-binding scroll**: `j`/`k`/`space`/`b`/`pgup`/`pgdn`/`g`/`G` + `↑`/`↓` | `screen_licenses.go` | ✅ |
| Scroll indicator `[lignes N-M/T · P %]` in header | `screen_licenses.go` (`renderDetailPEM`) | ✅ |
| Dual affordance: arrows nav table + PEM auto-reload via `tableSelectRowMsg` | `screen_licenses.go` | ✅ |
| Viewport height sized from layout reservation | `screen_licenses.go` (`WindowSizeMsg`) | ✅ |

### G. Truth tables — data-driven key dispatch tests

| Screen | File | Sub-tests |
|---|---|---|
| Licenses | `screen_licenses_keytable_test.go` | 29 |
| Issuers | `screen_issuers_keytable_test.go` | 10 |
| Recipients / Identities / Revocation | `screen_others_keytable_test.go` | 18 |
| TOTP / Audit | `screen_totp_audit_keytable_test.go` | 11 |
| Dashboard cross-screen hotkeys + tabs | `dashboard_xscreen_keytable_test.go` | 19 |
| Licenses search state machine | `screen_licenses_search_e2e_test.go` | 1 (5 sub-checks) |
| **Total** | | **88+** |

### H. Visual E2E — Windows-compatible (`cmd/tui-gif`)

| Item | Files | Status |
|---|---|---|
| Driver + GIF encoder, pure Go | `cmd/tui-gif/main.go` (~600 lines) | ✅ |
| Go Mono TTF embedded (truetype, unicode) | `golang.org/x/image/font/gofont/gomono` | ✅ |
| ASCII fallback for glyphs Go Mono misses (`◆ → *`) | `cmd/tui-gif/main.go` (`toASCII`) | ✅ |
| ANSI SGR parser (8-color + 256-color + truecolor) | `cmd/tui-gif/main.go` (`applySGR`) | ✅ |
| Tape directive `Seed <kind> <N>` | `cmd/tui-gif/main.go` (`buildSeedMsg`) | ✅ |
| 7 fixture builders exported | `internal/manager/tui/seed_fixtures.go` | ✅ |
| 7 unit tests for fixture builders | `seed_fixtures_test.go` | ✅ |
| 7 tapes covering PEM scroll + nav + tabs + filter + search | `vhs/tui-gif/*.tape` | ✅ |
| Wrapper script | `scripts/tui-gif-record.sh` | ✅ |
| Unit tests for tui-gif itself | `cmd/tui-gif/main_test.go`, `main_debug_test.go` | ✅ |

### I. Other scoped fixes

| Item | Files | Status |
|---|---|---|
| `IssuerService.ExportPrivate` + test | `service/issuer.go`, `service/issuer_test.go` | ✅ |
| `Seed*Msg` × 7 + tests | `seed_fixtures.go`, `seed_fixtures_test.go` | ✅ |

### How to verify on a fresh machine

```bash
# 1. Compile everything
go build ./...

# 2. Full TUI test suite (truth tables + integration + snapshots)
go test ./internal/manager/tui/ -count=1 -timeout 120s

# 3. Service tests (exported new ExportPrivate)
go test ./internal/manager/service/ -count=1

# 4. tui-gif unit tests
go test ./cmd/tui-gif/ -count=1

# 5. Re-record all GIFs (Windows OK — pure Go, no ttyd/ffmpeg needed)
scripts/tui-gif-record.sh
# → vhs/out/*.gif × 7

# 6. Inspect a tape frame-by-frame
TUI_GIF_DUMP="$(pwd)/vhs/out/pem-scroll-all-bindings.gif" \
  go test ./cmd/tui-gif/ -run TestDumpFrames -v
# → vhs/out/*.gif.frames/frame-N.png
```

### Key design decisions

1. **Truth table = executable spec.** `TestLicensesKeyDispatchTruthTable`
   and friends are data-driven (one row per (context, key) → expected effect).
   To change a binding: edit the table, run the test, fix the prod code.
2. **`tui-gif` instead of `vhs` on Windows.** Official `vhs` needs `ttyd`
   (no native Windows build) + `ffmpeg`. `tui-gif` drives the TUI in-process
   via `tea.Model.Update`/`View`, encodes via stdlib `image/gif` + embedded
   Go Mono TTF. Zero external setup.
3. **Fixture seed via exported `Seed*Msg(n, t0)` builders.** Deterministic
   row sets reusable by `tui-gif` AND any test/tool that wants fixtures
   without spinning a service+DB.
4. **Preview-list pattern on PEM tab.** ↑/↓ navigate the table (PEM
   auto-reloads via `tableSelectRowMsg`). j/k/space/b/pgup/pgdn/g/G scroll
   the viewport. Header indicator `[lignes N-M/T · P %]` makes scroll visible.

### Pending / unstarted

Nothing pending from this session. New defects should:

1. Reproduce as a new truth-table row in `*_keytable_test.go`
2. Get fixed in production
3. Verify the truth-table test passes
4. (Optional) Add a VHS tape if the regression is visual

---

## 2026-05-25 — Simplify pass + queued items + VHS infra

Second autonomous pass on the same day. Ran a 3-agent /simplify
(reuse / quality / efficiency) over `internal/manager/tui/`, applied
the surgical findings, closed handoff items #1, #4, #6, #7, and set
up VHS tape infrastructure.

### Commits shipped (newest first)

| SHA | Title |
|---|---|
| `a4d12b7` | feat(tooling): VHS regression tapes + tui-snap -theme flag |
| `844f370` | fix(tui/licenses): license detail tabs B/P/A/C polish |
| `2785230` | docs: add TOTP tab to README + tui-snapshots + Makefile snap target |
| `018edd2` | refactor(tui): more simplify wins — chips, wizard shared styles, audit cache |
| `da82675` | feat(tui): runtime theme swap (handoff item #1) |
| `73b677f` | fix(tui): TOTPLoadedMsg cmd-drop + cancelled sentinel |
| `72abb4c` | refactor(tui): simplify pass — theme reuse + heatmap perf + wizard hint |

### Real bugs fixed (not just polish)

1. **TOTPLoadedMsg cmd-drop** (`73b677f`) — `app.go` Update was
   discarding `loadCursorDetail()` returned by `m.totp.Update`. The
   TOTP screen's QR detail never refreshed after a list reload. Same
   hazard was latent for `LicensesLoadedMsg` / `IssuersLoadedMsg`.
   Fix returns early with the captured cmd, also eliminating the
   double-dispatch via routeToActive.
2. **Theme runtime swap** (`da82675`, handoff item #1) — Persisted
   `Setting.Theme` had no runtime effect. Now `ApplyTheme(name)`
   reassigns Palette + reseeds every theme.go style var +
   `core.Colors`. Wired into both `settingsSetThemeMsg` and
   `SettingsLoadedMsg`. Three palettes ship: `neon`, `mono`,
   `nord-soft`. Limitation: `widgets/` `sync.OnceValue` caches keep
   their boot-time look until restart.
3. **Cancelled sentinel** (`73b677f`) — `screen_wizard` compared
   `err.Error() == "cancelled"`. Replaced with `wizard.ErrCancelled`
   sentinel + `errors.Is`.

### Simplify findings applied

- 10 reuse, 8 efficiency, 8 quality findings reviewed; the safe
  surgical ones landed:
  - `screen_recipients` missed the BoxedInner sweep — fixed.
  - `chrome.go` uses GlowCyan instead of inline cyan+bold.
  - `drawer_probe.go`, `overlay_qr.go` local style shadows now alias
    `Glow*.UnsetBold()` / `Dim`.
  - `screen_dashboard` `serverPillOff` uses `Mute.Bold(true)`.
  - Heatmap cell allocations precomputed (`heatEmpty/Border/Green/Red/OutOfRange`)
    — 91 + 4 lipgloss.NewStyle per frame eliminated.
  - `theme.go` adds exported `ChipActive`/`ChipInactive`; `screen_audit`
    drops 14-line inline chip blocks.
  - `wizard/helpers.go` exposes `wizSel/wizFg/wizDim`; `step_binding_machine`
    drops 5 per-View NewStyle allocations.
  - `screen_audit` View() caches `selectedRow()` to halve `visibleRows()`
    iterations when the detail panel is visible.
- Deferred (need their own focused sessions):
  - `buildWidgetTree` rebuild per dashboard frame (efficiency #2)
  - Overlay-dimensions cached for mouse offset (efficiency #1)
  - Probe table column style hoisting (efficiency #3) — Servers screen is
    architecturally heavier; bundle with widget-tree work
  - `m.height` ↔ `m.hgt` field-name convergence
  - Magic overlay-ID strings → typed constants
  - Custom golden-file harness → `teatest.RequireEqualOutput`

### License detail tabs polish (handoff item #6)

- Audit tab: `detailAuditLoading` flag distinguishes loading from empty;
  rows cleared on tab open to prevent stale data.
- PEM tab: empty-PEM message now states the right diagnosis (storage
  problem, not a "forgot to emit").
- Chain tab: explicit stub note that parent/successor links are not yet
  modeled in the ent schema.

### Docs (handoff item #7)

- README: TOTP / QR / theme switch mentioned in the license-manager
  paragraph.
- `docs/license-manager/tui-snapshots.md`: tenth view listed (totp).
- `Makefile` `TUI_VIEWS`: now includes `totp` so `make snap-all` covers
  all 10 tabs.
- `docs/mitre.md` unchanged (license-manager isn't a technique).

### VHS infra (new)

- `tapes/` directory with three scripts:
  - `themes.tape` — dashboard in neon → mono → nord-soft (proves
    runtime swap visually).
  - `wizard-step3.tape` — paste-mode hint shows the reworded text.
  - `dashboard-smoke.tape` — minimal one-frame render for CI.
- `tui-snap` accepts `-theme <name>` so tapes can drive `ApplyTheme`
  without seeding a settings DB.
- Makefile targets: `tape-themes`, `tape-wizard-step3`, `tape-smoke`,
  and `tapes` (all three). Outputs land under `tapes/out/`
  (git-ignored).
- Requires `vhs`, `ttyd`, `ffmpeg` (already on dev host).
- Verified: three GIFs produced (235K + 70K + 87K), wizard hint
  reword present in raw output, theme ANSI codes change with
  `-theme` flag.

### Tests added

- `theme_test.go` — `TestApplyTheme_SwapsPaletteAndStyles` pins the
  Palette swap + style reseed + fallback for unknown theme names.
- `layout_constants_test.go` (from earlier session) — already pins
  Box/Modal frame metrics + `BoxedInner`/`BoxedWidth` values.

---

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
