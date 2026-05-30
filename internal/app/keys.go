package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
	Quit        key.Binding
	NextFocus   key.Binding
	PrevFocus   key.Binding
	Send        key.Binding
	Copy        key.Binding
	ToggleHist  key.Binding
	ToggleColl  key.Binding
	SaveToColl  key.Binding
	ImportFile  key.Binding
	ToggleTLS   key.Binding
	SwitchEnv   key.Binding
	Intruder    key.Binding
	Help        key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit:       key.NewBinding(key.WithKeys("ctrl+c")),
		NextFocus:  key.NewBinding(key.WithKeys("tab")),
		PrevFocus:  key.NewBinding(key.WithKeys("shift+tab")),
		Send:       key.NewBinding(key.WithKeys("ctrl+s")),
		Copy:       key.NewBinding(key.WithKeys("ctrl+y")),
		ToggleHist: key.NewBinding(key.WithKeys("ctrl+h")),
		ToggleColl: key.NewBinding(key.WithKeys("ctrl+k")),
		SaveToColl: key.NewBinding(key.WithKeys("ctrl+b")),
		ImportFile: key.NewBinding(key.WithKeys("ctrl+i")),
		ToggleTLS:  key.NewBinding(key.WithKeys("ctrl+t")),
		SwitchEnv:  key.NewBinding(key.WithKeys("ctrl+e")),
		Intruder:   key.NewBinding(key.WithKeys("ctrl+r")),
		// Ctrl+/ produces ASCII 0x1f (US) on most terminals, which bubbletea
		// reports as "ctrl+_". Modern terminals may also report "ctrl+/" via
		// the CSI-u protocol — bind both so either works.
		Help: key.NewBinding(key.WithKeys("ctrl+/", "ctrl+_")),
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
				{Keys: bindingKeys(k.ToggleColl), Desc: "Toggle collections panel"},
				{Keys: bindingKeys(k.SaveToColl), Desc: "Save request to collection"},
				{Keys: bindingKeys(k.ImportFile), Desc: "Import OpenAPI / Postman file"},
				{Keys: bindingKeys(k.ToggleTLS), Desc: "Toggle TLS verification skip"},
				{Keys: bindingKeys(k.SwitchEnv), Desc: "Switch variable environment"},
				{Keys: bindingKeys(k.Intruder), Desc: "Open Intruder (concurrent requests)"},
				{Keys: bindingKeys(k.Quit), Desc: "Quit"},
				{Keys: bindingKeys(k.Help), Desc: "This help"},
				{Keys: "u", Desc: "Undo last history delete"},
				{Keys: "Ctrl+L", Desc: "Redraw screen"},
			},
		},
		{Title: "History", Items: []HelpItem{
			{Keys: "↑/↓", Desc: "Move"},
			{Keys: "G", Desc: "Jump to last"},
			{Keys: "gg", Desc: "Jump to first"},
			{Keys: "Enter", Desc: "Load entry"},
			{Keys: "d", Desc: "Delete entry"},
			{Keys: "/", Desc: "Filter (Esc clears)"},
		}},
		{Title: "Collections", Items: []HelpItem{
			{Keys: "↑/↓", Desc: "Move"},
			{Keys: "G", Desc: "Jump to last"},
			{Keys: "gg", Desc: "Jump to first"},
			{Keys: "Enter", Desc: "Load entry"},
			{Keys: "e", Desc: "Rename entry"},
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
			{Keys: "←/→", Desc: "Switch tab (JSON / Form / Raw / GraphQL)"},
			{Keys: "Enter", Desc: "Enter editor"},
			{Keys: "Tab", Desc: "Indent (in editor)"},
			{Keys: "Esc", Desc: "Leave editor"},
			{Keys: "Ctrl+G", Desc: "Toggle GraphQL query ↔ variables pane"},
		}},
		{Title: "Response", Items: []HelpItem{
			{Keys: "↑/↓ PgUp/PgDn", Desc: "Scroll"},
			{Keys: "s", Desc: "Save response"},
			{Keys: "y", Desc: "Copy body to clipboard"},
			{Keys: "/", Desc: "jq filter"},
			{Keys: "Ctrl+F", Desc: "Search in body"},
			{Keys: "D", Desc: "Toggle diff vs previous"},
		}},
		{Title: "Intruder — markers", Items: []HelpItem{
			{Keys: "{{$payload}}", Desc: "Position 1 marker (alias of {{$payload1}}); use in Sniper"},
			{Keys: "{{$payload1..N}}", Desc: "Numbered marker for Pitchfork / ClusterBomb (N up to 8)"},
		}},
		{Title: "Intruder — config modal", Items: []HelpItem{
			{Keys: "←/→ on Mode", Desc: "Switch attack mode: Sniper / Pitchfork / ClusterBomb"},
			{Keys: "←/→ on Positions", Desc: "Adjust position count (Pitchfork / ClusterBomb only)"},
			{Keys: "←/→ on Payload kind", Desc: "Switch generator: Range / List / Brute / CaseToggle"},
			{Keys: "Tab / Shift+Tab", Desc: "Move between fields"},
			{Keys: "Enter", Desc: "Start run"},
			{Keys: "Esc", Desc: "Cancel modal"},
		}},
		{Title: "Intruder — results table", Items: []HelpItem{
			{Keys: "↑/↓ PgUp/PgDn", Desc: "Move cursor (window follows)"},
			{Keys: "g / G", Desc: "Jump to first / last row"},
			{Keys: "Enter", Desc: "Open detail view for focused row"},
			{Keys: "s", Desc: "Cycle sort column (# → status → size → ms → #)"},
			{Keys: "S", Desc: "Reverse sort direction on the current column"},
			{Keys: "/", Desc: "Open filter prompt (DSL: payload substring; size:>N, dur:>N, s:4xx, …)"},
			{Keys: "f", Desc: "Cycle status preset: All → Errors → 2xx → All"},
			{Keys: "e", Desc: "Export current results to CSV (path prompt)"},
			{Keys: "Esc", Desc: "Clear filter, or (no filter) cancel run + close overlay"},
		}},
		{Title: "Intruder — result detail", Items: []HelpItem{
			{Keys: "↑/↓ PgUp/PgDn", Desc: "Scroll response body"},
			{Keys: "g", Desc: "Jump to top"},
			{Keys: "Esc", Desc: "Back to results table"},
		}},
		{Title: "Chaining", Items: []HelpItem{
			{Keys: "{{response.body.<path>}}", Desc: "Value from last response (jq)"},
			{Keys: "{{response.headers.<n>}}", Desc: "Response header value"},
			{Keys: "{{response.status}}", Desc: "HTTP status code"},
		}},
	}
}

// bindingKeys returns the printable keys of a binding in Title-Case form
// (e.g. "Ctrl+S") so the help overlay matches the status bar's casing.
func bindingKeys(b key.Binding) string {
	keys := b.Keys()
	if len(keys) == 0 {
		return ""
	}
	out := formatKey(keys[0])
	for _, k := range keys[1:] {
		out += " / " + formatKey(k)
	}
	return out
}

// formatKey converts bubbles/key's lowercase token (e.g. "ctrl+s", "tab",
// "shift+tab") to display casing. Single-character segments are uppercased
// (so the modifier suffix in "ctrl+s" becomes "S") and multi-character
// segments are Title-Cased ("ctrl" → "Ctrl", "tab" → "Tab"). Lone literal
// keys like "d" or "/" pass through unchanged.
func formatKey(s string) string {
	parts := strings.Split(s, "+")
	if len(parts) == 1 {
		switch strings.ToLower(s) {
		case "tab", "enter", "esc", "shift", "ctrl", "alt", "meta", "space":
			return strings.ToUpper(s[:1]) + s[1:]
		}
		return s
	}
	for i, p := range parts {
		if len(p) == 1 {
			parts[i] = strings.ToUpper(p)
		} else if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "+")
}
