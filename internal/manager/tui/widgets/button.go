package widgets

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/oioio-space/maldev/internal/manager/tui"
)

// Button is a labeled interactive widget that fires OnPress when clicked.
// It implements tui.Clickable and tui.Focusable.
type Button struct {
	Label   string
	Hotkey  string
	OnPress func() tea.Cmd
	focused bool
	bounds  tui.Rect
	style   lipgloss.Style
}

// NewButton constructs a Button.
func NewButton(label, hotkey string, onPress func() tea.Cmd) *Button {
	return &Button{
		Label:   label,
		Hotkey:  hotkey,
		OnPress: onPress,
		style:   lipgloss.NewStyle().Padding(0, 2).Border(lipgloss.NormalBorder()),
	}
}

func (b *Button) Layout(bounds tui.Rect) { b.bounds = bounds }
func (b *Button) Bounds() tui.Rect      { return b.bounds }

func (b *Button) Update(msg tea.Msg) (tui.Widget, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		if b.focused && b.Hotkey != "" && km.String() == b.Hotkey {
			if b.OnPress != nil {
				return b, b.OnPress()
			}
		}
	}
	return b, nil
}

func (b *Button) View() string {
	label := b.Label
	if b.Hotkey != "" {
		label = "[" + b.Hotkey + "] " + label
	}
	st := b.style
	if b.focused {
		st = st.BorderForeground(lipgloss.Color("#ff36d4"))
	}
	return st.Render(label)
}

// OnClick implements tui.Clickable.
func (b *Button) OnClick(_, _ int, _ tea.MouseButton) tea.Cmd {
	if b.OnPress != nil {
		return b.OnPress()
	}
	return nil
}

// Focus implements tui.Focusable.
func (b *Button) Focus()        { b.focused = true }
func (b *Button) Blur()         { b.focused = false }
func (b *Button) Focused() bool { return b.focused }
