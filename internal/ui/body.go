package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea-151107/pollen/internal/history"
)

var bodyTypes = []history.BodyType{history.BodyJSON, history.BodyForm, history.BodyRaw}

type Body struct {
	tabIdx   int
	editors  map[history.BodyType]textarea.Model
	focused  bool
	tabFocus bool // true: switching tabs, false: editing
}

func NewBody() Body {
	editors := map[history.BodyType]textarea.Model{}
	for _, t := range bodyTypes {
		ta := textarea.New()
		ta.ShowLineNumbers = false
		ta.Prompt = ""
		switch t {
		case history.BodyJSON:
			ta.Placeholder = `{ "key": "value" }`
		case history.BodyForm:
			ta.Placeholder = "key=value\nfoo=bar"
		case history.BodyRaw:
			ta.Placeholder = "raw text body"
		}
		editors[t] = ta
	}
	return Body{tabIdx: 0, editors: editors, tabFocus: true}
}

func (b Body) Type() history.BodyType { return bodyTypes[b.tabIdx] }

func (b Body) Value() string {
	ta := b.editors[b.Type()]
	return ta.Value()
}

func (b *Body) Set(bodyType history.BodyType, value string) {
	for i, t := range bodyTypes {
		if t == bodyType {
			b.tabIdx = i
		}
	}
	// Reset every editor so unrelated tabs do not retain stale content from a
	// previous request restored from history.
	for t, ta := range b.editors {
		if t == bodyType {
			ta.SetValue(value)
		} else {
			ta.SetValue("")
		}
		b.editors[t] = ta
	}
}

func (b *Body) Focus() {
	b.focused = true
	b.tabFocus = true
}

func (b *Body) Blur() {
	b.focused = false
	b.tabFocus = true
	for t, ta := range b.editors {
		ta.Blur()
		b.editors[t] = ta
	}
}

func (b Body) Focused() bool { return b.focused }

// InEditorMode is true when the user is typing inside the textarea (as opposed
// to navigating the JSON/Form/Raw tab selector). Used by the parent model to
// route Tab to the editor for indentation instead of cycling focus.
func (b Body) InEditorMode() bool { return b.focused && !b.tabFocus }

func (b Body) Update(msg tea.Msg) (Body, tea.Cmd) {
	if !b.focused {
		return b, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok && b.tabFocus {
		return b, nil
	}

	if b.tabFocus {
		switch {
		case key.Matches(km, key.NewBinding(key.WithKeys("left", "h"))):
			b.tabIdx = (b.tabIdx - 1 + len(bodyTypes)) % len(bodyTypes)
			return b, nil
		case key.Matches(km, key.NewBinding(key.WithKeys("right", "l"))):
			b.tabIdx = (b.tabIdx + 1) % len(bodyTypes)
			return b, nil
		case key.Matches(km, key.NewBinding(key.WithKeys("enter", "i", "down"))):
			b.tabFocus = false
			ta := b.editors[b.Type()]
			ta.Focus()
			b.editors[b.Type()] = ta
			return b, nil
		}
		return b, nil
	}

	// In editor mode.
	if ok && km.String() == "esc" {
		ta := b.editors[b.Type()]
		ta.Blur()
		b.editors[b.Type()] = ta
		b.tabFocus = true
		return b, nil
	}

	// Tab inside the editor inserts two spaces (JSON-friendly indent), instead
	// of cycling focus. Esc → Tab is the path to leave the editor.
	if ok && km.String() == "tab" {
		ta := b.editors[b.Type()]
		ta.InsertString("  ")
		b.editors[b.Type()] = ta
		return b, nil
	}

	var cmd tea.Cmd
	ta := b.editors[b.Type()]
	ta, cmd = ta.Update(msg)
	b.editors[b.Type()] = ta
	return b, cmd
}

func (b Body) View(width, height int) string {
	inner := width - 2
	if inner < 1 {
		inner = 1
	}
	innerH := height - 2
	if innerH < 1 {
		innerH = 1
	}
	border := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		Width(inner)
	if b.focused {
		border = border.BorderForeground(lipgloss.Color("205"))
	} else {
		border = border.BorderForeground(lipgloss.Color("240"))
	}

	tabs := make([]string, len(bodyTypes))
	for i, t := range bodyTypes {
		label := strings.ToUpper(string(t))
		s := lipgloss.NewStyle().Padding(0, 1)
		switch {
		case i == b.tabIdx && b.focused && b.tabFocus:
			s = s.Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0"))
		case i == b.tabIdx:
			s = s.Background(lipgloss.Color("240")).Foreground(lipgloss.Color("230"))
		default:
			s = s.Foreground(lipgloss.Color("244"))
		}
		tabs[i] = s.Render(label)
	}
	tabBar := strings.Join(tabs, " ")

	hint := " "
	if b.focused {
		if b.tabFocus {
			hint = "  ←/→ tab  •  Enter to edit"
		} else {
			hint = "  Tab: indent  •  Esc: leave editor"
		}
	}
	hintLine := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(hint)

	ta := b.editors[b.Type()]
	ta.SetWidth(inner - 2) // -2 for left/right padding
	taH := innerH - 2      // -1 for tab bar, -1 for hint line
	if taH < 1 {
		taH = 1
	}
	ta.SetHeight(taH)

	return border.Render(tabBar + "\n" + hintLine + "\n" + ta.View())
}
