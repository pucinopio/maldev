// Package tui implements the bubbletea-based terminal UI for license-manager.
// It covers the full operator workflow: onboarding, passphrase unlock, dashboard,
// and placeholder screens for views that ship in later phases.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/httpsrv"
	"github.com/oioio-space/maldev/internal/manager/service"
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

	// Phase 1 screens
	passphrase passphraseModel
	onboarding onboardingModel
	dashboard  dashboardModel

	// Placeholder screens for views 2-9 (wired in later phases).
	placeholders map[ViewID]placeholderModel
}

// New constructs the root model ready to be handed to tea.NewProgram.
//
//   - services != nil + sess==SessionReady  → goes straight to Dashboard
//   - services == nil + sess==SessionLocked → passphrase prompt
//   - services == nil + sess==SessionOnboarding → onboarding wizard
func New(services *service.Services, bundle *httpsrv.Bundle, sess SessionState) rootModel {
	placeholders := map[ViewID]placeholderModel{
		ViewLicenses:   newPlaceholderModel(ViewLicenses, "Phase 2"),
		ViewIssuers:    newPlaceholderModel(ViewIssuers, "Phase 2"),
		ViewRecipients: newPlaceholderModel(ViewRecipients, "Phase 2"),
		ViewIdentities: newPlaceholderModel(ViewIdentities, "Phase 2"),
		ViewRevocation: newPlaceholderModel(ViewRevocation, "Phase 3"),
		ViewServers:    newPlaceholderModel(ViewServers, "Phase 4"),
		ViewAudit:      newPlaceholderModel(ViewAudit, "Phase 3"),
		ViewSettings:   newPlaceholderModel(ViewSettings, "Phase 2"),
	}
	return rootModel{
		session:      sess,
		active:       ViewDashboard,
		services:     services,
		httpsrv:      bundle,
		keys:         newKeyMap(),
		passphrase:   newPassphraseModel(),
		onboarding:   newOnboardingModel(),
		dashboard:    newDashboardModel(services, bundle),
		placeholders: placeholders,
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
		for id, ph := range m.placeholders {
			updated, _ := ph.Update(msg)
			m.placeholders[id] = updated
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case PassphraseResult:
		if msg.Passphrase == "" {
			return m, tea.Quit
		}
		// Passphrase accepted — main.go handles re-keying; here we just switch
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
	}

	// Route remaining messages to the active screen.
	return m.routeToActive(msg)
}

func (m rootModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global navigation: 1-9 switches views.
	for i, id := range viewOrder {
		digit := string(rune('1' + i))
		if msg.String() == digit {
			m.active = id
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

func (m rootModel) updateOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	top := m.overlays[len(m.overlays)-1]
	updated, cmd := top.Update(msg)

	if done, ok := msg.(OverlayDoneMsg); ok {
		m.overlays = m.overlays[:len(m.overlays)-1]
		if quit, ok := done.Result.(bool); ok && quit {
			return m, tea.Quit
		}
		return m, nil
	}
	m.overlays[len(m.overlays)-1] = updated
	return m, cmd
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
		if m.active == ViewDashboard {
			updated, cmd := m.dashboard.Update(msg)
			m.dashboard = updated
			return m, cmd
		}
		if ph, ok := m.placeholders[m.active]; ok {
			updated, cmd := ph.Update(msg)
			m.placeholders[m.active] = updated
			return m, cmd
		}
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

func (m rootModel) viewReady() string {
	title := renderTitleBar(m.width)
	tabs := renderTabStrip(m.active, m.width)

	var content string
	if m.active == ViewDashboard {
		content = m.dashboard.View()
	} else if ph, ok := m.placeholders[m.active]; ok {
		content = ph.View()
	}

	hints := []string{"q", "quit", "?", "help", "r", "refresh", "1-9", "switch view"}
	statusBar := renderStatusBar(hints, m.width)

	// Reserve 3 rows for chrome (title + tabs + status).
	contentH := m.hgt - 3
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
		statusBar,
	)
}
