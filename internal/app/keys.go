package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit       key.Binding
	NextFocus  key.Binding
	PrevFocus  key.Binding
	Send       key.Binding
	Copy       key.Binding
	ToggleHist key.Binding
	Cancel     key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit:       key.NewBinding(key.WithKeys("ctrl+c")),
		NextFocus:  key.NewBinding(key.WithKeys("tab")),
		PrevFocus:  key.NewBinding(key.WithKeys("shift+tab")),
		Send:       key.NewBinding(key.WithKeys("ctrl+s")),
		Copy:       key.NewBinding(key.WithKeys("ctrl+y")),
		ToggleHist: key.NewBinding(key.WithKeys("ctrl+h")),
		Cancel:     key.NewBinding(key.WithKeys("esc")),
	}
}
