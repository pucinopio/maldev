package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SelectResultMsg is emitted by selectOverlay when the operator picks an option.
// It shares the same ID/Value shape as InputResultMsg so dispatchOverlayResult
// can route it through the existing InputResultMsg case without a new switch arm.
type SelectResultMsg struct {
	ID    string
	Value string
}

// SelectOption is one item in a selectOverlay list.
type SelectOption struct {
	Label string // displayed text
	Value string // returned in SelectResultMsg.Value
}

// selectOverlay is a keyboard-navigable single-choice picker modal.
// Up/Down moves the cursor; Enter picks; Esc cancels. Clicking an item also selects.
type selectOverlay struct {
	id      string
	title   string
	options []SelectOption
	cursor  int
}

// newSelectOverlay constructs a select overlay. initialValue is matched against
// option Values to pre-position the cursor; if no match cursor starts at 0.
func newSelectOverlay(id, title string, options []SelectOption, initialValue string) *selectOverlay {
	cursor := 0
	for i, o := range options {
		if o.Value == initialValue {
			cursor = i
			break
		}
	}
	return &selectOverlay{id: id, title: title, options: options, cursor: cursor}
}

func (o *selectOverlay) Init() tea.Cmd { return nil }

func (o *selectOverlay) Update(msg tea.Msg) (Overlay, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "up", "k":
			if o.cursor > 0 {
				o.cursor--
			}
		case "down", "j":
			if o.cursor < len(o.options)-1 {
				o.cursor++
			}
		case "enter":
			if len(o.options) == 0 {
				return o, nil
			}
			id, val := o.id, o.options[o.cursor].Value
			return o, func() tea.Msg {
				return OverlayDoneMsg{Result: SelectResultMsg{ID: id, Value: val}}
			}
		case "esc":
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
	case tea.MouseMsg:
		if m.Button != tea.MouseButtonLeft || m.Action != tea.MouseActionPress {
			return o, nil
		}
		// Modal is 54×(6+len(options)) centred; items start at relative Y=3.
		itemStart := 3
		rel := m.Y - itemStart
		if rel >= 0 && rel < len(o.options) {
			id, val := o.id, o.options[rel].Value
			return o, func() tea.Msg {
				return OverlayDoneMsg{Result: SelectResultMsg{ID: id, Value: val}}
			}
		}
		// Footer Cancel button is at relative Y = itemStart + len + 1.
		footerY := itemStart + len(o.options) + 1
		if m.Y == footerY && m.X < 27 {
			return o, func() tea.Msg { return OverlayDoneMsg{Result: nil} }
		}
	}
	return o, nil
}

func (o *selectOverlay) View() string {
	const innerW = 48
	var lines []string
	for i, opt := range o.options {
		if i == o.cursor {
			lines = append(lines, GlowCyan.Render("▶ ")+Base.Bold(true).Render(opt.Label))
		} else {
			lines = append(lines, Dim.Render("  "+opt.Label))
		}
	}
	body := GlowMagent.Render(o.title) + "\n\n" +
		lipgloss.JoinVertical(lipgloss.Left, lines...) + "\n\n" +
		renderButtons(innerW,
			button{label: "Annuler", hotkey: "esc", kind: btnNeutral},
			button{label: "Choisir", hotkey: "↵", kind: btnPrimary, focused: true},
		)
	h := 6 + len(o.options)
	if h < 8 {
		h = 8
	}
	return lipgloss.Place(54, h, lipgloss.Center, lipgloss.Center, Modal.Render(body))
}
