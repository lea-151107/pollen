package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"

	"github.com/lea-151107/pollen/internal/ui"
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

// HelpSections returns the canonical help content. Global section is derived
// from KeyMap so adding a new global binding only needs updating one place.
// Panel-specific sections are still listed inline because they live in
// individual ui/* components. HelpItem / HelpSection live in package ui
// alongside the Help overlay component.
func (k KeyMap) HelpSections() []ui.HelpSection {
	return []ui.HelpSection{
		{
			Title: "Global",
			Items: []ui.HelpItem{
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
		{Title: "History", Items: []ui.HelpItem{
			{Keys: "↑/↓", Desc: "Move"},
			{Keys: "G", Desc: "Jump to last"},
			{Keys: "gg", Desc: "Jump to first"},
			{Keys: "Enter", Desc: "Load entry"},
			{Keys: "d", Desc: "Delete entry"},
			{Keys: "/", Desc: "Filter (Esc clears)"},
		}},
		{Title: "Collections", Items: []ui.HelpItem{
			{Keys: "↑/↓", Desc: "Move"},
			{Keys: "G", Desc: "Jump to last"},
			{Keys: "gg", Desc: "Jump to first"},
			{Keys: "Enter", Desc: "Load entry"},
			{Keys: "e", Desc: "Rename entry"},
			{Keys: "d", Desc: "Delete entry"},
			{Keys: "/", Desc: "Filter (Esc clears)"},
		}},
		{Title: "Method", Items: []ui.HelpItem{
			{Keys: "↑/↓", Desc: "Cycle methods"},
		}},
		{Title: "Query", Items: []ui.HelpItem{
			{Keys: "↑/↓ ←/→", Desc: "Navigate rows / fields"},
			{Keys: "Enter", Desc: "New row"},
			{Keys: "Ctrl+D", Desc: "Delete row"},
		}},
		{Title: "Auth", Items: []ui.HelpItem{
			{Keys: "←/→", Desc: "Select type (None / Bearer / Basic / OAuth / OAuth AC)"},
			{Keys: "Enter/↓", Desc: "Edit fields"},
			{Keys: "↓ / ↑", Desc: "Move between fields"},
			{Keys: "Esc/↑", Desc: "Back to type selector"},
			{Keys: "g", Desc: "OAuth: fetch (CC) / authorize (AC) / refresh"},
			{Keys: "Esc on action row", Desc: "OAuth AC: cancel in-flight authorization"},
		}},
		{Title: "Dynamic variables", Items: []ui.HelpItem{
			{Keys: "{{$timestamp}}", Desc: "Unix epoch seconds (fresh per request)"},
			{Keys: "{{$timestamp_ms}}", Desc: "Unix epoch milliseconds"},
			{Keys: "{{$datetime}}", Desc: "RFC3339 UTC timestamp"},
			{Keys: "{{$uuid}}", Desc: "UUID v4"},
			{Keys: "{{$random}}", Desc: "random uint32"},
			{Keys: "{{$random:N}}", Desc: "random 0..N-1"},
			{Keys: "{{$random:M-N}}", Desc: "random M..N inclusive"},
			{Keys: "{{$base64:VALUE}}", Desc: "base64-encode VALUE"},
			{Keys: "{{$urlencode:VALUE}}", Desc: "URL-encode VALUE"},
		}},
		{Title: "Headers", Items: []ui.HelpItem{
			{Keys: "↑/↓ ←/→", Desc: "Navigate rows / fields"},
			{Keys: "Enter", Desc: "New row"},
			{Keys: "Ctrl+D", Desc: "Delete row"},
			{Keys: "Tab", Desc: "Accept suggestion"},
		}},
		{Title: "Body", Items: []ui.HelpItem{
			{Keys: "←/→", Desc: "Switch tab (JSON / Form / Raw / GraphQL / Multipart)"},
			{Keys: "Enter", Desc: "Enter editor"},
			{Keys: "Tab", Desc: "Indent (in editor)"},
			{Keys: "Esc", Desc: "Leave editor"},
			{Keys: "Ctrl+G", Desc: "Toggle GraphQL query ↔ variables pane"},
			{Keys: "name=@path", Desc: "Multipart file upload (optional ;type=ct)"},
		}},
		{Title: "Response", Items: []ui.HelpItem{
			{Keys: "↑/↓ PgUp/PgDn", Desc: "Scroll"},
			{Keys: "s", Desc: "Save response"},
			{Keys: "y", Desc: "Copy body to clipboard"},
			{Keys: "/", Desc: "jq filter"},
			{Keys: "Ctrl+F", Desc: "Search in body"},
			{Keys: "D", Desc: "Toggle diff vs previous"},
		}},
		{Title: "Intruder — markers", Items: []ui.HelpItem{
			{Keys: "{{$payload}}", Desc: "Position 1 marker (alias of {{$payload1}}); use in Sniper"},
			{Keys: "{{$payload1..N}}", Desc: "Numbered marker for Pitchfork / ClusterBomb (N up to 8)"},
		}},
		{Title: "Intruder — config modal", Items: []ui.HelpItem{
			{Keys: "←/→ on Mode", Desc: "Switch attack mode: Sniper / Pitchfork / ClusterBomb"},
			{Keys: "←/→ on Positions", Desc: "Adjust position count (Pitchfork / ClusterBomb only)"},
			{Keys: "←/→ on Payload kind", Desc: "Switch generator: Range / List / Brute / CaseToggle"},
			{Keys: "Tab / Shift+Tab", Desc: "Move between fields"},
			{Keys: "Enter", Desc: "Start run"},
			{Keys: "Esc", Desc: "Cancel modal"},
		}},
		{Title: "Intruder — results table", Items: []ui.HelpItem{
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
		{Title: "Intruder — result detail", Items: []ui.HelpItem{
			{Keys: "↑/↓ PgUp/PgDn", Desc: "Scroll response body"},
			{Keys: "g", Desc: "Jump to top"},
			{Keys: "Esc", Desc: "Back to results table"},
		}},
		{Title: "Chaining", Items: []ui.HelpItem{
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
