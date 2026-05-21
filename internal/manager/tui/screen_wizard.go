package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/tui/wizard"
)

// wizardStep enumerates the 8 steps of the New License Wizard.
type wizardStep int

const (
	wizStepIdentity wizardStep = iota
	wizStepRecipient
	wizStepMachine
	wizStepBinary
	wizStepValidity
	wizStepFreeFields
	wizStepTOTP
	wizStepReview
	wizStepDone
)

// wizardModel is the top-level model for the New License Wizard.
// It owns the 8 step widgets and drives transitions between them.
type wizardModel struct {
	svc    *service.Services
	step   wizardStep
	state  wizard.WizardState
	width  int
	hgt    int

	stepIdentity  *wizard.StepIdentity
	stepRecipient *wizard.StepRecipient
	stepMachine   *wizard.StepBindingMachine
	stepBinary    *wizard.StepBindingBinary
	stepValidity  *wizard.StepValidity
	stepFreeFields *wizard.StepFreeFields
	stepTOTP      *wizard.StepTOTP
	stepReview    *wizard.StepReview
}

// WizardDoneMsg is sent when the wizard completes (success or cancel).
type WizardDoneMsg struct {
	Issued *service.IssuedLicense // nil on cancel
}

func newWizardModel(svc *service.Services) wizardModel {
	return wizardModel{
		svc:            svc,
		step:           wizStepIdentity,
		stepIdentity:   wizard.NewStepIdentity(svc),
		stepRecipient:  wizard.NewStepRecipient(svc),
		stepMachine:    wizard.NewStepBindingMachine(),
		stepBinary:     wizard.NewStepBindingBinary(),
		stepValidity:   wizard.NewStepValidity(),
		stepFreeFields: wizard.NewStepFreeFields(),
		stepTOTP:       wizard.NewStepTOTP(svc),
		stepReview:     wizard.NewStepReview(svc),
	}
}

func (m wizardModel) Init() tea.Cmd {
	m.stepIdentity.Focus()
	return m.stepIdentity.LoadCmd()
}

func (m wizardModel) Update(msg tea.Msg) (wizardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.hgt = msg.Height
		return m, nil

	// Step 1 outcomes.
	case wizard.IdentityLoadedMsg:
		_, cmd := m.stepIdentity.Update(msg)
		return m, cmd
	case wizard.IdentityChosenMsg:
		m.state.IssuerID = msg.IssuerID
		m.stepTOTP.SetIssuerID(msg.IssuerID)
		return m.advance(wizStepRecipient)

	// Step 2 outcomes.
	case wizard.RecipientLoadedMsg:
		_, cmd := m.stepRecipient.Update(msg)
		return m, cmd
	case wizard.RecipientChosenMsg:
		m.state.RecipientID = msg.RecipientID
		return m.advance(wizStepMachine)

	// Step 3 outcomes.
	case wizard.MachineBindingMsg:
		m.state.MachineID = msg.MachineID
		return m.advance(wizStepBinary)
	case wizard.OpenProbeDrawerMsg:
		// Open probe drawer as an overlay; machine step stays open.
		return m, func() tea.Msg {
			return pushOverlayMsg{newProbeDrawerOverlay(m.svc, func(machineID string) tea.Cmd {
				return func() tea.Msg { return wizard.MachineBindingMsg{MachineID: machineID} }
			})}
		}

	// Step 4 outcomes.
	case wizard.BinaryHashedMsg:
		_, cmd := m.stepBinary.Update(msg)
		return m, cmd
	case wizard.BinaryBindingMsg:
		m.state.BinarySHA256 = msg.SHA256
		m.state.BinarySize = msg.Size
		return m.advance(wizStepValidity)
	case wizard.OpenFilePickerMsg:
		return m, func() tea.Msg {
			return pushOverlayMsg{newFilePickerOverlay(func(path string) tea.Cmd {
				return func() tea.Msg { return filePickedMsg{path: path, callback: msg.Callback} }
			})}
		}
	case filePickedMsg:
		if msg.callback == "binary" {
			m.stepBinary.SetPath(msg.path)
			return m, hashFileCmd(msg.path)
		}
		return m, nil

	// Step 5 outcomes.
	case wizard.ValidityMsg:
		m.state.NotBefore = msg.NotBefore
		m.state.NotAfter = msg.NotAfter
		return m.advance(wizStepFreeFields)

	// Step 6 outcomes.
	case wizard.FreeFieldsMsg:
		m.state.FreeFields = msg.Fields
		return m.advance(wizStepTOTP)

	// Step 7 outcomes.
	case wizard.TOTPSecretsLoadedMsg:
		_, cmd := m.stepTOTP.Update(msg)
		return m, cmd
	case wizard.TOTPChoiceMsg:
		m.state.RequireTOTP = msg.Require
		m.state.TOTPSecretID = msg.TOTPSecretID
		m.stepReview.SetState(m.state)
		return m.advance(wizStepReview)

	// Step 8 outcomes.
	case wizard.IssueResultMsg:
		// Propagate the msg to stepReview to clear its issuing flag.
		m.stepReview.Update(msg) //nolint:errcheck — cmd is always nil for IssueResultMsg
		if msg.Err != nil && msg.Err.Error() == "cancelled" {
			return m, func() tea.Msg { return WizardDoneMsg{} }
		}
		if msg.Err != nil {
			return m, func() tea.Msg {
				return pushOverlayMsg{newErrorOverlay("Issue Failed", msg.Err.Error())}
			}
		}
		issued := msg.Issued
		return m, func() tea.Msg { return WizardDoneMsg{Issued: issued} }

	// Global back navigation.
	case tea.KeyMsg:
		if msg.String() == "esc" && m.step > wizStepIdentity {
			return m.retreat()
		}
		return m.routeKeyToStep(msg)
	}

	return m.routeMsgToStep(msg)
}

// advance moves to the next step, focuses it, and fires any init cmd.
func (m wizardModel) advance(next wizardStep) (wizardModel, tea.Cmd) {
	m.blurCurrent()
	m.step = next
	return m, m.initStep(next)
}

// retreat moves back one step.
func (m wizardModel) retreat() (wizardModel, tea.Cmd) {
	if m.step == wizStepIdentity {
		return m, nil
	}
	m.blurCurrent()
	m.step--
	return m, m.initStep(m.step)
}

func (m *wizardModel) blurCurrent() {
	switch m.step {
	case wizStepIdentity:
		m.stepIdentity.Blur()
	case wizStepRecipient:
		m.stepRecipient.Blur()
	case wizStepMachine:
		m.stepMachine.Blur()
	case wizStepBinary:
		m.stepBinary.Blur()
	case wizStepValidity:
		m.stepValidity.Blur()
	case wizStepFreeFields:
		m.stepFreeFields.Blur()
	case wizStepTOTP:
		m.stepTOTP.Blur()
	case wizStepReview:
		m.stepReview.Blur()
	}
}

func (m wizardModel) initStep(s wizardStep) tea.Cmd {
	switch s {
	case wizStepIdentity:
		m.stepIdentity.Focus()
		return m.stepIdentity.LoadCmd()
	case wizStepRecipient:
		m.stepRecipient.Focus()
		return m.stepRecipient.LoadCmd()
	case wizStepMachine:
		m.stepMachine.Focus()
	case wizStepBinary:
		m.stepBinary.Focus()
	case wizStepValidity:
		m.stepValidity.Focus()
	case wizStepFreeFields:
		m.stepFreeFields.Focus()
	case wizStepTOTP:
		m.stepTOTP.Focus()
		return m.stepTOTP.LoadCmd()
	case wizStepReview:
		m.stepReview.Focus()
	}
	return nil
}

func (m wizardModel) routeKeyToStep(msg tea.KeyMsg) (wizardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch m.step {
	case wizStepIdentity:
		_, cmd = m.stepIdentity.Update(msg)
	case wizStepRecipient:
		_, cmd = m.stepRecipient.Update(msg)
	case wizStepMachine:
		_, cmd = m.stepMachine.Update(msg)
	case wizStepBinary:
		_, cmd = m.stepBinary.Update(msg)
	case wizStepValidity:
		_, cmd = m.stepValidity.Update(msg)
	case wizStepFreeFields:
		_, cmd = m.stepFreeFields.Update(msg)
	case wizStepTOTP:
		_, cmd = m.stepTOTP.Update(msg)
	case wizStepReview:
		_, cmd = m.stepReview.Update(msg)
	}
	return m, cmd
}

func (m wizardModel) routeMsgToStep(msg tea.Msg) (wizardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch m.step {
	case wizStepValidity:
		_, cmd = m.stepValidity.Update(msg)
	}
	return m, cmd
}

// hashFileCmd computes SHA-256 of path in a goroutine and returns BinaryHashedMsg.
func hashFileCmd(path string) tea.Cmd {
	return func() tea.Msg {
		f, err := os.Open(path)
		if err != nil {
			return wizard.BinaryHashedMsg{Err: err}
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return wizard.BinaryHashedMsg{Err: err}
		}
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return wizard.BinaryHashedMsg{Err: err}
		}
		return wizard.BinaryHashedMsg{
			SHA256: hex.EncodeToString(h.Sum(nil)),
			Size:   info.Size(),
		}
	}
}

func (m wizardModel) View() string {
	fgDim := lipgloss.NewStyle().Foreground(Palette.FgDim)

	// Progress breadcrumb.
	steps := []string{"Identity", "Recipient", "Machine", "Binary", "Validity", "Fields", "TOTP", "Review"}
	crumbs := make([]string, len(steps))
	for i, s := range steps {
		if wizardStep(i) < m.step {
			crumbs[i] = fgDim.Render(s)
		} else if wizardStep(i) == m.step {
			crumbs[i] = GlowMagent.Render(s)
		} else {
			crumbs[i] = Mute.Render(s)
		}
	}
	progress := " " + lipgloss.JoinHorizontal(lipgloss.Top, joinWithSep(crumbs, Mute.Render(" › "))...)

	var body string
	switch m.step {
	case wizStepIdentity:
		body = m.stepIdentity.View()
	case wizStepRecipient:
		body = m.stepRecipient.View()
	case wizStepMachine:
		body = m.stepMachine.View()
	case wizStepBinary:
		body = m.stepBinary.View()
	case wizStepValidity:
		body = m.stepValidity.View()
	case wizStepFreeFields:
		body = m.stepFreeFields.View()
	case wizStepTOTP:
		body = m.stepTOTP.View()
	case wizStepReview:
		body = m.stepReview.View()
	}

	hints := renderStatusBar([]string{"esc", "back", "q", "cancel"}, m.width)
	return lipgloss.JoinVertical(lipgloss.Left,
		GlowMagent.Render(" New License Wizard"),
		progress,
		"",
		body,
		hints,
	)
}

// joinWithSep interleaves sep between each element of ss.
func joinWithSep(ss []string, sep string) []string {
	if len(ss) == 0 {
		return nil
	}
	out := make([]string, 0, len(ss)*2-1)
	for i, s := range ss {
		out = append(out, s)
		if i < len(ss)-1 {
			out = append(out, sep)
		}
	}
	return out
}

// filePickedMsg is an internal message carrying a path chosen by the file picker.
type filePickedMsg struct {
	path     string
	callback string
}

// openWizardCmd returns a Cmd that pushes the wizard as an overlay.
func openWizardCmd(svc *service.Services) tea.Cmd {
	return func() tea.Msg {
		wiz := newWizardModel(svc)
		return pushOverlayMsg{overlay: &wizardOverlay{model: wiz}}
	}
}

// wizardOverlay wraps wizardModel as a full-screen Overlay.
type wizardOverlay struct {
	model wizardModel
}

func (o *wizardOverlay) Init() tea.Cmd {
	return o.model.Init()
}

func (o *wizardOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	// WizardDoneMsg closes the overlay.
	if done, ok := msg.(WizardDoneMsg); ok {
		issued := done.Issued
		return o, func() tea.Msg {
			return OverlayDoneMsg{Result: issued}
		}
	}
	updated, cmd := o.model.Update(msg)
	o.model = updated
	return o, cmd
}

func (o *wizardOverlay) View() string {
	return o.model.View()
}

