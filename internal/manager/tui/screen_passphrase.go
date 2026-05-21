package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/tui/cmds"
)

const maxPassphraseAttempts = 3

// passphraseModel is the DB-unlock screen shown when the passphrase cascade
// did not resolve before the TUI launched.
type passphraseModel struct {
	store    *store.Store
	input    textinput.Model
	attempts int
	err      string
	done     bool
	result   string
	width    int
	hgt      int
}

// PassphraseResult carries the resolved passphrase out of a standalone
// tea.Program run (used by main.go for the sub-program flow).
type PassphraseResult struct {
	Passphrase string
}

// ResolvedPassphrase is implemented by models that can return a passphrase
// after a sub-program run completes. main.go uses this to extract the result
// from the tea.Model returned by tea.Program.Run.
type ResolvedPassphrase interface {
	ResolvedPassphrase() string
}

// ResolvedPassphrase implements ResolvedPassphrase on passphraseModel so
// main.go can do a single interface assertion after p.Run() returns.
func (m passphraseModel) ResolvedPassphrase() string { return m.result }

func newPassphraseModel() passphraseModel {
	ti := textinput.New()
	ti.Placeholder = "enter passphrase"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.Focus()
	return passphraseModel{input: ti}
}

// NewPassphrasePrompt builds a standalone tea.Model suitable for use as a
// sub-program from main.go when the passphrase cascade is unresolved.
func NewPassphrasePrompt(st *store.Store, _ string) passphraseModel {
	m := newPassphraseModel()
	m.store = st
	return m
}

func (m passphraseModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m passphraseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.update(msg)
	return updated, cmd
}

// update is the inner typed update so rootModel can call it without interface
// conversion.
func (m passphraseModel) update(msg tea.Msg) (passphraseModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.hgt = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			pass := m.input.Value()
			m.input.SetValue("")
			if m.store != nil {
				return m, cmds.TryUnlockCmd(m.store, pass)
			}
			// No store wired (test / onboarding path) — accept any non-empty input.
			if pass != "" {
				m.done = true
				m.result = pass
				return m, func() tea.Msg { return PassphraseResult{Passphrase: pass} }
			}
			m.err = "passphrase must not be empty"
			return m, nil

		case tea.KeyCtrlC:
			return m, tea.Quit
		}

	case cmds.UnlockResultMsg:
		if msg.Err != nil {
			m.err = fmt.Sprintf("error: %v", msg.Err)
			return m, nil
		}
		if msg.OK {
			m.done = true
			m.result = m.input.Value()
			return m, func() tea.Msg { return PassphraseResult{Passphrase: m.result} }
		}
		m.attempts++
		remaining := maxPassphraseAttempts - m.attempts
		if remaining <= 0 {
			m.err = "too many failed attempts"
			m.done = true
			return m, tea.Quit
		}
		m.err = fmt.Sprintf("wrong passphrase — %d attempt(s) remaining", remaining)
		return m, nil
	}

	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	return m, inputCmd
}

func (m passphraseModel) View() string {
	w := m.width
	h := m.hgt
	if w == 0 {
		w = 80
	}
	if h == 0 {
		h = 24
	}

	var lines []string
	lines = append(lines, GlowMagent.Render("license-manager"))
	lines = append(lines, "")
	lines = append(lines, Dim.Render("Enter the database passphrase to continue."))
	lines = append(lines, "")
	lines = append(lines, m.input.View())
	lines = append(lines, "")

	if m.err != "" {
		lines = append(lines, GlowRed.Render(m.err))
	} else {
		attemptsLeft := maxPassphraseAttempts - m.attempts
		lines = append(lines, Mute.Render(fmt.Sprintf("%d attempt(s) remaining", attemptsLeft)))
	}

	lines = append(lines, "")
	lines = append(lines,
		HintKey.Render("enter")+" "+HintText.Render("submit")+
			"  "+HintKey.Render("ctrl+c")+" "+HintText.Render("quit"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	box := BoxStyle.Width(50).Render(content)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}
