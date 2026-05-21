package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap holds the global keybindings active in every view.
type KeyMap struct {
	Quit   key.Binding
	Help   key.Binding
	Tab1   key.Binding
	Tab2   key.Binding
	Tab3   key.Binding
	Tab4   key.Binding
	Tab5   key.Binding
	Tab6   key.Binding
	Tab7   key.Binding
	Tab8   key.Binding
	Tab9   key.Binding
}

func newKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Tab1: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "dashboard")),
		Tab2: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "licenses")),
		Tab3: key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "issuers")),
		Tab4: key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "recipients")),
		Tab5: key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "identities")),
		Tab6: key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "revocation")),
		Tab7: key.NewBinding(key.WithKeys("7"), key.WithHelp("7", "servers")),
		Tab8: key.NewBinding(key.WithKeys("8"), key.WithHelp("8", "audit")),
		Tab9: key.NewBinding(key.WithKeys("9"), key.WithHelp("9", "settings")),
	}
}
