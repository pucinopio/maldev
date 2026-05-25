package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// probeDrawerState tracks progress of the probe flow.
type probeDrawerState int

const (
	probeStateIssuing  probeDrawerState = iota // generating token
	probeStateWaiting                          // token shown, waiting for agent
	probeStateReceived                         // agent result arrived
	probeStateError                            // error
)

// ProbeTokenIssuedMsg carries the newly created probe token.
type ProbeTokenIssuedMsg struct {
	Token *ent.ProbeToken
	Err   error
}

// ProbeAgentResultMsg carries the consumed token with agent data.
type ProbeAgentResultMsg struct {
	Token *ent.ProbeToken
}

// probeDrawerOverlay is a right-edge slide-in drawer that issues a probe token,
// shows the one-line curl command, and waits for the agent callback.
type probeDrawerOverlay struct {
	svc      *service.Services
	state    probeDrawerState
	token    *ent.ProbeToken
	errMsg   string
	onResult func(machineID string) tea.Cmd // called when agent reports in
	width    int
}

const probeDrawerWidth = 60

// newProbeDrawerOverlay constructs the probe drawer.
// onResult is called with the resolved CompositeHex machine-ID.
func newProbeDrawerOverlay(svc *service.Services, onResult func(string) tea.Cmd) *probeDrawerOverlay {
	return &probeDrawerOverlay{svc: svc, onResult: onResult}
}

func (o *probeDrawerOverlay) Init() tea.Cmd {
	return o.issueTokenCmd()
}

func (o *probeDrawerOverlay) issueTokenCmd() tea.Cmd {
	svc := o.svc
	return func() tea.Msg {
		if svc == nil {
			return ProbeTokenIssuedMsg{Err: fmt.Errorf("services unavailable")}
		}
		tok, err := svc.Probe.NewToken(
			context.Background(),
			"wizard-probe",
			24*time.Hour,
			"operator",
		)
		return ProbeTokenIssuedMsg{Token: tok, Err: err}
	}
}

func (o *probeDrawerOverlay) subscribeCmd(tokenID string) tea.Cmd {
	svc := o.svc
	return func() tea.Msg {
		ch := svc.Probe.Subscribe(tokenID)
		tok, ok := <-ch
		if !ok || tok == nil {
			return ProbeAgentResultMsg{}
		}
		return ProbeAgentResultMsg{Token: tok}
	}
}

func (o *probeDrawerOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	switch msg := msg.(type) {
	case ProbeTokenIssuedMsg:
		if msg.Err != nil {
			o.state = probeStateError
			o.errMsg = msg.Err.Error()
			return o, nil
		}
		o.token = msg.Token
		o.state = probeStateWaiting
		return o, o.subscribeCmd(msg.Token.ID)

	case ProbeAgentResultMsg:
		if msg.Token == nil {
			o.state = probeStateError
			o.errMsg = "probe subscription closed without result"
			return o, nil
		}
		o.token = msg.Token
		o.state = probeStateReceived
		return o, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "c":
			if o.state == probeStateWaiting && o.token != nil {
				_ = clipboard.WriteAll(o.curlCommand())
			}
			return o, nil
		case "enter":
			if o.state == probeStateReceived && o.token != nil {
				machineID := o.token.CompositeHex
				// Return both the done signal and the onResult cmd together.
				var resultCmd tea.Cmd
				if o.onResult != nil {
					resultCmd = o.onResult(machineID)
				}
				return o, tea.Batch(
					resultCmd,
					func() tea.Msg { return OverlayDoneMsg{Result: nil} },
				)
			}
		case "esc":
			if o.token != nil && o.state == probeStateWaiting {
				svc := o.svc
				id := o.token.ID
				go func() { _ = svc.Probe.Revoke(context.Background(), id, "operator") }()
			}
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
	}
	return o, nil
}

func (o *probeDrawerOverlay) curlCommand() string {
	if o.token == nil {
		return ""
	}
	return fmt.Sprintf("curl -sf https://localhost:8080/probe/%s | sh", o.token.ID)
}

func (o *probeDrawerOverlay) View() string {
	// Non-bold variants of the Glow* theme styles — same colors, no emphasis,
	// so the overlay stays calm against bordered status pills.
	green := GlowGreen.UnsetBold()
	red := GlowRed.UnsetBold()
	yellow := GlowYellow.UnsetBold()

	// Prototype: cyan "◆ FINGERPRINT PROBE" title with phase dot + status.
	var phaseDot, phaseStatus string
	switch o.state {
	case probeStateIssuing:
		phaseDot = yellow.Render("●")
		phaseStatus = Dim.Render("génération du token…")
	case probeStateWaiting:
		phaseDot = yellow.Render("●")
		phaseStatus = Dim.Render("subscribe channel · en attente")
	case probeStateReceived:
		phaseDot = green.Render("●")
		phaseStatus = green.Render("fingerprint reçu")
	case probeStateError:
		phaseDot = red.Render("●")
		phaseStatus = red.Render("erreur")
	}

	titleBar := lipgloss.JoinHorizontal(lipgloss.Top,
		GlowCyan.Render("◆ FINGERPRINT PROBE"),
		" ", phaseDot, " ", phaseStatus,
		"  ", Dim.Render("[esc] fermer"),
	)
	rule := lipgloss.NewStyle().Foreground(Palette.Cyan).Render(
		strings.Repeat("─", probeDrawerWidth),
	)

	var bodyLines []string
	switch o.state {
	case probeStateIssuing:
		bodyLines = []string{Dim.Render("  génération du token en cours…")}

	case probeStateWaiting:
		curl := o.curlCommand()
		bodyLines = []string{
			Dim.Render("URL à donner au client distant :"),
			"",
			GlowCyan.Render("  " + curl),
			"",
			Dim.Render("Le token expire dans 24 h."),
			"",
			HintKey.Render("c") + HintText.Render(" copier   ") +
				HintKey.Render("esc") + HintText.Render(" annuler"),
		}

	case probeStateReceived:
		tok := o.token
		bodyLines = []string{
			green.Render("✓ Fingerprint reçu"),
			"",
			Dim.Render("hostname   ") + Base.Render(tok.Hostname),
			Dim.Render("OS / arch  ") + Base.Render(fmt.Sprintf("%s / %s", tok.Os, tok.Arch)),
			Dim.Render("machine-id ") + GlowCyan.Render(tok.CompositeHex),
			"",
			HintKey.Render("↵") + HintText.Render(" utiliser cet ID   ") +
				HintKey.Render("esc") + HintText.Render(" ignorer"),
		}

	case probeStateError:
		bodyLines = []string{
			red.Render("✗ Erreur : " + o.errMsg),
			"",
			HintKey.Render("esc") + HintText.Render(" fermer"),
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		append([]string{titleBar, rule, ""}, bodyLines...)...,
	)
	return Modal.Width(probeDrawerWidth).Render(content)
}
