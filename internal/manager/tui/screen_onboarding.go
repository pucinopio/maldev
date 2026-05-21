package tui

import (
	"fmt"

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
		// Enter on field 0 advances focus to confirm; validation only runs on
		// field 1 so the operator never sees "do not match" before confirming.
		if m.passFocused == 0 {
			return m.advanceFocus(), nil
		}
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
		// Enter on name field advances focus to key ID; validation only runs on
		// field 1.
		if m.issuerFocus == 0 {
			return m.advanceFocus(), nil
		}
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

	// stepWelcome has no progress strip — it's a full-screen welcome banner.
	if m.step == stepWelcome {
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, m.viewWelcome())
	}

	// Steps 1-3 share a progress strip + centered content box.
	type stepInfo struct {
		n     int
		label string
	}
	info := []stepInfo{
		{1, "Passphrase DB"},
		{2, "Issuer & 1ère clé"},
		{3, "Première licence"},
	}[int(m.step)-1]

	// Progress strip.
	total := 3
	cur := info.n
	stripLeft := lipgloss.JoinHorizontal(lipgloss.Top,
		GlowMagent.Render("◆ PREMIÈRE UTILISATION"),
		Dim.Render("  étape  "),
		Base.Bold(true).Render(fmt.Sprintf("%d/3", cur)),
		Dim.Render("  ·  "),
		GlowCyan.Render(info.label),
	)
	stripHint := HintKey.Render("Tab") + HintText.Render(" continuer")
	strip := lipgloss.JoinHorizontal(lipgloss.Top,
		stripLeft,
		lipgloss.NewStyle().Width(w-lipgloss.Width(stripLeft)-lipgloss.Width(stripHint)-2).Render(""),
		stripHint,
	)
	bar := renderProgressBar(w, cur, total)
	progressStrip := lipgloss.JoinVertical(lipgloss.Left, strip, bar)

	var content string
	switch m.step {
	case stepPassphrase:
		content = m.viewPassphrase()
	case stepIssuer:
		content = m.viewIssuer()
	case stepLicense:
		content = m.viewLicense()
	}

	centred := lipgloss.Place(w, h-3, lipgloss.Center, lipgloss.Center, content)
	return lipgloss.JoinVertical(lipgloss.Left, progressStrip, centred)
}

// viewWelcome renders the prototype welcome banner with 4 feature cards in a
// 2×2 grid and a "Commencer" call to action.
func (m onboardingModel) viewWelcome() string {
	type card struct {
		n    int
		text string
		cyan bool // true = cyan border, false = magenta
	}
	cards := []card{
		{1, "Chiffrer une base SQLite locale avec ta passphrase", true},
		{2, "Créer ton identité d'émission (issuer)", true},
		{3, "Générer ta première paire de clés Ed25519", false},
		{4, "Émettre une licence pour toi, pinnée à ta machine", false},
	}

	cardW := 34
	var row1, row2 []string
	for i, c := range cards {
		color := Palette.Cyan
		if !c.cyan {
			color = Palette.Magenta
		}
		badge := lipgloss.NewStyle().
			Foreground(color).Bold(true).
			Border(lipgloss.NormalBorder()).BorderForeground(color).
			Padding(0, 0).Width(2).
			Render(fmt.Sprintf("%d", c.n))
		cell := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).BorderForeground(Palette.Border).
			Padding(0, 1).Width(cardW).
			Render(lipgloss.JoinHorizontal(lipgloss.Top, badge, " ", Dim.Render(c.text)))
		if i < 2 {
			row1 = append(row1, cell)
		} else {
			row2 = append(row2, cell)
		}
	}

	grid := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, row1...),
		lipgloss.JoinHorizontal(lipgloss.Top, row2...),
	)

	note := Dim.Render("Tu pourras revenir sur tous ces choix dans Settings après coup.")
	action := lipgloss.JoinHorizontal(lipgloss.Top,
		HintKey.Render("enter")+" "+HintText.Render("Commencer"),
		"   ",
		HintKey.Render("esc")+" "+HintText.Render("Quitter"),
	)

	header := GlowMagent.Render("◆ PREMIÈRE UTILISATION")
	subHead := Dim.Render("Aucune base détectée")

	return lipgloss.JoinVertical(lipgloss.Left,
		header, subHead, "", grid, "", note, "", action,
	)
}

func (m onboardingModel) viewPassphrase() string {
	lines := []string{
		GlowMagent.Render("2 / 4 — Passphrase de la base"),
		"",
		Dim.Render("Cette passphrase est demandée à chaque lancement."),
		Dim.Render("Note-la dans ton gestionnaire de mots de passe."),
		"",
		m.passInput.View(),
		m.passConfirm.View(),
		"",
	}
	if m.passErr != "" {
		lines = append(lines, GlowRed.Render(m.passErr))
	}
	lines = append(lines, "",
		HintKey.Render("tab")+" "+HintText.Render("champ suivant")+
			"  "+HintKey.Render("enter")+" "+HintText.Render("suivant"),
	)
	return BoxStyle.Width(58).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m onboardingModel) viewIssuer() string {
	lines := []string{
		GlowMagent.Render("3 / 4 — Issuer & première paire de clés"),
		"",
		Dim.Render("L'issuer est la clé Ed25519 qui signe tes licences."),
		"",
		Dim.Render("Nom de l'issuer :"),
		m.issuerName.View(),
		"",
		Dim.Render("KeyID (auto-suggéré) :"),
		m.issuerKeyID.View(),
		"",
		HintKey.Render("tab")+" "+HintText.Render("champ suivant")+
			"  "+HintKey.Render("enter")+" "+HintText.Render("Générer & continuer"),
	}
	return BoxStyle.Width(58).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m onboardingModel) viewLicense() string {
	lines := []string{
		GlowMagent.Render("4 / 4 — Première licence (pour toi, sur ta machine)"),
		"",
		Dim.Render("On crée une licence minimale pour valider le flow."),
		Dim.Render("Tu pourras en émettre d'autres depuis l'onglet Licences."),
		"",
		HintKey.Render("enter")+" "+HintText.Render("Émettre & entrer dans l'app")+
			"  "+HintKey.Render("esc")+" "+HintText.Render("passer"),
	}
	return BoxStyle.Width(58).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
