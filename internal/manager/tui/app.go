// Package tui implements the bubbletea-based terminal UI for license-manager.
// It covers the full operator workflow: onboarding, passphrase unlock, dashboard,
// and all operator views including the live Servers screen.
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/tui/widgets"
)

// eventRingCap is the maximum number of httpsrv events retained in the root
// ring buffer so the Servers log retains history across screen switches.
const eventRingCap = 500

// SessionState describes which top-level flow the TUI enters on start.
type SessionState int

const (
	// SessionLocked means the DB exists but the passphrase cascade did not resolve.
	SessionLocked SessionState = iota
	// SessionOnboarding means no DB exists yet; the wizard runs first.
	SessionOnboarding
	// SessionReady means the KEK has been verified and services are live.
	SessionReady
)

// ViewID names the nine navigable views in the tab strip.
type ViewID string

const (
	ViewDashboard  ViewID = "dashboard"
	ViewLicenses   ViewID = "licenses"
	ViewIssuers    ViewID = "issuers"
	ViewRecipients ViewID = "recipients"
	ViewIdentities ViewID = "identities"
	ViewRevocation ViewID = "revocation"
	ViewServers    ViewID = "servers"
	ViewAudit      ViewID = "audit"
	ViewSettings   ViewID = "settings"
)

// viewOrder maps 1-based tab position to ViewID for keyboard navigation.
// Populated by chrome.go's init() from tabDefs so the two stay in sync.
var viewOrder []ViewID

// nextView returns the ViewID dir steps from cur in viewOrder, wrapping around.
// dir is +1 (Tab) or -1 (Shift-Tab). Falls back to cur when viewOrder is empty.
func nextView(cur ViewID, dir int) ViewID {
	if len(viewOrder) == 0 {
		return cur
	}
	idx := 0
	for i, v := range viewOrder {
		if v == cur {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(viewOrder)) % len(viewOrder)
	return viewOrder[idx]
}

// rootModel is the top-level bubbletea model. It owns the session state,
// overlay stack, and all per-view sub-models.
type rootModel struct {
	session  SessionState
	active   ViewID
	overlays []Overlay
	width    int
	hgt      int
	keys     KeyMap

	services *service.Services
	httpsrv  *httpsrv.Bundle // nil until Phase 4

	// rootWidget is non-nil for screens that have been retrofitted to the
	// widget system. The mouse dispatcher uses it for hit-testing.
	rootWidget Widget

	// onboardingResult holds the completed OnboardingDoneMsg so main.go can
	// read it after the tea.Program exits. Populated before tea.Quit is sent.
	onboardingResult *OnboardingDoneMsg

	// Phase 1 screens
	passphrase passphraseModel
	onboarding onboardingModel
	dashboard  dashboardModel

	// Phase 2 screens
	licenses   licensesModel
	issuers    issuersModel
	recipients recipientsModel
	identities identitiesModel
	revocation revocationModel
	audit      auditModel
	settings   settingsModel

	// Phase 4 — live Servers screen.
	servers serversModel

	// eventRing retains the last eventRingCap httpsrv events so the Servers
	// log has history when the operator navigates away and back.
	eventRing    []httpsrv.Event
	eventRingIdx int // next write position (modulo cap)
}

// New constructs the root model ready to be handed to tea.NewProgram.
//
//   - services != nil + sess==SessionReady  → goes straight to Dashboard
//   - services == nil + sess==SessionLocked → passphrase prompt
//   - services == nil + sess==SessionOnboarding → onboarding wizard
func New(services *service.Services, bundle *httpsrv.Bundle, sess SessionState) rootModel {
	m := rootModel{
		session:    sess,
		active:     ViewDashboard,
		services:   services,
		httpsrv:    bundle,
		keys:       newKeyMap(),
		passphrase: newPassphraseModel(),
		onboarding: newOnboardingModel(),
		dashboard:  newDashboardModel(services, bundle),

		licenses:   newLicensesModel(services),
		issuers:    newIssuersModel(services),
		recipients: newRecipientsModel(services),
		identities: newIdentitiesModel(services),
		revocation: newRevocationModel(services),
		audit:      newAuditModel(services),
		settings:   newSettingsModel(services),

		servers:   newServersModel(bundleAsController(bundle)),
		eventRing: make([]httpsrv.Event, 0, eventRingCap),
	}
	return m
}

// RootModel is the interface main.go uses to extract post-run state from the
// rootModel without importing the unexported concrete type.
type RootModel interface {
	tea.Model
	// OnboardingResult returns the wizard payload after a SessionOnboarding run,
	// or nil if the operator quit before completing.
	OnboardingResult() *OnboardingDoneMsg
}

// OnboardingResult returns the OnboardingDoneMsg collected during the wizard,
// or nil if onboarding did not complete. main.go calls this on the tea.Model
// returned by tea.Program.Run() after a SessionOnboarding run.
func (m rootModel) OnboardingResult() *OnboardingDoneMsg {
	return m.onboardingResult
}

// bundleAsController converts a *Bundle to a Controller interface, returning
// nil (typed nil interface) when bundle itself is nil. This avoids the
// classic Go footgun where assigning a nil *Bundle to an interface variable
// produces a non-nil interface value that panics on method dispatch.
func bundleAsController(b *httpsrv.Bundle) httpsrv.Controller {
	if b == nil {
		return nil
	}
	return b
}

// listenServerEvents returns a tea.Cmd that reads one event from the merged
// channel and wraps it as serverEventMsg. The program re-issues this command
// after each delivery, forming a self-perpetuating listener that exits cleanly
// when the channel is closed. Nil bundle → returns nil (no-op).
func listenServerEvents(bundle *httpsrv.Bundle) tea.Cmd {
	if bundle == nil {
		return nil
	}
	ch := bundle.MergedEvents()
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil // channel closed — program is shutting down
		}
		return serverEventMsg{ev: ev}
	}
}

func (m rootModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	switch m.session {
	case SessionReady:
		cmds = append(cmds, m.dashboard.refresh())
	case SessionLocked:
		cmds = append(cmds, m.passphrase.Init())
	case SessionOnboarding:
		cmds = append(cmds, m.onboarding.Init())
	}
	// Start fan-in listener regardless of session state so events accumulate
	// in the ring buffer as soon as the program starts.
	if m.httpsrv != nil {
		cmds = append(cmds, listenServerEvents(m.httpsrv))
	}
	return tea.Batch(cmds...)
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Overlay stack takes priority.
	if len(m.overlays) > 0 {
		return m.updateOverlay(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.hgt = msg.Height
		m.dashboard, _ = m.dashboard.Update(msg)
		m.passphrase, _ = m.passphrase.update(msg)
		m.onboarding, _ = m.onboarding.update(msg)
		m.licenses, _ = m.licenses.Update(msg)
		m.issuers, _ = m.issuers.Update(msg)
		m.recipients, _ = m.recipients.Update(msg)
		m.identities, _ = m.identities.Update(msg)
		m.revocation, _ = m.revocation.Update(msg)
		m.audit, _ = m.audit.Update(msg)
		m.settings, _ = m.settings.Update(msg)
		m.servers, _ = m.servers.Update(msg)
		return m, nil

	case serverEventMsg:
		// Store in root ring buffer so log history survives screen switches.
		m.appendEventRing(msg.ev)
		// Forward to the Servers screen (updates cards + log).
		updated, cmd := m.servers.Update(msg)
		m.servers = updated
		// Re-arm the listener for the next event.
		return m, tea.Batch(cmd, listenServerEvents(m.httpsrv))

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case widgets.SwitchViewMsg:
		prev := m.active
		m.active = ViewID(msg.ID)
		if prev != m.active {
			return m, m.initScreen(m.active)
		}
		return m, nil

	case SwitchToLicensesMsg:
		m.active = ViewLicenses
		filterMap := map[string]licenseFilter{
			"active":   licFilterActive,
			"expiring": licFilterExpiring,
			"expired":  licFilterExpired,
			"revoked":  licFilterRevoked,
		}
		if f, ok := filterMap[msg.Filter]; ok {
			m.licenses.filter = f
		}
		return m, m.licenses.Init()

	case PassphraseResult:
		if msg.Passphrase == "" {
			return m, tea.Quit
		}
		// Passphrase accepted — main.go handles re-keying; here we switch
		// the TUI to onboarding or dashboard depending on whether services exist.
		if m.services != nil {
			m.session = SessionReady
			return m, m.dashboard.refresh()
		}
		return m, tea.Quit

	case OnboardingDoneMsg:
		// Stash result so main.go can read it via OnboardingResult() after
		// the tea.Program exits, then quit the program.
		m.onboardingResult = &msg
		return m, tea.Quit

	// pushOverlayMsg is generated by screens that need to open an overlay.
	case pushOverlayMsg:
		m.overlays = append(m.overlays, msg.overlay)
		return m, m.overlays[len(m.overlays)-1].Init()
	}

	// Route remaining messages to the active screen.
	return m.routeToActive(msg)
}

func (m rootModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global navigation (1-9, q, ?) applies only when the session is ready.
	// Onboarding and passphrase screens own all key events so digits and
	// letters reach the focused textinput instead of being intercepted here.
	if m.session != SessionReady {
		return m.routeToActive(msg)
	}

	for i, id := range viewOrder {
		digit := string(rune('1' + i))
		if msg.String() == digit {
			prev := m.active
			m.active = id
			// Trigger initial load when switching to a screen for the first time.
			if prev != id {
				return m, m.initScreen(id)
			}
			return m, nil
		}
	}

	switch msg.String() {
	case "tab":
		m.active = nextView(m.active, +1)
		return m, m.initScreen(m.active)
	case "shift+tab":
		m.active = nextView(m.active, -1)
		return m, m.initScreen(m.active)
	case "q":
		serversRunning := m.httpsrv != nil
		if serversRunning {
			m.overlays = append(m.overlays, newQuitOverlay(true))
			return m, m.overlays[len(m.overlays)-1].Init()
		}
		return m, tea.Quit

	case "?":
		m.overlays = append(m.overlays, NewHelpOverlay())
		return m, m.overlays[len(m.overlays)-1].Init()

	case "r":
		if m.active == ViewDashboard && m.session == SessionReady {
			m.dashboard.loading = true
			return m, m.dashboard.refresh()
		}

	case "A":
		// Start all servers (capital A, servers screen shortcut).
		if m.active == ViewServers && m.httpsrv != nil {
			return m, startAllCmd(m.httpsrv)
		}

	case "Z":
		// Stop all servers (capital Z, servers screen shortcut).
		if m.active == ViewServers && m.httpsrv != nil {
			return m, stopAllCmd(m.httpsrv)
		}
	}

	return m.routeToActive(msg)
}

// initScreen returns the Init cmd for a screen when first navigated to.
func (m *rootModel) initScreen(id ViewID) tea.Cmd {
	switch id {
	case ViewLicenses:
		return m.licenses.Init()
	case ViewIssuers:
		return m.issuers.Init()
	case ViewRecipients:
		return m.recipients.Init()
	case ViewIdentities:
		return m.identities.Init()
	case ViewRevocation:
		return m.revocation.Init()
	case ViewAudit:
		return m.audit.Init()
	case ViewSettings:
		return m.settings.Init()
	case ViewServers:
		// Replay ring buffer into the servers log so history is visible
		// even when the operator navigated away and back.
		return m.replayEventRing()
	}
	return nil
}

// appendEventRing inserts ev into the root ring buffer, evicting the oldest
// entry when the capacity is reached. Uses a slice with modulo write index
// rather than container/ring to keep the snapshot serialisable.
func (m *rootModel) appendEventRing(ev httpsrv.Event) {
	if len(m.eventRing) < eventRingCap {
		m.eventRing = append(m.eventRing, ev)
		return
	}
	m.eventRing[m.eventRingIdx%eventRingCap] = ev
	m.eventRingIdx++
}

// replayEventRing returns a Cmd that drains the ordered ring buffer into
// the servers log model as a batch of serverEventMsg messages.
func (m *rootModel) replayEventRing() tea.Cmd {
	if len(m.eventRing) == 0 {
		return nil
	}
	// Build ordered slice: oldest first.
	ordered := make([]httpsrv.Event, len(m.eventRing))
	if len(m.eventRing) < eventRingCap {
		copy(ordered, m.eventRing)
	} else {
		start := m.eventRingIdx % eventRingCap
		copy(ordered, m.eventRing[start:])
		copy(ordered[eventRingCap-start:], m.eventRing[:start])
	}
	cmds := make([]tea.Cmd, len(ordered))
	for i, ev := range ordered {
		e := ev
		cmds[i] = func() tea.Msg { return serverEventMsg{ev: e} }
	}
	return tea.Batch(cmds...)
}

func (m rootModel) updateOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	top := m.overlays[len(m.overlays)-1]
	// Translate absolute mouse coords into overlay-relative coords so the
	// overlay's Update can hit-test its button row without knowing where
	// composeOverlay centered it.
	if mm, ok := msg.(tea.MouseMsg); ok && m.width > 0 && m.hgt > 0 {
		ov := top.View()
		ovLines := strings.Split(ov, "\n")
		ovH := len(ovLines)
		ovW := 0
		for _, l := range ovLines {
			if w := lipgloss.Width(l); w > ovW {
				ovW = w
			}
		}
		topY := (m.hgt - ovH) / 2
		leftX := (m.width - ovW) / 2
		mm.X -= leftX
		mm.Y -= topY
		msg = mm
	}
	updated, cmd := top.Update(msg)

	if done, ok := msg.(OverlayDoneMsg); ok {
		m.overlays = m.overlays[:len(m.overlays)-1]
		// Dispatch overlay result to the active screen.
		m = m.dispatchOverlayResult(done.Result)
		// When a wizard completes, reload the licence list so the new row appears.
		if _, isLicense := done.Result.(*service.IssuedLicense); isLicense {
			cmd = tea.Batch(cmd, ListLicensesCmd(m.services))
		}
		return m, cmd
	}
	m.overlays[len(m.overlays)-1] = updated
	return m, cmd
}

// dispatchOverlayResult routes overlay results back to the correct screen.
func (m rootModel) dispatchOverlayResult(result any) rootModel {
	if result == nil {
		return m
	}

	switch res := result.(type) {
	case *service.IssuedLicense:
		// Wizard completed successfully — show QR overlay.
		// License list reload is triggered via pendingCmd returned by updateOverlay.
		if res != nil {
			m.overlays = append(m.overlays, newQROverlay(res))
		}
		return m

	case RevokeConfirmedMsg:
		updated, _ := m.licenses.handleRevokeResult(res)
		m.licenses = updated

	case ConfirmResultMsg:
		switch m.active {
		case ViewIssuers:
			// Retire confirm — no service call for retire yet (Phase 3 will add it).
		case ViewRecipients:
			updated, _ := m.recipients.handleRecipientConfirmResult(res)
			m.recipients = updated
		case ViewIdentities:
			updated, _ := m.identities.handleIdentityConfirmResult(res)
			m.identities = updated
		case ViewRevocation:
			updated, _ := m.revocation.handleRevocationConfirmResult(res)
			m.revocation = updated
		}

	case InputResultMsg:
		switch m.active {
		case ViewIssuers:
			updated, _ := m.issuers.handleIssuerInputResult(res)
			m.issuers = updated
		case ViewRecipients:
			updated, _ := m.recipients.handleRecipientInputResult(res)
			m.recipients = updated
		case ViewIdentities:
			updated, _ := m.identities.handleIdentityInputResult(res)
			m.identities = updated
		case ViewRevocation:
			updated, _ := m.revocation.handleRevocationInputResult(res)
			m.revocation = updated
		case ViewAudit:
			updated, _ := m.audit.handleAuditInputResult(res)
			m.audit = updated
		}
	}
	return m
}

func (m rootModel) routeToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.session {
	case SessionLocked:
		updated, cmd := m.passphrase.update(msg)
		m.passphrase = updated
		return m, cmd

	case SessionOnboarding:
		updated, cmd := m.onboarding.update(msg)
		m.onboarding = updated
		return m, cmd

	case SessionReady:
		return m.routeReady(msg)
	}
	return m, nil
}

func (m rootModel) routeReady(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.active {
	case ViewDashboard:
		updated, cmd := m.dashboard.Update(msg)
		m.dashboard = updated
		return m, cmd
	case ViewLicenses:
		updated, cmd := m.licenses.Update(msg)
		m.licenses = updated
		return m, cmd
	case ViewIssuers:
		updated, cmd := m.issuers.Update(msg)
		m.issuers = updated
		return m, cmd
	case ViewRecipients:
		updated, cmd := m.recipients.Update(msg)
		m.recipients = updated
		return m, cmd
	case ViewIdentities:
		updated, cmd := m.identities.Update(msg)
		m.identities = updated
		return m, cmd
	case ViewRevocation:
		updated, cmd := m.revocation.Update(msg)
		m.revocation = updated
		return m, cmd
	case ViewServers:
		updated, cmd := m.servers.Update(msg)
		m.servers = updated
		return m, cmd
	case ViewAudit:
		updated, cmd := m.audit.Update(msg)
		m.audit = updated
		return m, cmd
	case ViewSettings:
		updated, cmd := m.settings.Update(msg)
		m.settings = updated
		return m, cmd
	}
	return m, nil
}

func (m rootModel) View() string {
	if m.width == 0 {
		return ""
	}

	var body string
	switch m.session {
	case SessionLocked:
		body = m.passphrase.View()
	case SessionOnboarding:
		body = m.onboarding.View()
	case SessionReady:
		body = m.viewReady()
	}

	// Render overlay on top when present, with the body dimmed underneath so
	// the operator keeps spatial context (matches design/prototype/overlays.jsx
	// Scrim — `dim via Faint` per the prototype comment).
	if len(m.overlays) > 0 {
		overlay := m.overlays[len(m.overlays)-1].View()
		return composeOverlay(body, overlay, m.width, m.hgt)
	}
	return body
}

// handleMouse dispatches mouse events. Left-button press OR release triggers
// click dispatch; both are accepted because terminal emulators vary — some send
// only Press (e.g. Windows Terminal with legacy mouse encoding), others send
// only Release, and some send both. Accepting either avoids silent no-ops.
// Wheel events are handled inside WrappedViewport widgets directly.
func (m rootModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if msg.Action != tea.MouseActionPress && msg.Action != tea.MouseActionRelease {
		return m, nil
	}
	// Tab bar occupies row Y=1 (row 0 = title bar, row 1 = tabs).
	if msg.Y == 1 {
		tb := buildTabBar(m.active, m.width)
		cmd := tb.OnClick(msg.X, 0, tea.MouseButtonLeft)
		return m, cmd
	}
	if m.active == ViewDashboard {
		tree := m.dashboard.buildWidgetTree()
		cmd := dispatchClick(tree, msg.X, msg.Y)
		return m, cmd
	}
	// Per-screen click router: each non-dashboard screen implements its own
	// hit-testing for chips / sub-tabs / rows. Screens that don't implement
	// ScreenMouseClick simply ignore body clicks (tab strip still works).
	if mc, ok := m.activeScreenWithMouse(); ok {
		cmd := mc.OnClick(msg.X, msg.Y, m.width)
		return m, cmd
	}
	if m.rootWidget != nil {
		cmd := dispatchClick(m.rootWidget, msg.X, msg.Y)
		return m, cmd
	}
	return m, nil
}

// ScreenMouseClick is the optional interface screens implement to handle clicks
// inside their content area (everything below row 2 — title + tabs). x,y are
// absolute terminal cell coordinates; width is the current terminal width so
// hit-testing helpers can mirror the renderer's layout decisions.
type ScreenMouseClick interface {
	OnClick(x, y, width int) tea.Cmd
}

func (m rootModel) activeScreenWithMouse() (ScreenMouseClick, bool) {
	switch m.active {
	case ViewLicenses:
		return m.licenses, true
	case ViewIssuers:
		return m.issuers, true
	case ViewRecipients:
		return m.recipients, true
	case ViewIdentities:
		return m.identities, true
	case ViewRevocation:
		return m.revocation, true
	case ViewServers:
		return m.servers, true
	case ViewAudit:
		return m.audit, true
	case ViewSettings:
		return m.settings, true
	}
	return nil, false
}

// dispatchClick walks the widget tree rooted at w and calls OnClick on the
// deepest Clickable whose bounds contain (x, y).
func dispatchClick(w Widget, x, y int) tea.Cmd {
	if !w.Bounds().Contains(x, y) {
		return nil
	}
	// Try children first (depth-first).
	type hasChildren interface {
		Children() []Widget
	}
	if parent, ok := w.(hasChildren); ok {
		for _, child := range parent.Children() {
			if cmd := dispatchClick(child, x, y); cmd != nil {
				return cmd
			}
		}
	}
	// Fall back to this widget if it is Clickable.
	if c, ok := w.(Clickable); ok {
		b := w.Bounds()
		return c.OnClick(x-b.X, y-b.Y, tea.MouseButtonLeft)
	}
	return nil
}

// activeScreenWithHints returns the ScreenWithHints implementation for the
// currently active view, if it has one, plus a boolean ok flag.
func (m rootModel) activeScreenWithHints() (ScreenWithHints, bool) {
	switch m.active {
	case ViewLicenses:
		return m.licenses, true
	case ViewIssuers:
		return m.issuers, true
	case ViewRecipients:
		return m.recipients, true
	case ViewIdentities:
		return m.identities, true
	case ViewRevocation:
		return m.revocation, true
	case ViewServers:
		return m.servers, true
	case ViewAudit:
		return m.audit, true
	case ViewSettings:
		return m.settings, true
	}
	return nil, false
}

func (m rootModel) viewReady() string {
	title := renderTitleBar(m.width)
	tabs := renderTabStrip(m.active, m.width)
	crumb := renderBreadcrumb(m.active, m.licenses.filter, m.width)

	var content string
	switch m.active {
	case ViewDashboard:
		content = m.dashboard.View()
	case ViewLicenses:
		content = m.licenses.View()
	case ViewIssuers:
		content = m.issuers.View()
	case ViewRecipients:
		content = m.recipients.View()
	case ViewIdentities:
		content = m.identities.View()
	case ViewRevocation:
		content = m.revocation.View()
	case ViewServers:
		content = m.servers.View()
	case ViewAudit:
		content = m.audit.View()
	case ViewSettings:
		content = m.settings.View()
	}

	// chrome = title(1) + tabs(1) + breadcrumb(1) + statusbar(1) = 4 rows.
	contentH := m.hgt - 4
	if contentH < 0 {
		contentH = 0
	}
	// Clamp content to exactly contentH lines: pad short content up, trim tall
	// content down. lipgloss Height() only pads — never truncates — so we must
	// enforce the ceiling ourselves to keep the total view == m.hgt.
	content = clampToHeight(content, contentH, m.width)
	contentArea := lipgloss.NewStyle().
		Width(m.width).
		Height(contentH).
		Render(content)

	// Use screen-specific hints when the active screen provides them;
	// fall back to the global default hint set otherwise.
	defaultHints := []string{
		"1-9", "onglets",
		"n", "nouvelle licence",
		"/", "rechercher",
		"k", "clés actives",
		"?", "aide",
		"q", "quitter",
	}
	activeHints := defaultHints
	if sh, ok := m.activeScreenWithHints(); ok {
		activeHints = sh.Hints()
	}
	statusBar := renderStatusBar(activeHints, m.width)

	chrome := lipgloss.JoinVertical(lipgloss.Left, title, tabs, crumb)

	// Hard-clamp chrome + content to (m.hgt - 1) lines so there is always exactly
	// one row left for the status bar regardless of chrome wrapping.
	body := lipgloss.JoinVertical(lipgloss.Left, chrome, contentArea)
	body = clampToHeight(body, m.hgt-1, m.width)

	return lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
}
