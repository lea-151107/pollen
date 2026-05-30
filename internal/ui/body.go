package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea-151107/pollen/internal/history"
)

var bodyTypes = []history.BodyType{history.BodyJSON, history.BodyForm, history.BodyRaw, history.BodyGraphQL, history.BodyMultipart}

type Body struct {
	tabIdx  int
	editors map[history.BodyType]textarea.Model
	// graphqlVars is the second editor shown when the GraphQL tab is
	// active; it holds the JSON variables payload. The "main" editor
	// in editors[BodyGraphQL] holds the query string.
	graphqlVars textarea.Model
	// graphqlVarsFocus selects which of the two GraphQL sub-editors is
	// receiving input while in editor mode. Ignored outside the
	// GraphQL tab.
	graphqlVarsFocus bool
	focused          bool
	tabFocus         bool // true: switching tabs, false: editing
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
		case history.BodyGraphQL:
			ta.Placeholder = "query { ... }"
		case history.BodyMultipart:
			ta.Placeholder = "name=value\nupload=@/path/to/file\nimg=@/path/x.png;type=image/png"
		}
		editors[t] = ta
	}
	gv := textarea.New()
	gv.ShowLineNumbers = false
	gv.Prompt = ""
	gv.Placeholder = `{ "var": "value" }`
	return Body{tabIdx: 0, editors: editors, graphqlVars: gv, tabFocus: true}
}

func (b Body) Type() history.BodyType { return bodyTypes[b.tabIdx] }

func (b Body) Value() string {
	ta := b.editors[b.Type()]
	return ta.Value()
}

// GraphQLVariables returns the contents of the second GraphQL sub-editor.
// Always safe to call; returns "" when the current body type isn't
// GraphQL or the variables field is empty.
func (b Body) GraphQLVariables() string {
	return b.graphqlVars.Value()
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
	// Reset the GraphQL variables editor too — callers that want to
	// restore it pass through SetGraphQLVariables after Set.
	b.graphqlVars.SetValue("")
	b.graphqlVarsFocus = false
}

// SetGraphQLVariables restores the variables sub-editor. Intended for
// applyEntry restoring a history.Request that carried
// GraphQLVariables.
func (b *Body) SetGraphQLVariables(s string) {
	b.graphqlVars.SetValue(s)
}

func (b *Body) Focus() {
	b.focused = true
	b.tabFocus = true
}

func (b *Body) Blur() {
	b.focused = false
	b.tabFocus = true
	b.graphqlVarsFocus = false
	for t, ta := range b.editors {
		ta.Blur()
		b.editors[t] = ta
	}
	b.graphqlVars.Blur()
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
			b.graphqlVarsFocus = false
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
		b.graphqlVars.Blur()
		b.graphqlVarsFocus = false
		b.tabFocus = true
		return b, nil
	}

	// Ctrl+G toggles between query and variables when the GraphQL tab
	// is active. Outside that tab it's a no-op (left to fall through to
	// the textarea, which ignores it).
	if ok && km.String() == "ctrl+g" && b.Type() == history.BodyGraphQL {
		if b.graphqlVarsFocus {
			b.graphqlVarsFocus = false
			b.graphqlVars.Blur()
			ta := b.editors[history.BodyGraphQL]
			ta.Focus()
			b.editors[history.BodyGraphQL] = ta
		} else {
			b.graphqlVarsFocus = true
			ta := b.editors[history.BodyGraphQL]
			ta.Blur()
			b.editors[history.BodyGraphQL] = ta
			b.graphqlVars.Focus()
		}
		return b, nil
	}

	// Tab inside the editor inserts two spaces (JSON-friendly indent), instead
	// of cycling focus. Esc → Tab is the path to leave the editor.
	if ok && km.String() == "tab" {
		if b.Type() == history.BodyGraphQL && b.graphqlVarsFocus {
			b.graphqlVars.InsertString("  ")
			return b, nil
		}
		ta := b.editors[b.Type()]
		ta.InsertString("  ")
		b.editors[b.Type()] = ta
		return b, nil
	}

	var cmd tea.Cmd
	if b.Type() == history.BodyGraphQL && b.graphqlVarsFocus {
		b.graphqlVars, cmd = b.graphqlVars.Update(msg)
		return b, cmd
	}
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
	selStyle := lipgloss.NewStyle().Padding(0, 1)
	for i, t := range bodyTypes {
		label := strings.ToUpper(string(t))
		s := lipgloss.NewStyle().Padding(0, 1)
		switch {
		case i == b.tabIdx && b.focused && b.tabFocus:
			s = s.Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0"))
			selStyle = selStyle.Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0"))
		case i == b.tabIdx:
			s = s.Background(lipgloss.Color("240")).Foreground(lipgloss.Color("230"))
			if !(b.focused && b.tabFocus) {
				selStyle = selStyle.Background(lipgloss.Color("240")).Foreground(lipgloss.Color("230"))
			}
		default:
			s = s.Foreground(lipgloss.Color("244"))
		}
		tabs[i] = s.Render(label)
	}
	tabBar := strings.Join(tabs, " ")
	// Collapse to ‹ {selected} › when the full strip would overflow and
	// wrap onto a second line, which would push the editor area below
	// out of the panel.
	if lipgloss.Width(tabBar) > inner {
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		selectedLabel := strings.ToUpper(string(bodyTypes[b.tabIdx]))
		tabBar = dim.Render("‹") + selStyle.Render(selectedLabel) + dim.Render("›")
	}

	hint := " "
	if b.focused {
		switch {
		case b.tabFocus:
			hint = "  ←/→: tab  ·  Enter: edit"
		case b.Type() == history.BodyGraphQL:
			hint = "  Tab: indent  ·  Ctrl+G: query↔variables  ·  Esc: leave editor"
		default:
			hint = "  Tab: indent  ·  Esc: leave editor"
		}
	}
	hintLine := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(hint)

	taH := innerH - 2 // -1 for tab bar, -1 for hint line
	if taH < 1 {
		taH = 1
	}

	if b.Type() == history.BodyGraphQL {
		// Split vertical space: query gets the larger top region,
		// variables the smaller bottom region. Two labels prefix each.
		queryH := taH * 2 / 3
		if queryH < 2 {
			queryH = 2
		}
		varsH := taH - queryH - 2 // -2 for the two labels
		if varsH < 2 {
			varsH = 2
		}
		ta := b.editors[history.BodyGraphQL]
		ta.SetWidth(inner - 2)
		ta.SetHeight(queryH)
		b.editors[history.BodyGraphQL] = ta
		b.graphqlVars.SetWidth(inner - 2)
		b.graphqlVars.SetHeight(varsH)
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		queryLabel := labelStyle.Render("query")
		varsLabel := labelStyle.Render("variables (JSON)")
		if b.focused && !b.tabFocus {
			if b.graphqlVarsFocus {
				varsLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("variables (JSON)")
			} else {
				queryLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("query")
			}
		}
		return border.Render(strings.Join([]string{
			tabBar,
			hintLine,
			queryLabel,
			ta.View(),
			varsLabel,
			b.graphqlVars.View(),
		}, "\n"))
	}

	ta := b.editors[b.Type()]
	ta.SetWidth(inner - 2) // -2 for left/right padding
	ta.SetHeight(taH)

	return border.Render(tabBar + "\n" + hintLine + "\n" + ta.View())
}
