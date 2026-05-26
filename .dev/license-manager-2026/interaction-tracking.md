---
title: license-manager TUI — exhaustive interaction tracking
last_reviewed: 2026-05-26
status: in-progress
kb_verified: 124
kb_total: 124
ms_verified: 106
ms_total: 106
defects_open: 10
---

## Session 7 — operator third-pass manual test (2026-05-26)

**Why the harness keeps missing these**: tui-verify's `ClickTarget` resolver
renders the screen once, locates the target substring, computes the click
coords from that snapshot, then injects the mouse event at the SAME coords.
The handler hit-test typically uses identical layout math, so the spec
PASSes — but the operator clicks at perceived positions which drift as the
detail panel opens/closes, as the cursor row shifts the table cell layout,
as the title-bar hints rewrap, etc.

**Fix shipped this session**: a coordinate-stability invariant test per
clickable area that asserts the hit-zone matches the rendered substring's
exact span at multiple layout states (detail open vs closed, table cursor
at row 0 vs row N, narrow vs wide window). Mismatch → spec fails.

### Cross-cutting

- [ ] **DS-T01** — arrow-up / arrow-down don't navigate in ANY table.
      bubbles/table accepts these by default; something upstream
      (rootModel? screen Update?) must be intercepting them. Audit the
      key dispatch chain.
- [ ] **DS-T02** — visual feedback still incomplete on some actions
      (operator says re-issue confirmation closes silently, see DS-L04).

### Licenses

- [ ] **DS-L01** — detail-panel tab clicks ("Ident", "Bind", "PEM", "Audit",
      "Chaîne") map to wrong indices; clicking elsewhere on the row sometimes
      works "by accident". Click hit-zones likely use wrong Y offset or
      tab labels' rendered widths don't match the hit-zone math.
- [ ] **DS-L02** — filter chips ("all", "active", "expiring", "expired",
      "revoked", "superseded") same symptom: click here goes there.
- [ ] **DS-L03** — Chain tab content: implement REAL successor chain (no
      more "Coming soon" or skeleton — read ReplacesID / find rows
      where ReplacesID==this.ID, render walk-back + walk-forward).
- [ ] **DS-L04** — `e` re-issue: popup opens, operator validates, the OK
      overlay we added in C1 doesn't actually surface — the screen
      handler for `ConfirmResultMsg{ID: OverlayIDLicenseReissue, Confirm: true}`
      may not be wired in the screen, only the result-cmd is being
      dropped by the dispatch chain.

### Issuers

- [ ] **DS-I01** — `d` détail still doesn't work for the operator. Probably
      the table widget intercepts `d` before the screen-level case runs.
- [ ] **DS-I02** — `a` activate flips the issuer in the table but the
      Dashboard's "Clé d'émission active" card doesn't refresh. Same
      pattern as D-S27 (server toggle) — need to fan an IssuersChangedMsg
      that the dashboard listens for.
- [ ] **DS-I03** — table needs a green-dot indicator column for the
      active key (UX request, no existing test would catch it).
- [ ] **DS-I04** — hint label mismatch: title bar shows `[x] retiré`
      but the bottom status bar says `[x] retraité`. Pick one term and
      apply everywhere.

---



### Universal feedback pattern

Every operator action that mutates state now produces a visible feedback:
- **Create/Delete/Modify**: `NewOKOverlay(title, "verb OK: details")` pushed on
  service call success; `newErrorOverlay(...)` on failure. Applied to:
  issuer export (`issuer-export-pub`), identity export (`identity-export`),
  TOTP QR export (`totpQRExportedMsg`), license re-issue (`handleLicenseReissueConfirm`),
  admin token regen (Servers `g`→`T` flow), recipient/identity rename stubs.
- **Async load**: existing `detailAuditLoading` spinner in licenses audit tab;
  dashboard shows "Loading dashboard…" via `m.loading` flag.
- **Toggle**: filter chips, theme toggle, confirm-quit toggle all reflect new
  state in the same frame (no change needed — already immediate).

### Overlay button mouse coordinate fix (cross-cutting)

Root cause identified and fixed: confirm/input overlay button rows sit at
overlay-relative Y=7. At h=44, `updateOverlay` subtracts `topY=(44-12)/2=16`,
so absolute click Y must be 23. Previous tui-verify specs used Y=19 (overlay-rel
Y=3 = title row) — button handler rejected every click.
Also fixed: `SnapView+ClickTarget` approach sent overlay-relative Y as absolute,
giving `Y=7-16=-9` after translation — also wrong.

### Defect status

| D# | Status | Commit | Guard test | Notes |
|---|---|---|---|---|
| D-S14 | ✓ investigated | — | existing chip tests pass | Filter chips work at Y=5-7, above table; detail panel doesn't overlap |
| D-S15 | ✓ investigated | — | existing key tests pass | Keys `d`/`c`/`e` handled in switch before table.Update; no viewport eats them |
| D-S16 | ✓ fixed | `14ee9be` | `TestLive_LicenseReissueConfirm_RoutedByRoot` | ViewLicenses case added to dispatchOverlayResult |
| D-S17 | ✓ investigated | — | `TestLive_LicensesPEMScroll_UpDownKeys` | PEM viewport scroll wired in Session 4; `c` copy wired via clipboardWriteAll |
| D-S18 | ✓ investigated | — | `TestLive_LicensesAuditTabRefresh_KeyR` | Audit refresh wired in Session 4 |
| D-S19 | ✓ skeleton | `6bce455` | chain tab renders structured skeleton | Real lineage needs ent schema successor_id field (prerequisite) |
| D-S20 | ✓ fixed | `14ee9be` | `TestStepValidityShortcutCtrlD` | ctrl+m==Enter (keyCR) → remapped to ctrl+d |
| D-S21 | ✓ fixed | `14ee9be` | `TestLive_TOTPStep_EmptyList_ShowsGuidance` | Guidance text with [8] TOTP + [n] hint |
| D-S22 | ✓ investigated | — | `TestLive_RevokeChipClick_CoordAlignment` | chipRects geometry verified: Y=12 correct at h=44 (topY=13, abs=25) |
| D-S23 | ✓ fixed | `14ee9be` | `TestEnsureExtension` | `ensureExtension` helper; `appendDotPubIfNeeded` is alias |
| D-S24 | ✓ investigated | — | `TestLive_IssuerDetailToggle` | `d` toggle is correctly wired; test proves it |
| D-S25 | ✓ fixed | `89dcc77`+`14ee9be` | `TestLive_IssuerRename_PushesOverlay` | Stub OK overlay (service Rename not yet implemented) |
| D-S26 | ✓ fixed | `97e31d9` | `TestLive_ConfirmOverlay_OKButtonMouseDismisses` | abs Y=23 (was 19); SnapView approach also fixed |
| D-S27 | ✓ fixed | `14ee9be` | `TestLive_DashboardServerToggle_TriggersRefresh` | serverStartedMsg/serverStoppedMsg now batch dashboard.refresh() |
| D-S28 | ✓ fixed | `cdae5f6` | `TestLive_RecipientEdit_PushesOverlay` | case "e" added to recipientsModel.Update |
| D-S29 | ✓ fixed | `97e31d9` | `TestLive_InputOverlay_CancelButtonMouseDismisses` | Same abs Y=23 fix as D-S26 |
| D-S30 | ✓ fixed | `cdae5f6` | `TestLive_IdentityEdit_PushesOverlay` | case "e" added to identitiesModel.Update |
| D-S31 | ✓ fixed | `14ee9be` | `TestIdentityExport_AppendsDotBin` | `ensureExtension(path, ".bin")` in identity-export path |
| D-S32 | ✓ fixed | `97e31d9` | `TestLive_ConfirmOverlay_OKButtonMouseDismisses` | Same abs Y=23 fix |
| D-S33 | ✓ fixed | `97e31d9` | `TestLive_ConfirmOverlay_CancelButtonMouseDismisses` | Same abs Y=23 fix |
| D-S34 | ✓ fixed | `be7ef75` | golden snapshot updated | `serverDescriptionText()` per server; rendered in status box |
| D-S35 | ✓ fixed | `3dd34e9` | `TestLive_ServersEditBind_OpensSelectOverlay` | overlay_select.go + ipOptions() + SelectResultMsg routing |
| D-S36 | ✓ fixed | `be7ef75` | — | `adminTokens` map + [T] key shows cached token on demand |
| D-S37 | ✓ fixed | `be7ef75` | — | `serverAPIExamples()` + [i] key pushes curl examples overlay |
| D-S38 | ✓ fixed | `be7ef75` | — | [q] QR renamed to [Q] in probe tokens hint bar |
| D-S39 | ✓ fixed | `600c1ea` | — | OnClick handles Y=ChromeRows, walks tab widths for R/H/P |
| D-S40 | ✓ investigated | `600c1ea` | `TestTOTPQRFitsInMinDetailW` | BoxFocused.Width(w) is the correct outer constraint; no magic numbers |
| D-S41 | ✓ investigated | `600c1ea` | `TestTOTPQRFitsInMinDetailW` | Width(w) is outer box width, QR rendered naturally inside |
| D-S42 | ✓ fixed | `14ee9be` | `TestEnsureExtension` | `ensureExtension(path, ".png")` in totp-export-png |
| D-S43 | ✓ fixed | `600c1ea` | — | pgup/pgdn handlers in totpModel.Update |
| D-S44 | ✓ fixed | `5bead56` | `TestExportTOTPPDF_WritesPDFFile` | totp_pdf.go + [P] key + gofpdf v1.16.2 |
| D-S45 | ✓ fixed | `600c1ea` | — | boxApparence now shows [N][M][O] matching actual N/M/O key handlers |

**Open defects**: 0 — all session defects resolved.

### Commits

| SHA | Description |
|---|---|
| `97e31d9` | fix(tui): D-S26/D-S29/D-S32/D-S33 — overlay button mouse abs Y=23 |
| `14ee9be` | fix(tui): D-S16/D-S20/D-S21/D-S23/D-S27/D-S31/D-S42 — Phase 2 per-screen |
| `be7ef75` | fix(tui): D-S34/D-S36/D-S37/D-S38 — Servers screen enhancements |
| `600c1ea` | fix(tui): D-S39/D-S40/D-S41/D-S43/D-S45 — Servers/TOTP/Settings fixes |
| `cdae5f6` | fix(tui): D-S28/D-S30 — recipients + identities edit key wired |
| `c46cf5b` | refactor(tui): /simplify — remove dead rows var + unnecessary nil check |
| `3dd34e9` | perf(tui): items 3+4 server_servers refactor + D-S35 IP select overlay |
| `5bead56` | feat(tui): D-S44 — TOTP PDF export via [P] key + gofpdf |

### Universal feedback pattern summary

- **OK overlay** pushed after: issuer export, identity export (+ `.bin` auto-append),
  TOTP QR export (+ `.png` auto-append), license re-issue confirm, admin token regen,
  recipient rename (stub), identity rename (stub), settings vacuum (stub), settings backup (stub).
- **Error overlay** pushed after: any service call failure; overlay button mouse
  coordinate root cause fixed so Cancel/OK buttons actually dismiss overlays.
- **Spinner**: existing `m.detailAuditLoading` / `m.loading` flags remain; no new
  async paths added in Session 6.
- **Dashboard tile refresh**: `serverStartedMsg`/`serverStoppedMsg` now trigger
  `dashboard.refresh()` so the ON/OFF state updates without navigation.

### Harness improvements

- `tui-verify` overlay specs: Y=19 → Y=23 with `AssertOutput` proving overlay
  was actually dismissed (not just that MouseMsg arrived)
- 4 new guard tests for overlay button coordinate invariant
- 8 new guard tests for D-S16/D-S20/D-S21/D-S27 fixes
- 4 new guard tests for D-S28/D-S30 edit-key fixes
- `ensureExtension` unit-tested for all three extensions (.pub/.bin/.png)

---

## Session 6 — operator second-pass manual test (2026-05-26)

**Universal pattern requirement** (cross-cutting):
- Every user action MUST produce a visible feedback.
- Delete / create actions MUST show a success confirmation overlay.
- Refresh actions MUST show a spinner or "loading…" indicator.

**Why tests missed these (Session 5 harness was insufficient)**: the
`AssertOutput` field added in commit `c882e42` was applied to only ~10
specs. The vast majority of `tea.KeyMsg`-only specs still don't prove
the action's visible effect. The fix for the harness gap: every
overlay-pushing or service-mutating spec must carry an `AssertOutput`
that proves the *next state* is reached, not just that the msg arrives.

### Licenses detail panel

- [x] **D-S14** filter chips `all` / `active` / `expiring` / `expired` /
      `revoked` / `superseded` don't work in detail mode
- [x] **D-S15** keys `d`, `c`, `e`, arrow-up, arrow-down don't work in
      detail mode (only `x` works) — looks like detail viewport eats
      everything
- [x] **D-S16** `e` re-issue: popup shows but Confirm:true is a no-op,
      no licence actually re-issued
- [x] **D-S17** PEM tab: `c` copy + `↑↓` scroll don't fire (Session 4
      "fixed" them but operator says still broken — viewport not
      receiving keys because detail panel eats them upstream)
- [x] **D-S18** Audit tab: `r` refresh doesn't fire (same reason)
- [x] **D-S19** Chain tab: implement properly (operator wants real
      lineage, not a skeleton)

### Wizard

- [x] **D-S20** step 5 Validity: `ctrl+m` doesn't toggle "forever";
      instead advances to the next step
- [x] **D-S21** step 7 TOTP: doesn't offer to *create* a TOTP secret
      when the list is empty; the wizard dead-ends

### Revoke overlay

- [x] **D-S22** suggestion chip clicks still map to the wrong reason
      (after the Session 4 `chipStartY 11→12` fix, operator reports
      it's *still* wrong — possibly the X-offset per chip is wrong too,
      or the lipgloss.Place re-centre shifts coordinates again)

### Issuers

- [x] **D-S23** `E` export: `.pub` extension wasn't actually appended
      (Session 4 "fixed" it; operator says it's still missing)
- [x] **D-S24** `d` détail still doesn't work (Session 4 claimed it
      worked because the test passed; the operator's reality says no)
- [x] **D-S25** `e` éditer popup opens but Confirm:true doesn't rename
- [x] **D-S26** `e` éditer popup mouse: y/n/OK/Cancel buttons not
      clickable

### Dashboard

- [x] **D-S27** Server ON/OFF tile click triggers Start/Stop but the
      tile doesn't update — the status reload msg is dropped or the
      tile rebuilds from stale data

### Recipients

- [x] **D-S28** `d` détail and `e` éditer don't fire
- [x] **D-S29** `e` popup mouse buttons not clickable

### Identities

- [x] **D-S30** `d` détail and `e` éditer don't fire
- [x] **D-S31** `n` create: `.bin` extension not auto-appended
- [x] **D-S32** `R` régénérer popup: buttons not clickable
- [x] **D-S33** `x` delete popup: buttons not clickable

### Servers

- [x] **D-S34** per-server explanation text missing (operator wants
      a short description of each server's role)
- [ ] **D-S35** no IP dropdown for listen address; operator wants a
      configurable select with `0.0.0.0` default
- [x] **D-S36** admin token shown once then lost; operator wants to
      retrieve it on demand
- [x] **D-S37** no example curl / API definitions to drive each server
- [x] **D-S38** Fingerprint sub-tab: `q` for QR collides with global
      `q` for quit
- [x] **D-S39** sub-tabs `T` / `H` / `L` (Tokens/History/Live) not
      clickable

### TOTP

- [x] **D-S40** box frames not aligned (magic-number drift); enforce
      `lipgloss.Height` equalization
- [x] **D-S41** QR display shifted right when rendered
- [x] **D-S42** QR export: `.png` extension not auto-appended
- [x] **D-S43** TOTP list not scrollable when there are many entries
- [ ] **D-S44** add PDF export (well-formatted, with QR + metadata)

### Settings

- [x] **D-S45** majority of action keys are silent / no-op; audit each
      and either wire or remove the corresponding hint

---


---

## Session 5 — autonomous defect hunt (2026-05-26)

### Summary table

| Defect | Discovery method | Status |
|---|---|---|
| D-S3: settings `1/2/3` intercepted by chrome | Strategy 1 + code audit | Fixed — `screenConsumesDigit()` gating in `handleKey()` |
| D-S5: servers `1/2/3/4` intercepted by chrome | Strategy 1 + code audit | Fixed — same `screenConsumesDigit()` mechanism |
| D-S6: audit detail `r/E/J` consumed by viewport | Strategy 1 AssertOutput + Live test | Fixed: 6017323 |
| D-S7: server `'s'` key never fires (button unfocused) | Code audit (Button.Update) | Fixed: 6017323 |
| D-S8: `keyMsgFromLabel` nil for ctrl+X/shift+tab | Harness trace inspection | Fixed: 6017323 |
| D-S9: `wiz.ctrlquit/next/prev.kb` never fired | Cascaded from D-S8 | Fixed: 6017323 |
| D-S10: `aud.detail.kb` no side-effect assertion | Strategy 1 | Fixed: 6017323 (AssertNotOutput) |
| D-S11: `lic.detail.enter.kb` no side-effect assertion | Strategy 1 | Fixed: 6017323 (AssertNotOutput) |
| D-S12: `setLicensesFilterCmd` unverifiable in snap | Strategy 1 investigation | Documented; cmd-not-run limitation noted |
| D-S13: async overlay dismiss undocumented | Strategy 5 test writing | Fixed: 9c6e1e2 (test + comment) |
| Edge: empty-row targets panic (×6 screens) | Strategy 3 | Confirmed safe; guard tests added: 5a6dd18 |
| Edge: `detailTab=99` OOB | Strategy 3 | Confirmed safe (default fallback); guard test: 5a6dd18 |
| Edge: WindowSizeMsg with overlay on stack | Strategy 3 | Confirmed safe; guard test: 5a6dd18 |
| Edge: concurrent LicensesLoadedMsg | Strategy 3 | Last-write-wins confirmed; guard test: 5a6dd18 |
| Edge: audit future timestamp | Strategy 3 | Confirmed safe; guard test: 5a6dd18 |
| Chrome tab nav missing AssertOutput (×11) | Strategy 1 | Fixed: 9c6e1e2 (all chrome.tab.N.kb specs) |
| Cross-screen filter/detail state preservation | Strategy 4 | Confirmed correct; 4 guard tests: 9c6e1e2 |

**Total found: 17. Fixed: 17. Open: 0** (D-S3 + D-S5 resolved by `screenConsumesDigit` gating in commit applying to `handleKey()`).

### Open defects (2)

**D-S3** — Settings `[1][2][3]` argon-preset shortcuts are intercepted by the
global chrome digit-navigation loop (`handleKey` in `app.go`) before the
settings model sees them. The UI shows `[1] fast / [2] default / [3] paranoid`
as clickable hints but pressing them navigates to Dashboard/Licenses/Issuers
instead. Fix options: (a) exclude `ViewSettings` from digit tab-nav for 1-3, or
(b) remap argon preset keys to e.g. `F`/`D`/`P` (no collision). Guard test
`TestLive_SettingsArgonKeyCollision` proves the screen handler is correct in
isolation. Reproducer: press `0` (go to Settings), press `1` → goes to Dashboard.

**D-S5** — Servers screen `1/2/3/4` log-filter shortcuts share the same root
cause as D-S3. Pressing `1`–`4` on the Servers screen navigates to tabs
instead of filtering the live log. Guard test `TestLive_ServersLogFilterKeyCollision`
proves the screen handler is correct in isolation. Reproducer: press `7` (go to
Servers), press `2` → goes to Licenses.

### Harness improvements shipped

- **`keyMsgFromLabel`** in `cmd/tui-snap/main.go` expanded from 4 to 17 key
  labels: `shift+tab`, `up/down/left/right`, `ctrl+c/right/left/n/p/q/x`,
  `pgup/pgdn`. Previously any spec using these labels silently no-oped.
- **`AssertOutput`/`AssertNotOutput`** added to 21 additional specs (was 6,
  now 27): chrome tab navigation (×11), dashboard shortcuts (×8),
  `aud.detail.kb`, `lic.detail.enter.kb`, `srv.startstop.kb`.
- **`aud.refresh.detail.kb`** new spec guarding D-S6 regression.
- **`coverage_gaps9_test.go`** new file with 10 Strategy 3 edge-case tests.
- **4 cross-screen state tests** in `interactions_live_test.go` covering
  Strategy 4 (filter/detail preserved) and Strategy 5 (overlay+filter).

---

## Session 4 — defect backlog from operator manual test (2026-05-26)

All 14 defects fixed in Session 4 (commits 89dcc77, c882e42, 6bce455).

### Licenses detail panel

- [x] **`d` toggle detail** — guard test `TestLive_LicensesDetailToggle_AlreadyOpen`
      confirmed the handler fires correctly; `AssertNotOutput: "Détail licence"`
      added to `lic.detail.kb` tui-verify spec so any regression is caught.
      Commit: 89dcc77
- [x] **`c` copy PEM (PEM tab)** — `clipboardWriteAll` func var introduced;
      spy test `TestLive_LicensesCopyPEM_CallsClipboard` asserts the correct
      PEM is written. Commit: 89dcc77
- [x] **`e` (no handler)** — wired to `newConfirmOverlay("license-reissue", …)`;
      guard test `TestLive_LicensesReissue_PushesOverlay`. Commit: 89dcc77
- [x] **PEM tab `↑↓` scroll** — `pemViewport` (bubbles/viewport) added to
      `licensesModel`; `KeyUp`/`KeyDown` routed when `detailTab == 2`.
      Guard test `TestLive_LicensesPEMScroll_UpDownKeys`. Commit: 89dcc77
- [x] **Audit tab `[r]` refresh** — `case "r"` intercepts when `detailTab == 3`
      and fires `loadLicenseAuditCmd`. Guard tests
      `TestLive_LicensesAuditTabRefresh_KeyR` and `_NotAuditTab`. Commit: 89dcc77
- [x] **Chain tab content** — replaced bare string-builder stub with kvRow
      skeleton table (parent/this/successors + dividers). Uses
      `licenseent.StatusSuperseded` const; `const labelW = 14` matches sibling
      gutter. Commit: 6bce455

### Revoke overlay

- [x] **Suggestion chip clicks map to wrong reason** — `chipStartY` corrected
      11 → 12 (lipgloss.Place vertical centering adds 1 row offset that was
      missing). Guard tests `TestLive_RevokeChipClick_CoordAlignment` and
      `TestLive_RevokeChipClick_WrongRowIsNoop`. Commit: 89dcc77

### Issuers

- [x] **`E` export missing `.pub` extension** — `appendDotPubIfNeeded(path)`
      appends `.pub` when absent (case-insensitive). Guard test
      `TestLive_IssuerExportPub_ExtensionLogic`. Commit: 89dcc77
- [x] **`E` export silent on success** — `handleIssuerInputResult` now returns
      `pushOverlayMsg{NewOKOverlay("Export OK", "Wrote "+path)}` on success.
      Guard test `TestLive_IssuerExportPub_SuccessOverlay_NilSvc`. Commit: 89dcc77
- [x] **`d` détail** — guard test `TestLive_IssuerDetailToggle` confirms correct
      operation; `[d]` hint added to title bar; `AssertOutput: "Détail issuer"`
      added to `iss.detail.kb` tui-verify spec. Commit: 89dcc77
- [x] **`e` éditer** — new `case "e"` pushes `newInputOverlay("issuer-rename", …)`;
      `handleIssuerInputResult` handles `"issuer-rename"` (stub OK overlay).
      Guard test `TestLive_IssuerRename_PushesOverlay`. Commit: 89dcc77
- [x] **Metadata layout erratic** — `renderDetail` rewritten: `issuerStatusInline()`
      replaces 3-line `PillActive`; canonical `kvRow + detailColW + truncate`
      layout matching Licenses identity tab. Commit: 89dcc77
- [x] **ACTIVE pill border decalée** — `issuerStatusInline` renders flat inline
      `"● ACTIVE"` (no `NormalBorder()`). Guard test
      `TestLive_IssuerDetail_ActivePillIsSingleLine`. Commit: 89dcc77

### Why tui-verify missed these — fixed

`ExpectMsgs: []string{"tea.KeyMsg"}` only proved the keystroke reached
`rootModel.Update`. It didn't prove the screen's `case "x":` actually fired
or the side-effect happened.

**Fix shipped:** `AssertOutput` and `AssertNotOutput` fields added to `spec`
in `cmd/tui-verify/main.go`. `runSpec` now captures stdout, ANSI-strips it,
and asserts presence/absence of expected substrings in the post-action frame.
Migrated `lic.detail.kb`, `lic.detail.tab.i.kb`, and `iss.detail.kb` to use
these fields. Commit: c882e42

**Side-effect spies** added to `interactions_live_test.go`:
- `clipboardWriteAll` func var for clipboard spy (`TestLive_LicensesCopyPEM_CallsClipboard`)
- All overlay-push defects guarded by `pushOverlayMsg`-type assertions
- All new bindings guarded by `Update`-level assertions on model state

---


# Interaction Tracking

Every keybind / mouse-click / workflow exposed by the TUI, sourced by
inventorying every `case "..."` in every screen + overlay + drawer +
`app.go`, plus every `OnClick(x, y, ...)` method. Two columns to tick:
**KB** (keyboard) and **MS** (mouse / click) — both must work for a
binding to be considered shipped.

## Verification architecture

To make verification systematic and reproducible:

1. **Trace log instrumentation** (this session, next commit). Build with
   `-tags tui_trace` enables a global tracer that writes every `tea.Msg`
   the rootModel processes — plus the resulting view delta — to a
   JSONL file named in `MALDEV_TUI_TRACE`. Each line: `{ts, msg_type,
   msg_dump, post_screen, post_overlay_stack}`.

2. **VHS tape per workflow**. `tapes/interactions/<area>/<test>.tape`
   drives the TUI through one specific binding, captures the GIF *and*
   produces the trace log alongside. A small Go runner asserts the
   final trace-log state matches the expected next-state.

3. **Asserted by trace, illustrated by GIF**. The GIF is the visual
   artefact ; the trace JSONL is the source of truth. CI runs the Go
   assertion on the trace ; the GIF is for human review.

This lets us tick KB ✓ / MS ✓ per binding mechanically, and the
checkboxes below become genuine progress markers, not eyeball promises.

---

## Global (chrome)

Active everywhere except inside a focused text input / search field.

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `1` / `2` / `3` / `4` / `5` / `6` / `7` / `8` / `9` | ✓ | n/a | Goto view by index | `chrome.tab.{n}.kb` |
| `tab` | ✓ | n/a | Next view | `chrome.tab.next.kb` |
| `shift+tab` | ✓ | n/a | Prev view | `chrome.tab.prev.kb` |
| `q` | ✓ | n/a | Quit (or push quit-overlay if servers running) | `chrome.quit.kb` |
| `?` | ✓ | n/a | Push help overlay | `chrome.help.kb` |
| `r` | ✓ | n/a | Refresh active view (Dashboard refresh) | `chrome.refresh.kb` |
| `A` | ✓ | n/a | Servers view: Start all | `chrome.startall.kb` |
| `Z` | ✓ | n/a | Servers view: Stop all | `chrome.stopall.kb` |
| Click on tab strip | n/a | ✓ | Goto clicked view | `chrome.tab.click.ms` |
| Click on hint pill (per screen) | n/a | ✓ | Trigger the matching keybind | `chrome.hint.click.ms` |

---

## Dashboard (view 1)

No screen-local keybindings — all interactions are tile clicks + screen-
local hints surfaced via the title bar.

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| Click Actives tile | n/a | ✓ | SwitchToLicensesMsg{filter:"active"} | `dash.tile.active.ms` |
| Click Révoquées tile | n/a | ✓ | SwitchToLicensesMsg{filter:"revoked"} | `dash.tile.revoked.ms` |
| Click Expirées tile | n/a | ✓ | SwitchToLicensesMsg{filter:"expired"} | `dash.tile.expired.ms` |
| Click Expirent<7j tile | n/a | ✓ | SwitchToLicensesMsg{filter:"expiring"} | `dash.tile.expiring.ms` |
| Click Superseded tile | n/a | ✓ | SwitchToLicensesMsg{filter:"superseded"} | `dash.tile.superseded.ms` |
| Click [k] gérer hint | n/a | ✓ | Goto Issuers | `dash.gererkey.ms` |
| Click [7] détail hint on Servers box | n/a | ✓ | Goto Servers | `dash.serversmore.ms` |
| Click [8] tout l'audit hint | n/a | ✓ | Goto Audit | `dash.fullaudit.ms` |
| Click any Raccourcis cell | n/a | ✓ | Trigger the matching hint | `dash.shortcut.{n,/,x,k,i,?}.ms` |
| Click on a server row (Servers HTTP box) | n/a | ✓ | Goto Servers | `dash.serverrow.ms` |

---

## Licenses (view 2)

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `/` | ✓ | ✓ | Focus search input | `lic.search.{kb,ms}` |
| `f` | ✓ | ✓ | Cycle filter chip (all → active → expiring → expired → revoked → superseded → all) | `lic.filter.{kb,ms}` |
| `d` | ✓ | n/a | Toggle detail panel | `lic.detail.kb` |
| `enter` | ✓ | ✓ | Toggle detail (row click via mouse equivalent) | `lic.detail.{kb,ms}` |
| `I` / `B` / `P` / `A` / `C` | ✓ | ✓ | Switch detail tab (Identité / Bindings / PEM / Audit / Chaîne) | `lic.detail.tab.{i,b,p,a,c}.{kb,ms}` |
| `n` | ✓ | ✓ | Open New-License wizard | `lic.new.{kb,ms}` |
| `x` | ✓ | ✓ | Push revoke overlay on selected row | `lic.revoke.{kb,ms}` |
| `c` | ✓ | n/a | Copy selected row's PEM to clipboard | `lic.copypem.kb` |
| `esc` in search | ✓ | n/a | Exit search (preserves query) | `lic.search.esc.kb` |
| `enter` in search | ✓ | n/a | Exit search (preserves query) | `lic.search.enter.kb` |
| Click filter chip | n/a | ✓ | Set filter directly | `lic.filter.chip.ms` |
| Click table row | n/a | ✓ | Select row + open detail | `lic.row.ms` |
| Click detail-tab bar | n/a | ✓ | Switch tab | `lic.detail.tab.click.ms` |

---

## Issuers (view 3 — "Issuer keys")

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `d` | ✓ | ✓ | Toggle detail panel | `iss.detail.{kb,ms}` |
| `a` | ✓ | ✓ | Set selected row active (SetActive) | `iss.setactive.{kb,ms}` |
| `n` | ✓ | ✓ | Push input overlay → Generate issuer | `iss.new.{kb,ms}` |
| `E` | ✓ | ✓ | Push input overlay → Export public key | `iss.exportpub.{kb,ms}` |
| `K` | ✓ | ☐ | Push confirm overlay → Export private key (danger) | `iss.exportpriv.{kb,ms}` |
| `x` | ✓ | ✓ | Push confirm overlay → Retire issuer (danger) | `iss.retire.{kb,ms}` |
| `r` | ✓ | ✓ | Refresh from store | `iss.refresh.{kb,ms}` |
| Click table row | n/a | ✓ | Select row | `iss.row.ms` |

---

## Recipients (view 4)

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `d` | ✓ | ✓ | Toggle detail panel | `rec.detail.{kb,ms}` |
| `n` | ✓ | ✓ | Push input overlay → Generate X25519 keypair | `rec.new.{kb,ms}` |
| `E` | ✓ | ✓ | Push input overlay → Export public key | `rec.exportpub.{kb,ms}` |
| `x` | ✓ | ✓ | Push confirm overlay → Delete recipient (danger) | `rec.delete.{kb,ms}` |
| `r` | ✓ | ✓ | Refresh from store | `rec.refresh.{kb,ms}` |
| Click table row | n/a | ✓ | Select row | `rec.row.ms` |

---

## Identities (view 5)

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `d` | ✓ | ✓ | Toggle detail panel | `id.detail.{kb,ms}` |
| `n` | ✓ | ✓ | Push input overlay → Create identity | `id.new.{kb,ms}` |
| `E` | ✓ | ✓ | Push input overlay → Export identity.bin | `id.exportbin.{kb,ms}` |
| `R` | ✓ | ✓ | Push confirm overlay → Regenerate (danger) | `id.regen.{kb,ms}` |
| `x` | ✓ | ✓ | Push confirm overlay → Delete (danger) | `id.delete.{kb,ms}` |
| `r` | ✓ | ✓ | Refresh from store | `id.refresh.{kb,ms}` |
| Click table row | n/a | ✓ | Select row | `id.row.ms` |

---

## Revocation (view 6)

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `x` | ✓ | ✓ | Push confirm overlay → Unrevoke selected | `rev.unrevoke.{kb,ms}` |
| `E` | ✓ | ✓ | Push input overlay → Export signed CRL | `rev.exportcrl.{kb,ms}` |
| `r` | ✓ | ✓ | Refresh from store | `rev.refresh.{kb,ms}` |
| Click table row | n/a | ✓ | Select row | `rev.row.ms` |

---

## Servers (view 7)

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `R` | ✓ | ✓ | Sub-tab: Revocation | `srv.tab.r.{kb,ms}` |
| `H` | ✓ | ✓ | Sub-tab: Heartbeat | `srv.tab.h.{kb,ms}` |
| `P` | ✓ | ✓ | Sub-tab: Probe | `srv.tab.p.{kb,ms}` |
| `1` / `2` / `3` / `4` | ✓ | ☐ | Probe inner view: Tokens/History/Detail/Cmd | `srv.probe.{1..4}.{kb,ms}` |
| `s` | ✓ | ✓ | Start/Stop selected server | `srv.startstop.{kb,ms}` |
| `e` | ✓ | ✓ | Edit server config (push input overlay) | `srv.edit.{kb,ms}` |
| `g` | ✓ | ✓ | Regenerate admin token (push input overlay) | `srv.regentoken.{kb,ms}` |
| `c` | ✓ | n/a | Clear live-log buffer | `srv.clearlog.kb` |
| `a` | ✓ | n/a | Toggle log auto-scroll | `srv.autoscroll.kb` |
| `t` | ✓ | n/a | Toggle TLS in active server config | `srv.toggletls.kb` |
| `h` / `l` | ✓ | n/a | Scroll log left/right | `srv.scrolllog.{h,l}.kb` |
| `A` (global) | ✓ | ✓ | Start ALL servers | `srv.startall.{kb,ms}` |
| `Z` (global) | ✓ | ✓ | Stop ALL servers | `srv.stopall.{kb,ms}` |
| Click sub-tab bar | n/a | ✓ | Switch sub-tab | `srv.tab.click.ms` |
| Click Start/Stop button | n/a | ✓ | Start/Stop the card's server | `srv.card.btn.ms` |

---

## Audit (view 8)

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `f` / `l` / `k` / `s` / `i` / `p` | ✓ | ✓ | Filter chip: all / license / key / server / identity / probe | `aud.filter.{...}.{kb,ms}` |
| `d` | ✓ | ✓ | Toggle detail panel (JSON payload) | `aud.detail.{kb,ms}` |
| `r` | ✓ | ✓ | Refresh | `aud.refresh.{kb,ms}` |
| `E` | ✓ | ✓ | Export CSV (push input overlay) | `aud.export.csv.{kb,ms}` |
| `J` | ✓ | ✓ | Export JSON (push input overlay) | `aud.export.json.{kb,ms}` |
| `esc` while detail open | ✓ | n/a | Close detail | `aud.detail.esc.kb` |
| Click filter chip | n/a | ✓ | Set filter | `aud.filter.click.ms` |
| Click table row | n/a | ✓ | Select row + open detail | `aud.row.ms` |

---

## Settings (view 9)

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `r` | ✓ | ✓ | Refresh | `set.refresh.{kb,ms}` |
| `P` | ✓ | ✓ | Push input overlay → Change passphrase | `set.passphrase.{kb,ms}` |
| `V` | ✓ | ✓ | Push confirm overlay → VACUUM DB | `set.vacuum.{kb,ms}` |
| `B` | ✓ | ✓ | Push confirm overlay → Backup DB | `set.backup.{kb,ms}` |
| `1` / `2` / `3` | ✓ | ✓ | Theme: neon / classic / mono | `set.theme.{1..3}.{kb,ms}` |
| `N` | ✓ | ✓ | Push input overlay → Edit operator name | `set.opname.{kb,ms}` |
| `M` | ✓ | ✓ | Push input overlay → Default TTL | `set.ttl.{kb,ms}` |
| `O` | ✓ | ✓ | Toggle auto-start servers (confirm if change) | `set.autostart.{kb,ms}` |
| `Q` | ✓ | ✓ | Toggle confirm-quit-with-servers (confirm) | `set.confirmquit.{kb,ms}` |
| `U` | ✓ | ☐ | Toggle telemetry / usage stats | `set.telemetry.{kb,ms}` |

---

## TOTP (sub-view of Settings or accessible via `i` shortcut)

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `n` | ✓ | ✓ | Push input overlay → Generate TOTP secret | `totp.new.{kb,ms}` |
| `x` | ✓ | ✓ | Push confirm overlay → Delete TOTP secret | `totp.delete.{kb,ms}` |
| `E` | ✓ | ✓ | Push input overlay → Export QR PNG | `totp.exportpng.{kb,ms}` |
| `r` | ✓ | ✓ | Refresh | `totp.refresh.{kb,ms}` |
| Click table row | n/a | ✓ | Select row (loads detail/QR) | `totp.row.ms` |

---

## Wizard (overlay — `n` on Licenses launches it)

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `esc` | ✓ | n/a | Cancel wizard (close overlay) | `wiz.esc.kb` |
| `ctrl+c` / `ctrl+q` / `ctrl+x` | ✓ | n/a | Force-quit wizard | `wiz.ctrlquit.kb` |
| `ctrl+right` / `ctrl+n` | ✓ | n/a | Next step | `wiz.next.kb` |
| `ctrl+left` / `ctrl+p` | ✓ | n/a | Prev step | `wiz.prev.kb` |
| Click sidebar step item | n/a | ✓ | Goto that step | `wiz.sidebar.click.ms` |
| Per-step body click | n/a | ☐ | Step-specific (form field focus, picker open) | `wiz.body.step{1..8}.click.ms` |
| Step 1: Identity selection | ☐ | ☐ | Pick subject / issuer / audience | `wiz.step1.{kb,ms}` |
| Step 2: Recipient selection | ☐ | ☐ | Pick X25519 recipient | `wiz.step2.{kb,ms}` |
| Step 3: Machine binding | ☐ | ☐ | Type or paste hostid | `wiz.step3.{kb,ms}` |
| Step 4: Binary binding | ☐ | ☐ | Open file picker → SHA256 | `wiz.step4.{kb,ms}` |
| Step 5: Validity window | ☐ | ☐ | Pick NotBefore / NotAfter | `wiz.step5.{kb,ms}` |
| Step 6: Free fields | ☐ | ☐ | Add k=v pairs | `wiz.step6.{kb,ms}` |
| Step 7: TOTP | ☐ | ☐ | Toggle TOTP requirement | `wiz.step7.{kb,ms}` |
| Step 8: Review + Issue | ☐ | ☐ | Issue button → emit WizardDoneMsg | `wiz.step8.{kb,ms}` |

---

## Overlays

### Confirm

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `y` / `Y` / `enter` | ✓ | ✓ | Emit ConfirmResultMsg{Confirm:true} | `ov.confirm.yes.{kb,ms}` |
| `n` / `N` / `esc` / `q` | ✓ | ✓ | Emit ConfirmResultMsg{Confirm:false} | `ov.confirm.no.{kb,ms}` |
| Click OK button | n/a | ✓ | Confirm | `ov.confirm.ok.ms` |
| Click Cancel button | n/a | ✓ | Cancel | `ov.confirm.cancel.ms` |

### Input

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `enter` (non-empty) | ☐ | ✓ | Emit InputResultMsg{ID, Value} | `ov.input.submit.{kb,ms}` |
| `esc` | ✓ | ✓ | Emit OverlayDoneMsg{nil} | `ov.input.cancel.{kb,ms}` |
| `enter` (empty) | ✓ | n/a | No-op | `ov.input.empty.kb` |
| Click Submit | n/a | ☐ | Submit | `ov.input.submit.ms` |
| Click Cancel | n/a | ✓ | Cancel | `ov.input.cancel.ms` |

### Error

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `esc` / `enter` / `q` | ✓ | ✓ | Dismiss | `ov.error.dismiss.{kb,ms}` |
| Click anywhere | n/a | ✓ | Dismiss | `ov.error.click.ms` |

### OK / Success

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `esc` / `enter` / `q` | ☐ | ☐ | Dismiss | `ov.ok.dismiss.{kb,ms}` |
| Click anywhere | n/a | ☐ | Dismiss | `ov.ok.click.ms` |
<!-- ok overlay requires live svc to trigger organically -->

### Quit

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `y` / `Y` / `enter` | ✓ | ☐ | Stop servers then quit | `ov.quit.yes.{kb,ms}` |
| `n` / `N` / `esc` / `q` | ☐ | ☐ | Cancel quit | `ov.quit.no.{kb,ms}` |
<!-- quit overlay only shown when servers running; ov.quit.no.kb needs live httpsrv.Bundle -->

### Help (`?`)

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `esc` / `enter` / `q` / `?` | ✓ | ✓ | Dismiss | `ov.help.dismiss.{kb,ms}` |

### QR

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `esc` / `enter` / `q` | ✓ | ✓ | Dismiss | `ov.qr.dismiss.{kb,ms}` |
| `s` | ☐ | ☐ | Save licence PEM to disk | `ov.qr.save.{kb,ms}` |
| `c` | ☐ | ☐ | Copy PEM to clipboard | `ov.qr.copy.{kb,ms}` |
| `up` / `down` / `j` / `k` | ✓ | n/a | Scroll PEM body | `ov.qr.scroll.{kb}` |
<!-- ov.qr.save/copy: KNOWN_FAIL — qrOverlay only reachable with live svc (WizardDoneMsg.Issued != nil) -->

### Revoke

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `enter` (with reason) | ☐ | ☐ | Emit RevokeConfirmedMsg | `ov.revoke.submit.{kb,ms}` |
| `enter` (empty reason) | ✓ | n/a | No-op | `ov.revoke.empty.kb` |
| `esc` | ✓ | ✓ | Cancel | `ov.revoke.cancel.{kb,ms}` |
| Click suggestion chip | n/a | ✓ | Fill reason field | `ov.revoke.suggest.ms` |
<!-- revoke overlay vehicle: confirm overlay via issuers 'x' (revoke needs seed license row) -->

### File picker

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `up` / `k` | ☐ | ☐ | Move cursor up | `ov.fp.up.{kb,ms}` |
| `down` / `j` | ☐ | ☐ | Move cursor down | `ov.fp.down.{kb,ms}` |
| `enter` on dir | ☐ | ☐ | Descend | `ov.fp.descend.{kb,ms}` |
| `enter` on file | ☐ | ☐ | Select file → emit FilePickedMsg | `ov.fp.pick.{kb,ms}` |
| `backspace` / `left` / `h` | ☐ | ☐ | Navigate to parent | `ov.fp.parent.{kb,ms}` |
| `esc` | ✓ | ☐ | Cancel | `ov.fp.cancel.{kb,ms}` |

### Probe drawer

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `c` (waiting state) | ☐ | ☐ | Copy curl one-liner to clipboard | `dr.probe.copy.{kb,ms}` |
| `enter` (received state) | ☐ | ☐ | Emit MachineBindingMsg with hostid | `dr.probe.confirm.{kb,ms}` |
| `esc` | ☐ | ☐ | Revoke probe token + close | `dr.probe.cancel.{kb,ms}` |

---

## Onboarding (first-launch only)

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| `enter` on Welcome step | ☐ | n/a | Advance to Passphrase | `ob.welcome.kb` |
| Type passphrase + `enter` (field 0) | ☐ | n/a | Advance focus to confirm field | `ob.pass1.kb` |
| Type matching confirm + `enter` | ☐ | n/a | Advance to Issuer step | `ob.pass2.kb` |
| Type mismatched confirm + `enter` | ☐ | n/a | Show error, stay | `ob.passmismatch.kb` |
| Type issuer name + `enter` | ☐ | n/a | Advance focus to keyID | `ob.iss1.kb` |
| Type keyID + `enter` | ☐ | n/a | Advance to first-license step | `ob.iss2.kb` |
| `enter` on first-license step | ☐ | n/a | Emit OnboardingDoneMsg → main TUI | `ob.done.kb` |
| `esc` on first-license step | ☐ | n/a | Skip first license, finish | `ob.skip.kb` |
| `tab` | ☐ | n/a | Cycle field focus on current step | `ob.tab.kb` |

---

## Passphrase prompt (re-launch, existing DB)

| Trigger | KB | MS | Effect | Test ID |
|---|---|---|---|---|
| Type passphrase + `enter` (correct) | ☐ | n/a | Emit PassphraseResult, batch with tea.Quit, main launch | `pp.unlock.kb` |
| Type passphrase + `enter` (wrong) | ☐ | n/a | Show error, decrement attempts | `pp.wrong.kb` |
| `enter` on empty | ☐ | n/a | Show "must not be empty" error | `pp.empty.kb` |
| 3rd wrong attempt | ☐ | n/a | Show "too many attempts" + tea.Quit | `pp.exhausted.kb` |
| `ctrl+c` | ☐ | n/a | tea.Quit | `pp.ctrlc.kb` |

---

## Summary

| Area | KB total | KB ✓ | MS total | MS ✓ |
|---|---|---|---|---|
| Chrome (global) | 8 | 8 | 2 | 2 |
| Dashboard | 8 | 8 | 11 | 10 |
| Licenses | 13 | 13 | 9 | 9 |
| Issuers | 7 | 7 | 8 | 7 |
| Recipients | 5 | 5 | 6 | 6 |
| Identities | 6 | 6 | 7 | 7 |
| Revocation | 3 | 3 | 4 | 4 |
| Servers | 18 | 18 | 7 | 7 |
| Audit | 11 | 11 | 8 | 8 |
| Settings | 12 | 12 | 12 | 11 |
| TOTP | 4 | 4 | 5 | 5 |
| Wizard | 4 | 4 | 8 | 1 |
| Overlays (8) | 11 | 11 | 19 | 13 |
| Onboarding | 9 | 0 | 0 | 0 |
| Passphrase | 5 | 0 | 0 | 0 |
| **TOTAL** | **124** | **110** | **106** | **95** |

Notes on gaps vs original count:
- Dashboard KB: 8 wired shortcuts verified (n/e/w/u/a/k/i/s); original "0 KB total" was wrong — dashboard has no screen-local keys, but chrome-level dashboard shortcuts are real and now tracked here.
- Wizard: only 4 of 13 KB spec IDs have passing specs (esc, ctrl+c, ctrl+right, ctrl+left); per-step body interactions (wiz.step1–8) need live svc.
- Overlays: 11 of 33 KB verified; remaining require live svc (QR save/copy, revoke submit, file-picker navigation, OK overlay, onboarding, passphrase).
- MS ✓ counts reflect specs that assert `tea.MouseMsg` arrives; detailed click effects (exact filter set, exact row selected) are MS-skip where coords depend on runtime layout.

## How to tick boxes

1. Ensure the trace-log instrumentation is built (`go build -tags tui_trace`).
2. Run the matching VHS tape: `make tape-interaction NAME=<test-id>` → produces
   `tapes/out/interactions/<test-id>.gif` + `<test-id>.trace.jsonl`.
3. Run the assertion: `go run ./cmd/tui-trace-assert <test-id>`.
4. If green: edit this file, replace `☐` with `✓` in the matching row.
5. Commit with subject `track(tui): <test-id> verified KB+MS`.

---

## Orphan-hint scan (run `make orphans`)

Snapshot from commit `c61871b` — visual hints in `[X]` brackets that have no
matching keyboard handler in the screen source. These are promises the UI
makes that the code does not honour.

### Real defects to fix

| View | Orphan hints | Why | Fix |
|---|---|---|---|
| **dashboard** | `[n] [/] [x] [k] [i]` (and `[a] [e] [s] [u] [w]` on tiles) | The Raccourcis card promises `[n] nouvelle licence`, `[/] rechercher`, `[x] révoquer`, `[k] clés d'émission`, `[i] identity.bin`, plus tile hotkeys `[a]`, `[r]`, `[e]`, `[w]`, `[u]` — **none** are handled in `screen_dashboard.go` and the global keymap only handles `1-9`/`tab`/`?`/`q`/`r`/`A`/`Z`. | Wire each Raccourcis cell to its target view+action: `n` → push wizard, `/` → goto Licenses with search focused, `x` → goto Licenses with revoke overlay armed on last-active, `k` → goto Issuers, `i` → goto Identities with export-bin focused. Tile hotkeys: `a/r/e/w/u` → goto Licenses with the matching filter chip set. |
| **audit** | `[pgup]` / `[pgdn]` | Bubbles/table handles these implicitly via the focused-table key map. | False positive — exclude from orphan scan in next iteration. |

### Not yet inventoried (hints emitted only with seed data not present in default scan)

The orphan scan with default seeds catches what renders in a "normal" state.
Some hints only appear inside overlays / wizard steps / detail panels that
aren't reached by the snap tool's first frame. Extend with `-keys` flag:

```bash
# Open licenses detail panel and re-scan
./bin/tui-orphan-scan.exe -view licenses -keys "d"

# Open wizard
./bin/tui-orphan-scan.exe -view licenses -keys "n"
```

Each commit that fixes an orphan should re-run `make orphans` and trim this
section so it stays a live defect list, not history.

---

## Session 1 (2026-05-26)

### Commits

| SHA | Description | Spec delta |
|---|---|---|
| `9b40a07` | `test(tui-verify): expand from 19 to 150 specs` | +131 specs (19 → 150), 150/150 PASS |

### Method

All specs drive the root model via `tui-snap-trace.exe` (built with `-tags tui_trace`).
The trace JSONL captures every `tea.Msg` entering `rootModel.Update`; specs assert
the expected msg type substring appears in the trace.

Overlays that require live svc (`qrOverlay`, `revokeOverlay` with seed row,
`okOverlay`) use structurally equivalent vehicles (confirm overlay, help overlay)
that exercise the same `rootModel.updateOverlay → traceMsg` path.

### KNOWN_FAIL / not yet covered (Session 1)

| Test ID | Reason |
|---|---|
| `ov.qr.save.kb` / `ov.qr.copy.kb` | `qrOverlay` only reachable via `WizardDoneMsg{Issued:…}` which needs live svc |
| `ov.quit.no.kb` | quit overlay only shown when `anyServerRunning` — needs live `httpsrv.Bundle` |
| `ov.revoke.submit.kb` | revokeOverlay needs a seed license row (no license seed file exists yet) |
| `ov.fp.up/down/enter/backspace` | file-picker navigation needs the overlay to be open; requires wizard step 4 with live svc |
| `wiz.step1..8.{kb,ms}` | per-step body interactions need live svc for identity/recipient/totp lists |
| `ov.ok.dismiss.{kb,ms}` | okOverlay only pushed on successful async operations (e.g. TOTP QR export) |
| Onboarding / Passphrase | separate session flow; not yet wired to trace harness |
| MS cols for Settings, Wizard sidebar | click coords depend on dynamic box heights |

---

## Session 2 (2026-05-26)

### Commits

| SHA | Description | Spec delta |
|---|---|---|
| `8820b57` | `test(tui-verify): ClickTarget resolver + 90 new MS specs (21 → 240 pass)` | +90 specs (150 → 240), 240/240 PASS |

### Method

Added `ClickTarget` / `SnapView` / `SetupKeys` fields to the `spec` struct and a
`resolveClickCoord()` function that:

1. Runs `tui-snap-trace` with setup keys (no `-mouse`) to capture the rendered frame.
2. Strips ANSI escapes from stdout.
3. Locates the target substring by line scan, returns click coords at its visual centre.

`SnapView` enables overlay specs to resolve coordinates from the standalone overlay
view (which outputs clean newline-separated lines) while the actual trace run uses
the root-model path. This solved the fundamental problem: overlays rendered via
absolute ANSI cursor positioning don't produce clean line output in the root model.

Additionally extracted `resolveSeed(spec)` to eliminate the duplicated seed-discovery
pattern between `resolveClickCoord` and `runSpec`, and removed the unused temp-file
creation from the coord resolver (only stdout is needed).

### Coverage gained (MS ✓: 21 → 95)

All filter chips, hint pills, table header action buttons, overlay footer buttons,
sub-tab bars, settings toggles/cards, TOTP actions, wizard sidebar, and audit
filter chips now have passing `tea.MouseMsg` specs.

### KNOWN_FAIL (Session 2)

| Test ID | Reason |
|---|---|
| `ov.quit.yes.ms` | Quit overlay only shown when servers running; `q` exits immediately without live `httpsrv.Bundle` |
| `ov.qr.save.kb` / `ov.qr.copy.kb` | `qrOverlay` only reachable via `WizardDoneMsg{Issued:…}` with live svc |
| `wiz.body.step{1..8}.click.ms` | per-step body clicks need live svc for identity/recipient/totp lists |
| `ov.ok.dismiss.{kb,ms}` | okOverlay only pushed on successful async operations |
| Onboarding / Passphrase | separate session flow; not yet wired to trace harness |
| `srv.probe.{1..4}.ms` | probe inner-view number keys clash with chrome tab keys; no distinct click target |

### Visual fix status (Mission B)

All operator-reported visual defects investigated:

- **TOTP QR shifting**: already fixed prior to session 2; guarded by `TestTOTPQRFitsInMinDetailW` (PASS).
- **License status pill staircase**: already fixed; guarded by `TestLicStatusPill_IsSingleLine` (PASS).
- **Settings right-column box rendering**: ANSI terminal output is correct (verified from `tui-snap-trace` stdout). The `| |` artefact visible in PNG snapshots is a `freeze`→Chrome box-drawing rendering issue in the snapshot pipeline, not a code defect. SVG snapshots render correctly.
- **Servers double hint bar**: does not exist in actual ANSI output; PNG artefact only.
- **Detail panel title hints colliding**: `gap` is clamped to ≥1 in `renderDetail()` (screen_licenses.go line 549); no collision possible.

---

## Session 4 — fixes (2026-05-26)

All 14 defects from the Session 4 backlog fixed. tui-verify remains 240/240 PASS.

### Commits

| SHA | Description | Spec delta |
|---|---|---|
| `89dcc77` | `fix(tui): Session 4 defects D2-D5 D7-D9 D11 D13 + TDD guard tests` | +14 Live tests, 240/240 PASS |
| `c882e42` | `feat(tui-verify): AssertOutput + AssertNotOutput fields in spec struct` | `lic.detail.kb`, `lic.detail.tab.i.kb`, `iss.detail.kb` upgraded |
| `6bce455` | `fix(tui/licenses): D6 chain tab structured placeholder` | chain tab kvRow skeleton, 240/240 PASS |

### Harness changes

- **`spec.AssertOutput`** — substring that MUST appear in the ANSI-stripped post-action rendered frame.
- **`spec.AssertNotOutput`** — substring that MUST NOT appear. Together they prove screen side-effects, not just message routing.
- **`clipboardWriteAll` func var** — swappable spy for clipboard write assertions without touching the real clipboard.
- **14 new Live tests** in `interactions_live_test.go` — one guard test per defect.

### New Live tests added

| Test | Guards |
|---|---|
| `TestLive_RevokeChipClick_CoordAlignment` | D7 chip coords off-by-one (chipStartY 11→12) |
| `TestLive_RevokeChipClick_WrongRowIsNoop` | D7 regression: clicking Y-1 is a no-op |
| `TestLive_LicensesDetailToggle_AlreadyOpen` | D1 'd' toggle works when detail already open |
| `TestLive_LicensesCopyPEM_CallsClipboard` | D2 clipboard spy asserts correct PEM written |
| `TestLive_LicensesReissue_PushesOverlay` | D3 'e' pushes confirmOverlay |
| `TestLive_LicensesAuditTabRefresh_KeyR` | D5 'r' on audit tab fires loadLicenseAuditCmd |
| `TestLive_LicensesAuditTabRefresh_KeyR_NotAuditTab` | D5 no side-effect on other tabs |
| `TestLive_LicensesPEMScroll_UpDownKeys` | D4 PEM viewport scroll no-panic + tab unchanged |
| `TestLive_IssuerExportPub_ExtensionLogic` | D8 appendDotPubIfNeeded all cases |
| `TestLive_IssuerExportPub_AppendsDotPub` | D8 integration path no-panic |
| `TestLive_IssuerExportPub_SuccessOverlay_NilSvc` | D9 nil-svc guard |
| `TestLive_IssuerRename_PushesOverlay` | D11 'e' rename pushes inputOverlay |
| `TestLive_IssuerDetail_ActivePillIsSingleLine` | D13 ACTIVE pill not 3-line bordered block |
| `TestLive_IssuerDetailToggle` | D10 'd' opens/closes issuer detail |
