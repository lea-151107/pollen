package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit       key.Binding
	NextFocus  key.Binding
	PrevFocus  key.Binding
	Send       key.Binding
	Copy       key.Binding
	ToggleHist key.Binding
	ToggleTLS  key.Binding
	SwitchEnv  key.Binding
	Help       key.Binding
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
		ToggleTLS:  key.NewBinding(key.WithKeys("ctrl+t")),
		SwitchEnv:  key.NewBinding(key.WithKeys("ctrl+e")),
		// Ctrl+/ produces ASCII 0x1f (US) on most terminals, which bubbletea
		// reports as "ctrl+_". Modern terminals may also report "ctrl+/" via
		// the CSI-u protocol — bind both so either works.
		Help:   key.NewBinding(key.WithKeys("ctrl+/", "ctrl+_")),
		Cancel: key.NewBinding(key.WithKeys("esc")),
	}
}

// HelpItem is one row in the help overlay.
type HelpItem struct {
	Keys string
	Desc string
}

// HelpSection groups related HelpItems under a title.
type HelpSection struct {
	Title string
	Items []HelpItem
}

// HelpSections returns the canonical help content. Global section is derived
// from KeyMap so adding a new global binding only needs updating one place.
// Panel-specific sections are still listed inline because they live in
// individual ui/* components.
func (k KeyMap) HelpSections() []HelpSection {
	return []HelpSection{
		{
			Title: "Global",
			Items: []HelpItem{
				{Keys: "Tab / Shift+Tab", Desc: "Move focus"},
				{Keys: bindingKeys(k.Send), Desc: "Send request"},
				{Keys: bindingKeys(k.Copy), Desc: "Copy as cURL / fetch"},
				{Keys: bindingKeys(k.ToggleHist), Desc: "Toggle history panel"},
				{Keys: bindingKeys(k.ToggleTLS), Desc: "Toggle TLS verification skip"},
				{Keys: bindingKeys(k.SwitchEnv), Desc: "Switch variable environment"},
				{Keys: bindingKeys(k.Quit), Desc: "Quit"},
				{Keys: bindingKeys(k.Help), Desc: "This help"},
				{Keys: "u", Desc: "Undo last history delete"},
			},
		},
		{Title: "History", Items: []HelpItem{
			{Keys: "↑/↓", Desc: "Move"},
			{Keys: "Enter", Desc: "Load entry"},
			{Keys: "d", Desc: "Delete entry"},
			{Keys: "/", Desc: "Filter (Esc clears)"},
		}},
		{Title: "Method", Items: []HelpItem{
			{Keys: "↑/↓", Desc: "Cycle methods"},
		}},
		{Title: "Query", Items: []HelpItem{
			{Keys: "↑/↓ ←/→", Desc: "Navigate rows / fields"},
			{Keys: "Enter", Desc: "New row"},
			{Keys: "Ctrl+D", Desc: "Delete row"},
		}},
		{Title: "Auth", Items: []HelpItem{
			{Keys: "←/→", Desc: "Select type (None/Bearer/Basic)"},
			{Keys: "Enter/↓", Desc: "Edit fields"},
			{Keys: "Esc/↑", Desc: "Back to type selector"},
		}},
		{Title: "Headers", Items: []HelpItem{
			{Keys: "↑/↓ ←/→", Desc: "Navigate rows / fields"},
			{Keys: "Enter", Desc: "New row"},
			{Keys: "Ctrl+D", Desc: "Delete row"},
			{Keys: "Tab", Desc: "Accept suggestion"},
		}},
		{Title: "Body", Items: []HelpItem{
			{Keys: "←/→", Desc: "Switch tab"},
			{Keys: "Enter", Desc: "Enter editor"},
			{Keys: "Tab", Desc: "Indent (in editor)"},
			{Keys: "Esc", Desc: "Leave editor"},
		}},
		{Title: "Response", Items: []HelpItem{
			{Keys: "↑/↓ PgUp/PgDn", Desc: "Scroll"},
			{Keys: "s", Desc: "Save response"},
		}},
	}
}

// bindingKeys returns the printable keys of a binding (e.g. "ctrl+s") in a
// form suitable for help display.
func bindingKeys(b key.Binding) string {
	keys := b.Keys()
	if len(keys) == 0 {
		return ""
	}
	out := keys[0]
	for _, k := range keys[1:] {
		out += " / " + k
	}
	return out
}
