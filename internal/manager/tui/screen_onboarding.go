package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// onboardingStep identifies which step of the 4-step wizard is active.
type onboardingStep int

const (
	stepWelcome    onboardingStep = iota // 0 — welcome banner
	stepPassphrase                       // 1 — set passphrase
	stepIssuer                           // 2 — first issuer name + key ID
	stepLicense                          // 3 — first license or skip
)

// OnboardingDoneMsg is sent when the wizard completes (or is skipped).
type OnboardingDoneMsg struct {
	Passphrase  string
	IssuerName  string
	IssuerKeyID string
	Skipped     bool
}

// onboardingModel drives the first-launch wizard.
type onboardingModel struct {
	step onboardingStep

	// step 1 — passphrase
	passInput    textinput.Model
	passConfirm  textinput.Model
	passFocused  int // 0 = pass, 1 = confirm
	passErr      string

	// step 2 — issuer
	issuerName  textinput.Model
	issuerKeyID textinput.Model
	issuerFocus int // 0 = name, 1 = key ID

	width int
	hgt   int
}

func newOnboardingModel() onboardingModel {
	pass := textinput.New()
	pass.Placeholder = "passphrase"
	pass.EchoMode = textinput.EchoPassword
	pass.EchoCharacter = '•'
	pass.Focus()

	confirm := textinput.New()
	confirm.Placeholder = "confirm passphrase"
	confirm.EchoMode = textinput.EchoPassword
	confirm.EchoCharacter = '•'

	name := textinput.New()
	name.Placeholder = "e.g. production-2026"
	name.Focus()

	keyID := textinput.New()
	keyID.Placeholder = "e.g. maldev-prod-01"

	return onboardingModel{
		passInput:   pass,
		passConfirm: confirm,
		issuerName:  name,
		issuerKeyID: keyID,
	}
}

func (m onboardingModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m onboardingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.update(msg)
	return updated, cmd
}

func (m onboardingModel) update(msg tea.Msg) (onboardingModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.hgt = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyTab:
			return m.advanceFocus(), nil

		case tea.KeyEnter:
			return m.handleEnter()

		case tea.KeyEsc:
			// Skip license step only.
			if m.step == stepLicense {
				return m, func() tea.Msg {
					return OnboardingDoneMsg{Skipped: true}
				}
			}
		}
	}

	return m.forwardToInput(msg)
}

// advanceFocus cycles focus within multi-field steps.
func (m onboardingModel) advanceFocus() onboardingModel {
	switch m.step {
	case stepPassphrase:
		if m.passFocused == 0 {
			m.passFocused = 1
			m.passInput.Blur()
			m.passConfirm.Focus()
		} else {
			m.passFocused = 0
			m.passConfirm.Blur()
			m.passInput.Focus()
		}
	case stepIssuer:
		if m.issuerFocus == 0 {
			m.issuerFocus = 1
			m.issuerName.Blur()
			m.issuerKeyID.Focus()
		} else {
			m.issuerFocus = 0
			m.issuerKeyID.Blur()
			m.issuerName.Focus()
		}
	}
	return m
}

func (m onboardingModel) handleEnter() (onboardingModel, tea.Cmd) {
	switch m.step {
	case stepWelcome:
		m.step = stepPassphrase
		return m, nil

	case stepPassphrase:
		p := m.passInput.Value()
		c := m.passConfirm.Value()
		if p == "" {
			m.passErr = "passphrase must not be empty"
			return m, nil
		}
		if p != c {
			m.passErr = "passphrases do not match"
			m.passConfirm.SetValue("")
			return m, nil
		}
		m.passErr = ""
		m.step = stepIssuer
		return m, nil

	case stepIssuer:
		name := m.issuerName.Value()
		keyID := m.issuerKeyID.Value()
		if name == "" || keyID == "" {
			return m, nil
		}
		m.step = stepLicense
		return m, nil

	case stepLicense:
		// For Phase 1, entering on the license step completes with skip.
		return m, func() tea.Msg {
			return OnboardingDoneMsg{
				Passphrase:  m.passInput.Value(),
				IssuerName:  m.issuerName.Value(),
				IssuerKeyID: m.issuerKeyID.Value(),
				Skipped:     true,
			}
		}
	}
	return m, nil
}

func (m onboardingModel) forwardToInput(msg tea.Msg) (onboardingModel, tea.Cmd) {
	var cmd tea.Cmd
	switch m.step {
	case stepPassphrase:
		if m.passFocused == 0 {
			m.passInput, cmd = m.passInput.Update(msg)
		} else {
			m.passConfirm, cmd = m.passConfirm.Update(msg)
		}
	case stepIssuer:
		if m.issuerFocus == 0 {
			m.issuerName, cmd = m.issuerName.Update(msg)
		} else {
			m.issuerKeyID, cmd = m.issuerKeyID.Update(msg)
		}
	}
	return m, cmd
}

func (m onboardingModel) View() string {
	w := m.width
	h := m.hgt
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}

	var content string
	switch m.step {
	case stepWelcome:
		content = m.viewWelcome()
	case stepPassphrase:
		content = m.viewPassphrase()
	case stepIssuer:
		content = m.viewIssuer()
	case stepLicense:
		content = m.viewLicense()
	}

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}

func (m onboardingModel) viewWelcome() string {
	lines := []string{
		GlowCyan.Render("  license-manager  "),
		"",
		Dim.Render("First launch — no database found."),
		Dim.Render("This wizard will set up your store in a few steps."),
		"",
		Base.Render("Steps:"),
		Mute.Render("  1. Set a passphrase"),
		Mute.Render("  2. Create your first issuer key"),
		Mute.Render("  3. Issue a first license (optional)"),
		"",
		HintKey.Render("enter") + HintText.Render(" begin"),
	}
	return BoxStyle.Width(54).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m onboardingModel) viewPassphrase() string {
	lines := []string{
		GlowMagent.Render("Step 1 of 3 — Set passphrase"),
		"",
		Dim.Render("Choose a strong passphrase to protect the key store."),
		"",
		m.passInput.View(),
		m.passConfirm.View(),
		"",
	}
	if m.passErr != "" {
		lines = append(lines, GlowRed.Render(m.passErr))
	}
	lines = append(lines, "",
		HintKey.Render("tab")+" "+HintText.Render("next field")+
			"  "+HintKey.Render("enter")+" "+HintText.Render("confirm"),
	)
	return BoxStyle.Width(54).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m onboardingModel) viewIssuer() string {
	lines := []string{
		GlowMagent.Render("Step 2 of 3 — First issuer"),
		"",
		Dim.Render("An issuer is an Ed25519 signing key with a human name."),
		"",
		Base.Render("Name:"),
		m.issuerName.View(),
		"",
		Base.Render("Key ID (short identifier):"),
		m.issuerKeyID.View(),
		"",
		HintKey.Render("tab")+" "+HintText.Render("next field")+
			"  "+HintKey.Render("enter")+" "+HintText.Render("create"),
	}
	return BoxStyle.Width(54).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m onboardingModel) viewLicense() string {
	lines := []string{
		GlowMagent.Render("Step 3 of 3 — First license"),
		"",
		Dim.Render("You can issue a first license now, or skip and do it later"),
		Dim.Render("from the Licenses view."),
		"",
		HintKey.Render("enter")+" "+HintText.Render("skip for now")+
			"  "+HintKey.Render("esc")+" "+HintText.Render("skip"),
	}
	return BoxStyle.Width(54).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
