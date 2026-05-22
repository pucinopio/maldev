package wizard

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui/core"
)

// BinaryBindingMsg is emitted when binary binding is confirmed or skipped.
type BinaryBindingMsg struct {
	SHA256 string // empty = skip
	Size   int64
}

// BinaryHashedMsg carries the result of hashing a file.
type BinaryHashedMsg struct {
	SHA256 string
	Size   int64
	Err    error
}

// StepBindingBinary is step 4: optional binary (SHA-256) binding.
type StepBindingBinary struct {
	pathIn  textinput.Model
	sha256  string
	size    int64
	hashing bool
	hashErr string
	focused bool
	bounds  core.Rect
}

// NewStepBindingBinary constructs step 4.
func NewStepBindingBinary() *StepBindingBinary {
	ti := textinput.New()
	ti.Placeholder = "path to binary (or drag-and-drop)…"
	ti.CharLimit = 512
	return &StepBindingBinary{pathIn: ti}
}

func (s *StepBindingBinary) Layout(b core.Rect) { s.bounds = b }
func (s *StepBindingBinary) Bounds() core.Rect  { return s.bounds }

func (s *StepBindingBinary) Update(msg tea.Msg) (core.Widget, tea.Cmd) {
	switch msg := msg.(type) {
	case BinaryHashedMsg:
		s.hashing = false
		if msg.Err != nil {
			s.hashErr = msg.Err.Error()
			return s, nil
		}
		s.sha256 = msg.SHA256
		s.size = msg.Size
		s.hashErr = ""
		return s, nil

	case tea.KeyMsg:
		if !s.focused {
			return s, nil
		}
		switch msg.String() {
		case "esc", "ctrl+s":
			return s, func() tea.Msg { return BinaryBindingMsg{} }
		case "s":
			if !s.pathIn.Focused() {
				return s, func() tea.Msg { return BinaryBindingMsg{} }
			}
		case "ctrl+f":
			// Bypass input focus so the operator can open the file picker
			// while typing in the path field.
			return s, func() tea.Msg { return OpenFilePickerMsg{Callback: "binary"} }
		case "enter":
			if s.sha256 != "" {
				sha, sz := s.sha256, s.size
				return s, func() tea.Msg { return BinaryBindingMsg{SHA256: sha, Size: sz} }
			}
			path := strings.TrimSpace(s.pathIn.Value())
			if path == "" {
				return s, nil
			}
			// Open the file picker instead of direct hashing — signals wizard.
			return s, func() tea.Msg { return OpenFilePickerMsg{Callback: "binary"} }
		case "f":
			if !s.pathIn.Focused() {
				return s, func() tea.Msg { return OpenFilePickerMsg{Callback: "binary"} }
			}
		}
		if s.pathIn.Focused() {
			updated, cmd := s.pathIn.Update(msg)
			s.pathIn = updated
			return s, cmd
		}
	}
	if s.pathIn.Focused() {
		updated, cmd := s.pathIn.Update(msg)
		s.pathIn = updated
		return s, cmd
	}
	return s, nil
}

// OpenFilePickerMsg asks the wizard to open the file picker overlay.
type OpenFilePickerMsg struct {
	// Callback identifies which step field should receive the picked path.
	Callback string
}

func (s *StepBindingBinary) View() string {
	fgDim := lipgloss.NewStyle().Foreground(core.Colors.FgDim)
	green := lipgloss.NewStyle().Foreground(core.Colors.Green)
	red := lipgloss.NewStyle().Foreground(core.Colors.Red)

	title := lipgloss.NewStyle().Foreground(core.Colors.Magenta).Bold(true).Render("Step 4 — Binary Binding (optional)")
	sub := fgDim.Render("Bind this licence to a specific binary by SHA-256 hash.")
	header := lipgloss.JoinVertical(lipgloss.Left, title, sub, "")

	var statusLine string
	switch {
	case s.hashing:
		statusLine = fgDim.Render("  computing SHA-256…")
	case s.hashErr != "":
		statusLine = red.Render("  error: " + s.hashErr)
	case s.sha256 != "":
		statusLine = green.Render("  SHA-256: " + s.sha256)
	default:
		statusLine = fgDim.Render("  no file selected")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		fgDim.Render("  Binary path:"),
		"  "+s.pathIn.View(),
		"",
		statusLine,
		"",
		renderHints("enter hash/confirm", "ctrl+f file picker", "ctrl+s/esc skip"),
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

// SetPath sets the path field (called when file picker returns a path).
func (s *StepBindingBinary) SetPath(path string) {
	s.pathIn.SetValue(path)
	s.sha256 = ""
	s.size = 0
	s.hashErr = ""
}

func (s *StepBindingBinary) Focus() {
	s.focused = true
	s.pathIn.Focus()
}
func (s *StepBindingBinary) Blur()         { s.focused = false; s.pathIn.Blur() }
func (s *StepBindingBinary) Focused() bool { return s.focused }
