# license-manager TUI — E2E test tracking

Living document. Every feature, every interaction, every screen has a row.
A change touching the TUI updates the matching `Last verified` date.

## Coverage progress

| Date | Batch | Tests added | Bugs surfaced + fixed |
|---|---|---|---|
| 2026-05-25 | 1 | TitleHintsSynthesiseKey × 24 chips | — |
| 2026-05-25 | 3 | AuditFilterChips × 6, LicenseFilterChips × 6, LicenseDetailTabs × 5 | screen_licenses tableHeaderY hard-coded (off-by-N) + detail-tab strip Y off-by-1 (fixed in 729376f) |
| 2026-05-25 | 6 | TopTabBarClicks × 10 | — |
| 2026-05-25 | 4 | WizardSidebarSteps × 8, WizardReviewButtons (Issue + Cancel) | StepReview.View() recorded button Y as slice index instead of rendered row (fixed in d017f75) |
| 2026-05-25 | 5 | ServersSubTabBar × 3, ServersActionChips × 5 | — |
| 2026-05-25 | 2 | DashboardTiles × 5 | — |
| 2026-05-25 | 7 | SettingsKeyboard × 11 (rekey/vacuum/backup/argon×3/theme×3/toggle×2) | — |
| 2026-05-25 | 10 | ServerLogFilters × 4 (keys 1-4) + RootKeys_AZ_NoPanic | — |
| 2026-05-25 | 11 | ConfirmOverlay (y+n), InputOverlay (typed enter), OK+Error (dismiss) | — |
| 2026-05-25 | 8 | OnboardingHappyPath (welcome→passphrase→issuer→license, payload assert) | — |
| 2026-05-25 | 9 | WizardFullHappyPath (8 steps msg injection + final state assertion) | — |

**Total: ~95 interactions covered by auto:click + auto:teatest tests this session. 2 layout bugs caught + fixed.**

## Legend

| Symbol | Meaning |
|---|---|
| ✅ | Verified PASS — test green / VHS frame matches / manual ok |
| ❌ | Verified FAIL — open bug, must fix |
| ⏸ | Blocked — needs upstream change or infra |
| ❓ | Not yet verified — interaction exists, test missing |
| — | Not applicable |

| Test method | Notation |
|---|---|
| `auto:teatest` | `internal/manager/tui/*_test.go` runs `Update()` + asserts |
| `auto:snap` | golden-file diff via `snapshot_test.go` (`UPDATE_GOLDEN=1` to regen) |
| `auto:click` | `screen_click_test.go` — synthesises `tea.MouseMsg` + asserts `Cmd` |
| `vhs` | tape under `tapes/` rendered to `tapes/out/*.gif` |
| `manual` | operator clicks through `bin/license-manager` |

---

## Chrome (global)

| Feature | Method | Status | Last verified | Note |
|---|---|---|---|---|
| Title bar render | auto:snap | ✅ | 2026-05-25 | included in every screen snapshot |
| Tab strip render + active underline | auto:snap | ✅ | 2026-05-25 | bottom border adds row → `TopChromeRows=4` |
| Tab click navigation (1-9, 0) | auto:click | ❓ | — | needs test |
| Tab Tab/Shift+Tab cycle | auto:teatest | ❓ | — | |
| Breadcrumb extras render | auto:snap | ✅ | 2026-05-25 | |
| Status bar hint render | auto:snap | ✅ | 2026-05-25 | per-screen via Hints() |
| Global `q` quit + servers-on confirm | auto:teatest | ✅ | prior session | `quitOverlay` covers it |
| Global `?` help overlay | auto:click | ❓ | — | overlay click covered, key path not asserted |

## Theming

| Feature | Method | Status | Last verified | Note |
|---|---|---|---|---|
| Neon palette renders | vhs `tapes/themes.tape` | ✅ | 2026-05-25 | |
| Mono palette renders | vhs `tapes/themes.tape` | ✅ | 2026-05-25 | |
| Nord-soft palette renders | vhs `tapes/themes.tape` | ✅ | 2026-05-25 | |
| `ApplyTheme()` swaps Palette + reseeds styles | auto:teatest `TestApplyTheme_SwapsPaletteAndStyles` | ✅ | 2026-05-25 | |
| settings click → live palette change | manual | ❓ | — | needs operator check |
| widgets/ caches don't pick up theme until restart | — | ✅ | 2026-05-25 | documented limitation |
| **Rule**: no inline `lipgloss.NewStyle().Foreground(Palette.X)` outside theme.go | auto:grep | ❓ | — | needs grep test |

---

## Screen: Dashboard

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Tile `Actives` → licenses filter=active | `a` | tile rect | auto:click | ✅ | widget tree dispatch |
| Tile `Révoquées` → filter=revoked | `r` | tile rect | auto:click | ✅ | |
| Tile `Expirées` → filter=expired | `e` | tile rect | auto:click | ✅ | |
| Tile `Expirent <7j` → filter=expiring | `w` | tile rect | auto:click | ✅ | |
| Tile `Superseded` → filter=superseded | `u` | tile rect | auto:click | ✅ | |
| Active issuer box → Issuers tab | `k` | box row | auto:click | ❓ | |
| Server `[N]` rows → toggle | — | server-row | auto:click | ❓ | covered by `dashboardServerToggleMsg` |
| Shortcut grid cells | n/x// k i ? | grid cell | auto:click | ❓ | |
| Heatmap intensity (1-2 vs 3+) | — | — | auto:snap | ❓ | regressed pass-2, fixed |
| Widget tree cache invalidation | — | — | auto:teatest `TestDashboardCacheReused` | ✅ | 2026-05-25 |

## Screen: Licenses

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Title hint `[d]` détail | d | hint chip | auto:click | ❓ | |
| Title hint `[n]` nouvelle | n | hint chip | auto:click | ❓ | |
| Title hint `[x]` révoquer | x | hint chip | auto:click | ❓ | |
| Title hint `[e]` re-émettre | e | hint chip | auto:click | ❓ | |
| Filter chip pills (all/active/expiring/expired/revoked/superseded) | f cycle | pill 3-row | auto:click | ❓ | Y range fixed today |
| Search input `/` | `/` | search area | manual | ❓ | |
| Table row select | ↑↓/click | data row | auto:click | ❓ | **bug fixed today** |
| Detail tabs [I/B/P/A/C] | I/B/P/A/C | tab strip | auto:click | ❓ | |
| Detail PEM `[c]` copy | c | hint | auto:teatest | ❓ | |
| Audit-tab loading state | — | — | auto:teatest | ❓ | |

## Screen: Issuers

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Title hint `[n]` générer | n | hint chip | auto:click | ❓ | |
| Title hint `[a]` activer | a | hint chip | auto:click | ❓ | |
| Title hint `[E]` export .pub | E | hint chip | auto:click | ❓ | |
| Title hint `[x]` retraiter | x | hint chip | auto:click | ❓ | |
| Table row select | ↑↓/click | data row | auto:click | ❌→✅ | **user-reported bug fixed `726eaf8`** |

## Screen: Recipients

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Title hints [n/i/E/x] | n/i/E/x | hint chip | auto:click | ❓ | same fix applied |
| Table row select | ↑↓/click | data row | auto:click | ❓ | **fixed today** |

## Screen: Identities

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Title hints [n/E/R/x] | n/E/R/x | hint chip | auto:click | ❓ | |
| Table row select | ↑↓/click | data row | auto:click | ❓ | **fixed today** |

## Screen: Revocation

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Title hints [n/x/d/r] | n/x/d/r | hint chip | auto:click | ❓ | |
| Table row select | ↑↓/click | data row | auto:click | ❓ | **fixed today** |
| 3 KPI tiles | — | tile | — | ❓ | not yet clickable |

## Screen: Servers

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Sub-tab Revocation | R | sub-tab bar | auto:click | ❓ | |
| Sub-tab Heartbeat | H | sub-tab bar | auto:click | ❓ | |
| Sub-tab Probe | P | sub-tab bar | auto:click | ❓ | |
| Probe inner Tokens | t | inner tab | auto:click | ❓ | |
| Probe inner History | h | inner tab | auto:click | ❓ | |
| Probe inner Live | l | inner tab | auto:click | ❓ | |
| Action chip `[s]` start/stop | s | chip in action bar | auto:click | ❓ | |
| Action chip `[e]` edit bind | e | chip | auto:click | ❓ | |
| Action chip `[g]` regen token | g | chip | auto:click | ❓ | |
| Action chip `[c]` clear log | c | chip | auto:click | ❓ | |
| Action chip `[a]` autoscroll | a | chip | auto:click | ❓ | |
| Status pill ON/OFF toggle | — | pill rect | auto:click | ❓ | |
| Global `A` start-all | A | — | auto:teatest | ❓ | no visual target |
| Global `Z` stop-all | Z | — | auto:teatest | ❓ | |
| Log viewport wheel | wheel | log area | manual | ❓ | |

## Screen: TOTP

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Title hints [n/E/x/r] | n/E/x/r | hint chip | auto:click | ❓ | |
| Table row select | ↑↓/click | data row | auto:click | ❓ | **fixed today** |
| QR detail rendering | — | — | auto:snap | ❓ | |

## Screen: Audit

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Filter chips f/l/k/s/i/p | hotkey | chip pill | auto:click | ❓ | Y range was correct |
| `[d]` detail | d | hint | — | ❓ | not yet clickable |
| `[r]` refresh | r | — | — | ❓ | not yet clickable |
| Export CSV / JSON | E / J | text hint | — | ❓ | not yet clickable |
| Table row select | ↑↓ | — | — | ❓ | **not yet clickable** |

## Screen: Settings

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Argon preset 1/2/3 | 1/2/3 | row | auto:click | ✅ | hitmap-based |
| DB rekey [P] | P | row | auto:click | ✅ | |
| DB vacuum [V] | V | row | auto:click | ✅ | |
| DB backup [B] | B | row | auto:click | ✅ | |
| Theme [N]/[M]/[O] | N/M/O | marker | auto:click | ✅ | live palette swap |
| Toggle confirm_quit | Q | toggle row | auto:click | ✅ | |
| Toggle auto_start | U | toggle row | auto:click | ✅ | |

## Screen: Passphrase

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Submit | enter | — | auto:teatest | ✅ | covered in workflows_e2e_test |
| Cancel | ctrl+c | — | auto:teatest | ✅ | |

## Screen: Onboarding

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Step 0 Welcome → next | enter | any body click | auto:click | ❓ | added today |
| Steps 1-3 forms | tab/enter | — | manual | ❓ | keyboard-only by design |

## Wizard (overlay)

| Interaction | Hotkey | Click target | Method | Status | Note |
|---|---|---|---|---|---|
| Sidebar step badges 1-8 | digit | sidebar row | auto:click | ❓ | |
| Step 1 issuer row select | ↑↓/click | row | auto:click | ❓ | added pass-2 |
| Step 2 recipient row select | ↑↓/click | row | auto:click | ❓ | |
| Step 3 machine paste/probe | tab | — | — | ❓ | input-driven |
| Step 4 binary file picker | ctrl+f | body click | auto:click | ❓ | |
| Step 5 validity preset chips | ctrl+w/m/y/f | shortcut row | auto:click | ❓ | |
| Step 6 freefields | a/d | — | — | ❓ | deferred |
| Step 7 TOTP toggle + row | t/↑↓ | row | auto:click | ❓ | original click coverage |
| Step 8 Issue/Cancel buttons | enter/esc | button row | auto:click | ❓ | added pass-2 |

## Overlays

| Overlay | Buttons / chips | Click | Method | Status |
|---|---|---|---|---|
| confirm | OK / Cancel | y/n | auto:click | ✅ |
| quit | Y / N | y/n | auto:click | ✅ |
| input | OK / Cancel | enter/esc | auto:click | ✅ |
| revoke | OK / Cancel + 5 suggestion chips | enter/esc/click chip | auto:click | ✅ (chips added pass-2) |
| error | dismiss | any | auto:click | ✅ |
| ok | dismiss | any | auto:click | ✅ |
| help | dismiss | any | auto:click | ✅ |
| qr | save / copy PEM / close | s/c/esc | auto:click | ✅ (added pass-2) |
| filepicker | row select + navigate | enter/click row | auto:click | ✅ (added pass-2) |
| probe drawer | copier / confirmer / annuler | c/enter/esc | auto:click | ✅ (added pass-2) |

---

## How to add a test

1. Pick a row marked `❓` from the table above.
2. If `auto:click`: add a case to `screen_click_test.go::TestClickMapping`.
3. If `auto:snap`: add an entry to `snapshot_test.go`, run with `UPDATE_GOLDEN=1`, eyeball the `.golden`, commit it.
4. If `vhs`: add a `.tape` under `tapes/`, wire a Makefile target.
5. Run `go test ./internal/manager/tui/...` green.
6. Update this file: change `❓` to `✅` and set `Last verified` date.

## How to record a manual failure

1. Add a row to `## Manual regressions` below with date, reproduction, expected vs actual.
2. Create a task in the conversation; commit fix; mark row ✅ + date.

## Manual regressions

| Date | Screen | Reproduction | Expected | Actual | Fixed by |
|---|---|---|---|---|---|
| 2026-05-25 | Issuers | Click on a table data row | Row selected | Click landed BESIDE the row instead | `726eaf8` (off-by-N click mapping) |
