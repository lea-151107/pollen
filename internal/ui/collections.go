package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea/pollen/internal/collections"
)

type Collections struct {
	entries    []collections.Entry
	selected   int
	focused    bool
	filter     string
	filterMode bool
}

func NewCollections() Collections { return Collections{} }

func (c *Collections) SetEntries(entries []collections.Entry) {
	c.entries = entries
	fe := c.filtered()
	switch {
	case len(fe) == 0:
		c.selected = 0
	case c.selected >= len(fe):
		c.selected = len(fe) - 1
	case c.selected < 0:
		c.selected = 0
	}
}

func (c Collections) filtered() []collections.Entry {
	if c.filter == "" {
		return c.entries
	}
	needle := strings.ToLower(c.filter)
	var out []collections.Entry
	for _, e := range c.entries {
		hay := strings.ToLower(e.Name + " " + e.Request.Method + " " + e.Request.URL)
		if strings.Contains(hay, needle) {
			out = append(out, e)
		}
	}
	return out
}

func (c Collections) Selected() *collections.Entry {
	fe := c.filtered()
	if c.selected < 0 || c.selected >= len(fe) {
		return nil
	}
	return &fe[c.selected]
}

// SetFilter pre-sets the filter text (used at startup for --collection flag).
func (c *Collections) SetFilter(f string) {
	c.filter = f
	c.filterMode = false
	c.selected = 0
}

func (c *Collections) Focus() { c.focused = true }
func (c *Collections) Blur() {
	c.focused = false
	c.filterMode = false
}
func (c Collections) Focused() bool    { return c.focused }
func (c Collections) InFilterMode() bool { return c.filterMode }

type CollectionSelectMsg struct{ Entry collections.Entry }
type CollectionDeleteMsg struct{ ID string }
type CollectionRenameMsg struct{ ID string }

func (c Collections) Update(msg tea.Msg) (Collections, tea.Cmd) {
	if !c.focused {
		return c, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return c, nil
	}

	if c.filterMode {
		switch km.String() {
		case "esc":
			c.filterMode = false
			c.filter = ""
			c.selected = 0
		case "enter":
			c.filterMode = false
		case "backspace", "ctrl+h":
			if len(c.filter) > 0 {
				c.filter = c.filter[:len(c.filter)-1]
				c.selected = 0
			}
		default:
			s := km.String()
			if len(s) >= 1 && s[0] >= ' ' && s != "tab" {
				c.filter += s
				c.selected = 0
			}
		}
		return c, nil
	}

	switch {
	case km.String() == "/":
		c.filterMode = true
	case km.String() == "esc" && c.filter != "":
		c.filter = ""
		c.selected = 0
	case key.Matches(km, key.NewBinding(key.WithKeys("up", "k"))):
		if c.selected > 0 {
			c.selected--
		}
	case key.Matches(km, key.NewBinding(key.WithKeys("down", "j"))):
		if c.selected < len(c.filtered())-1 {
			c.selected++
		}
	case key.Matches(km, key.NewBinding(key.WithKeys("enter"))):
		if e := c.Selected(); e != nil {
			entry := *e
			return c, func() tea.Msg { return CollectionSelectMsg{Entry: entry} }
		}
	case km.String() == "d":
		if e := c.Selected(); e != nil {
			id := e.ID
			return c, func() tea.Msg { return CollectionDeleteMsg{ID: id} }
		}
	case km.String() == "e":
		if e := c.Selected(); e != nil {
			id := e.ID
			return c, func() tea.Msg { return CollectionRenameMsg{ID: id} }
		}
	}
	return c, nil
}

func (c Collections) View(width, height int) string {
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
		Width(inner).
		Height(innerH)
	if c.focused {
		border = border.BorderForeground(lipgloss.Color("205"))
	} else {
		border = border.BorderForeground(lipgloss.Color("240"))
	}

	title := lipgloss.NewStyle().Bold(true).Render("Collections")

	filterLine := ""
	titleH := 1
	if c.filterMode || c.filter != "" {
		filterLine = renderFilterLine(c.filter, c.filterMode)
		titleH = 2
	}

	entries := c.filtered()
	if len(entries) == 0 {
		var msg string
		if c.filter != "" {
			msg = "(no matches)"
		} else {
			msg = "(empty — Ctrl+B to save)"
		}
		empty := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(msg)
		out := title
		if filterLine != "" {
			out += "\n" + filterLine
		}
		out += "\n" + empty
		return border.Render(out)
	}

	maxRows := innerH - titleH
	if maxRows < 1 {
		maxRows = 1
	}
	start := 0
	if c.selected >= maxRows {
		start = c.selected - maxRows + 1
	}
	end := start + maxRows
	if end > len(entries) {
		end = len(entries)
	}

	var lines []string
	lines = append(lines, title)
	if filterLine != "" {
		lines = append(lines, filterLine)
	}
	innerWidth := inner - 2
	for i := start; i < end; i++ {
		selected := c.focused && i == c.selected
		lines = append(lines, renderCollectionRow(entries[i], innerWidth, selected, c.filter))
	}
	return border.Render(joinLines(lines))
}

// renderCollectionRow lays out "NAME  METH  URL..." within width chars.
// When filter is non-empty and the row is not selected, matching substrings are
// highlighted in the name and URL columns.
func renderCollectionRow(e collections.Entry, width int, selected bool, filter string) string {
	if width <= 0 {
		return ""
	}
	name := e.Name
	method := e.Request.Method
	if len(method) > 6 {
		method = method[:6]
	}
	url := e.Request.URL

	const minNameW = 6
	methW := 6

	nameW := width / 3
	if nameW < minNameW {
		nameW = minNameW
	}
	urlSpace := width - nameW - 1 - methW - 1
	if urlSpace < 4 {
		urlSpace = 4
	}

	nameTrunc := truncate(name, nameW)
	urlTrunc := truncate(url, urlSpace)

	raw := padRight(nameTrunc, nameW) + " " + padRight(method, methW) + " " + urlTrunc
	if selected {
		return selectedStyle().Render(padRight(raw, width))
	}
	nameStyled := highlightMatchColored(nameTrunc, nameW, filter, lipgloss.Color("44"))
	methStyled := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(padRight(method, methW))
	return nameStyled + " " + methStyled + " " + highlightMatch(urlTrunc, filter)
}
