// Package tui implements the bubbletea-based terminal UI for license-manager.
// It covers the full operator workflow: onboarding, passphrase unlock, dashboard,
// and placeholder screens for views that ship in later phases.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/tui/widgets"
)

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

	// Servers stays placeholder until Phase 4.
	serversPH placeholderModel
}

// New constructs the root model ready to be handed to tea.NewProgram.
//
//   - services != nil + sess==SessionReady  → goes straight to Dashboard
//   - services == nil + sess==SessionLocked → passphrase prompt
//   - services == nil + sess==SessionOnboarding → onboarding wizard
func New(services *service.Services, bundle *httpsrv.Bundle, sess SessionState) rootModel {
	return rootModel{
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

		serversPH: newPlaceholderModel(ViewServers, "Phase 4"),
	}
}

func (m rootModel) Init() tea.Cmd {
	switch m.session {
	case SessionReady:
		return m.dashboard.refresh()
	case SessionLocked:
		return m.passphrase.Init()
	case SessionOnboarding:
		return m.onboarding.Init()
	}
	return nil
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
		m.serversPH, _ = m.serversPH.Update(msg)
		return m, nil

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
		// Onboarding complete — switch to locked/dashboard or quit so main can
		// build the service layer with the collected passphrase.
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
	// Global navigation: 1-9 switches views and lazily loads.
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
	case "q":
		serversRunning := m.httpsrv != nil
		if serversRunning {
			m.overlays = append(m.overlays, newQuitOverlay(true))
			return m, m.overlays[len(m.overlays)-1].Init()
		}
		return m, tea.Quit

	case "?":
		// Help overlay — Phase 2 placeholder.
		return m, nil

	case "r":
		if m.active == ViewDashboard && m.session == SessionReady {
			m.dashboard.loading = true
			return m, m.dashboard.refresh()
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
	}
	return nil
}

func (m rootModel) updateOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	top := m.overlays[len(m.overlays)-1]
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
		updated, cmd := m.serversPH.Update(msg)
		m.serversPH = updated
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

	// Render overlay on top when present.
	if len(m.overlays) > 0 {
		overlay := m.overlays[len(m.overlays)-1].View()
		return lipgloss.Place(m.width, m.hgt, lipgloss.Center, lipgloss.Center, overlay)
	}
	return body
}

// handleMouse dispatches mouse events. Left-button release triggers click
// dispatch; wheel is handled inside WrappedViewport widgets directly.
func (m rootModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
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
		if m.rootWidget != nil {
			cmd := dispatchClick(m.rootWidget, msg.X, msg.Y)
			return m, cmd
		}
	}
	return m, nil
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

func (m rootModel) viewReady() string {
	title := renderTitleBar(m.width)
	tabs := renderTabStrip(m.active, m.width)

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
		content = m.serversPH.View()
	case ViewAudit:
		content = m.audit.View()
	case ViewSettings:
		content = m.settings.View()
	}

	// Reserve 2 rows for chrome (title + tabs); status bar is rendered by each screen.
	contentH := m.hgt - 2
	if contentH < 0 {
		contentH = 0
	}
	contentArea := lipgloss.NewStyle().
		Width(m.width).
		Height(contentH).
		Render(content)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		tabs,
		contentArea,
	)
}
