package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
		stepMachine:    wizard.NewStepBindingMachine(svc),
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
		m.state.Subject = msg.Subject
		m.state.Audience = msg.Audience
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

	// Global back / discard navigation.
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+q", "ctrl+x":
			// Universal discard — drops everything entered so far and closes
			// the wizard overlay. ctrl+x is the friendlier alias for users who
			// expect a "cancel" shortcut that doesn't double as the terminal
			// SIGINT key.
			return m, func() tea.Msg { return WizardDoneMsg{Issued: nil} }
		case "esc":
			if m.step > wizStepIdentity {
				return m.retreat()
			}
			// On step 1 (no progress yet) esc discards.
			return m, func() tea.Msg { return WizardDoneMsg{Issued: nil} }
		case "ctrl+right", "ctrl+n":
			// Explicit "next step" — skip the current step with whatever
			// optional state has already been entered. Mandatory steps (1, 2)
			// require Enter via the step's own handler so they're not skipped
			// here.
			if m.step < wizStepReview {
				return m.advance(m.step + 1)
			}
			return m, nil
		case "ctrl+left", "ctrl+p":
			// Explicit "previous step" — equivalent to esc but works even when
			// the step has a textinput focused (where esc would otherwise be
			// caught by the input).
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

// gotoStep jumps to an arbitrary step — used by the sidebar click handler.
// Backward jumps allowed; forward jumps allowed too (the operator owns the
// flow, idempotent steps in the wizard cope with revisits).
func (m wizardModel) gotoStep(target wizardStep) (wizardModel, tea.Cmd) {
	if target < wizStepIdentity || target > wizStepReview {
		return m, nil
	}
	m.blurCurrent()
	m.step = target
	return m, m.initStep(target)
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
		return m.stepMachine.FocusCmd()
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
		// Refresh the snapshot the review screen displays so it stays
		// in sync whenever the user lands here — via the natural
		// step-7→step-8 advance OR via a direct sidebar click that
		// skipped intermediate SetState calls.
		m.stepReview.SetState(m.state)
		m.stepReview.Focus()
	}
	return nil
}

// routeBodyClick dispatches a wizard-body click (already in step-local coords)
// to whichever step exposes an OnClick. Only steps with click affordances
// need to participate; others return nil and the click is ignored.
func (m wizardModel) routeBodyClick(x, y int) tea.Cmd {
	type clickable interface {
		OnClick(x, y int) tea.Cmd
	}
	switch m.step {
	case wizStepTOTP:
		if c, ok := any(m.stepTOTP).(clickable); ok {
			return c.OnClick(x, y)
		}
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

// wizardStepMeta holds display metadata for one wizard step.
type wizardStepMeta struct {
	label string
	hint  string
}

// wizardStepMetas maps each wizardStep (by index) to its prototype display
// label and hint. Order must match the wizardStep iota constants.
var wizardStepMetas = []wizardStepMeta{
	/* wizStepIdentity  */ {"Identité", "subject · issuer · audience · KeyID"},
	/* wizStepRecipient */ {"Destinataire", "clé X25519 du destinataire"},
	/* wizStepMachine   */ {"Machine", "binding machine (hostid)"},
	/* wizStepBinary    */ {"Binaire", "SHA256 du binaire"},
	/* wizStepValidity  */ {"Validité", "NotBefore · NotAfter"},
	/* wizStepFreeFields*/ {"Champs libres", "claims JSON libres"},
	/* wizStepTOTP      */ {"TOTP", "binding TOTP (optionnel)"},
	/* wizStepReview    */ {"Récap & émettre", "aperçu + signature"},
}

func (m wizardModel) View() string {
	total := len(wizardStepMetas)
	// Clamp step index — wizStepDone equals len(wizardStepMetas) and must not
	// be used as an array index; show the last real step in that transient state.
	stepIdx := int(m.step)
	if stepIdx >= total {
		stepIdx = total - 1
	}
	cur := stepIdx + 1 // 1-based for display

	// ── Progress strip ────────────────────────────────────────────────────
	meta := wizardStepMetas[stepIdx]
	stripLeft := lipgloss.JoinHorizontal(lipgloss.Top,
		GlowMagent.Render("NOUVELLE LICENCE"),
		Dim.Render("  étape  "),
		Base.Render(fmt.Sprintf("%d/%d", cur, total)),
		Dim.Render("  ·  "),
		GlowCyan.Render(meta.label),
	)
	stripHints := lipgloss.JoinHorizontal(lipgloss.Top,
		HintKey.Render("Tab"), HintText.Render(" suivant  "),
		HintKey.Render("⇧Tab"), HintText.Render(" précédent  "),
		HintKey.Render("1-8"), HintText.Render(" aller à  "),
		HintKey.Render("esc"), HintText.Render(" précédent / annuler "),
		HintKey.Render("ctrl+c"), HintText.Render(" discarder"),
	)
	strip := lipgloss.JoinHorizontal(lipgloss.Top,
		stripLeft,
		lipgloss.NewStyle().Width(m.width-lipgloss.Width(stripLeft)-lipgloss.Width(stripHints)-2).Render(""),
		stripHints,
	)

	bar := renderProgressBar(m.width, cur, total)

	progressStrip := lipgloss.JoinVertical(lipgloss.Left, strip, bar)

	// ── Sidebar ───────────────────────────────────────────────────────────
	// sideW is the inner text width of the sidebar column (before the right border).
	// Prototype uses ~260px ≈ 26 chars of text; add 1 for the right border = 27 total.
	sideW := 27
	var sideLines []string
	for i, sm := range wizardStepMetas {
		s := wizardStep(i)
		// Badge: "[N]" styled by active/done state — single-line, no lipgloss borders.
		var badge string
		if s == m.step {
			badge = lipgloss.NewStyle().Foreground(Palette.Magenta).Bold(true).Render(fmt.Sprintf("[%d]", i+1))
		} else {
			badge = lipgloss.NewStyle().Foreground(Palette.FgMute).Render(fmt.Sprintf("[%d]", i+1))
		}

		var labelStyle lipgloss.Style
		if s == m.step {
			labelStyle = Base.Bold(true)
		} else {
			labelStyle = Dim
		}
		label := labelStyle.Render(sm.label)

		// Active step gets a "│ " left-margin accent; others get "  ".
		var prefix string
		if s == m.step {
			prefix = lipgloss.NewStyle().Foreground(Palette.Magenta).Render("│") + " "
		} else {
			prefix = "  "
		}
		row := prefix + badge + " " + label
		sideLines = append(sideLines, row)
		// Truncate hint so it never wraps: 4-char indent + right border + 1 clearance.
		sideLines = append(sideLines, "    "+Mute.Render(truncateRunes(sm.hint, sideW-6)))
	}
	sidebar := lipgloss.NewStyle().
		Width(sideW).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(Palette.Border).
		Render(lipgloss.JoinVertical(lipgloss.Left, sideLines...))

	// ── Step content ──────────────────────────────────────────────────────
	var stepBody string
	switch m.step {
	case wizStepIdentity:
		stepBody = m.stepIdentity.View()
	case wizStepRecipient:
		stepBody = m.stepRecipient.View()
	case wizStepMachine:
		stepBody = m.stepMachine.View()
	case wizStepBinary:
		stepBody = m.stepBinary.View()
	case wizStepValidity:
		stepBody = m.stepValidity.View()
	case wizStepFreeFields:
		stepBody = m.stepFreeFields.View()
	case wizStepTOTP:
		stepBody = m.stepTOTP.View()
	case wizStepReview:
		stepBody = m.stepReview.View()
	}

	contentW := m.width - sideW - 4
	if contentW < 20 {
		contentW = 20
	}
	content := lipgloss.NewStyle().Width(contentW).Padding(0, 1).Render(stepBody)

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	hints := renderStatusBar([]string{"Tab", "next", "⇧Tab", "prev", "1-8", "goto", "esc", "cancel"}, m.width)
	return lipgloss.JoinVertical(lipgloss.Left,
		progressStrip,
		body,
		hints,
	)
}

// NewWizardSnap constructs a standalone wizardModel (svc=nil) sized to the given
// terminal dimensions. The returned model implements tea.Model so cmd/tui-snap
// can drive it via Update and render individual steps for visual snapshots.
func NewWizardSnap(width, height int) tea.Model {
	m := newWizardModel(nil)
	m.width = width
	m.hgt = height
	return wizardSnapModel{m}
}

// wizardSnapModel wraps wizardModel as a tea.Model for tui-snap use.
type wizardSnapModel struct{ inner wizardModel }

func (w wizardSnapModel) Init() tea.Cmd { return w.inner.Init() }
func (w wizardSnapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := w.inner.Update(msg)
	return wizardSnapModel{updated}, cmd
}
func (w wizardSnapModel) View() string { return w.inner.View() }

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
	// Sidebar step click: Y=0,1 are progress strip, Y=2..17 are 8×2-line
	// step rows. Compute which step the click landed on and jump there.
	if mm, ok := msg.(tea.MouseMsg); ok && mm.Button == tea.MouseButtonLeft && mm.Action == tea.MouseActionPress {
		if mm.X < 28 && mm.Y >= 2 && mm.Y < 2+2*len(wizardStepMetas) {
			target := wizardStep((mm.Y - 2) / 2)
			updated, cmd := o.model.gotoStep(target)
			o.model = updated
			return o, cmd
		}
		// Body click → translate into step-local coords (sidebar=28 cols, top
		// progress strip = 2 rows) and let the active step react.
		if mm.X >= 28 {
			localX := mm.X - 28
			localY := mm.Y - 2
			if cmd := o.model.routeBodyClick(localX, localY); cmd != nil {
				return o, cmd
			}
		}
	}
	updated, cmd := o.model.Update(msg)
	o.model = updated
	return o, cmd
}

func (o *wizardOverlay) View() string {
	return o.model.View()
}

