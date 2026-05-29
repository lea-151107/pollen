package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Param is one query-string parameter (key=value pair).
type Param struct {
	Key, Value string
}

type queryRow struct {
	keyInput textinput.Model
	valInput textinput.Model
}

func newQueryRow() queryRow {
	k := textinput.New()
	k.Placeholder = "key"
	k.CharLimit = 200
	v := textinput.New()
	v.Placeholder = "value"
	v.CharLimit = 4096
	return queryRow{keyInput: k, valInput: v}
}

// Query is a key-value editor for URL query parameters. Mirrors the Headers
// component but without autocomplete (query keys are application-specific).
type Query struct {
	rows      []queryRow
	activeRow int
	activeCol int // 0=key, 1=value
	focused   bool
}

func NewQuery() Query {
	return Query{rows: []queryRow{newQueryRow()}}
}

func (q Query) Values() []Param {
	out := make([]Param, 0, len(q.rows))
	for _, r := range q.rows {
		k := strings.TrimSpace(r.keyInput.Value())
		if k == "" {
			continue
		}
		out = append(out, Param{Key: k, Value: r.valInput.Value()})
	}
	return out
}

func (q *Query) Set(params []Param) {
	q.rows = q.rows[:0]
	for _, p := range params {
		r := newQueryRow()
		r.keyInput.SetValue(p.Key)
		r.valInput.SetValue(p.Value)
		q.rows = append(q.rows, r)
	}
	q.rows = append(q.rows, newQueryRow())
	q.activeRow = 0
	q.activeCol = 0
	q.refreshFocus()
}

func (q *Query) Focus() {
	q.focused = true
	q.refreshFocus()
}

func (q *Query) Blur() {
	q.focused = false
	for i := range q.rows {
		q.rows[i].keyInput.Blur()
		q.rows[i].valInput.Blur()
	}
}

func (q Query) Focused() bool { return q.focused }

func (q *Query) refreshFocus() {
	for i := range q.rows {
		q.rows[i].keyInput.Blur()
		q.rows[i].valInput.Blur()
	}
	if !q.focused {
		return
	}
	if q.activeRow >= len(q.rows) {
		q.activeRow = len(q.rows) - 1
	}
	row := &q.rows[q.activeRow]
	if q.activeCol == 0 {
		row.keyInput.Focus()
	} else {
		row.valInput.Focus()
	}
}

func (q Query) Update(msg tea.Msg) (Query, tea.Cmd) {
	if !q.focused {
		return q, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return q, nil
	}
	switch {
	case key.Matches(km, key.NewBinding(key.WithKeys("up"))):
		if q.activeRow > 0 {
			q.activeRow--
			q.refreshFocus()
		}
		return q, nil
	case key.Matches(km, key.NewBinding(key.WithKeys("down"))):
		if q.activeRow < len(q.rows)-1 {
			q.activeRow++
			q.refreshFocus()
		}
		return q, nil
	case km.String() == "right" && q.activeCol == 0 && cursorAtEnd(q.rows[q.activeRow].keyInput):
		q.activeCol = 1
		q.refreshFocus()
		return q, nil
	case km.String() == "left" && q.activeCol == 1 && q.rows[q.activeRow].valInput.Position() == 0:
		q.activeCol = 0
		q.refreshFocus()
		return q, nil
	case km.String() == "enter":
		if strings.TrimSpace(q.rows[q.activeRow].keyInput.Value()) == "" {
			return q, nil
		}
		if q.activeRow == len(q.rows)-1 {
			q.rows = append(q.rows, newQueryRow())
		}
		q.activeRow++
		q.activeCol = 0
		q.refreshFocus()
		return q, nil
	case key.Matches(km, key.NewBinding(key.WithKeys("ctrl+d"))):
		if len(q.rows) > 1 {
			q.rows = append(q.rows[:q.activeRow], q.rows[q.activeRow+1:]...)
			if q.activeRow >= len(q.rows) {
				q.activeRow = len(q.rows) - 1
			}
			q.refreshFocus()
		}
		return q, nil
	}

	var cmd tea.Cmd
	if q.activeCol == 0 {
		q.rows[q.activeRow].keyInput, cmd = q.rows[q.activeRow].keyInput.Update(msg)
	} else {
		q.rows[q.activeRow].valInput, cmd = q.rows[q.activeRow].valInput.Update(msg)
	}
	return q, cmd
}

func (q Query) View(width int) string {
	inner := width - 2
	if inner < 1 {
		inner = 1
	}
	border := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		Width(inner)
	if q.focused {
		border = border.BorderForeground(lipgloss.Color("205"))
	} else {
		border = border.BorderForeground(lipgloss.Color("240"))
	}

	contentW := inner - 2
	keyWidth := (contentW - 5) / 3
	if keyWidth < 5 {
		keyWidth = 5
	}
	valWidth := contentW - 5 - keyWidth
	if valWidth < 5 {
		valWidth = 5
	}

	var lines []string
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Query")
	lines = append(lines, title)

	for i, r := range q.rows {
		r.keyInput.Width = keyWidth
		r.valInput.Width = valWidth
		marker := "  "
		if q.focused && i == q.activeRow {
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("> ")
		}
		row := marker + r.keyInput.View() + " = " + r.valInput.View()
		lines = append(lines, row)
	}

	hint := " "
	if q.focused {
		hint = "  Enter: new row  ·  Ctrl+D: delete row"
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(hint))

	return border.Render(strings.Join(lines, "\n"))
}
