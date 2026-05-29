package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea-151107/pollen/internal/headers"
	"github.com/lea-151107/pollen/internal/history"
)

type headerRow struct {
	keyInput textinput.Model
	valInput textinput.Model
}

func newHeaderRow() headerRow {
	k := textinput.New()
	k.Placeholder = "Header-Name"
	k.CharLimit = 200
	v := textinput.New()
	v.Placeholder = "value"
	v.CharLimit = 4096
	return headerRow{keyInput: k, valInput: v}
}

type Headers struct {
	rows        []headerRow
	activeRow   int
	activeCol   int // 0=key, 1=value
	focused     bool
	suggestions []string // top matches for the current key prefix; [0] is what Tab accepts
}

const maxSuggestions = 5

func NewHeaders() Headers {
	return Headers{rows: []headerRow{newHeaderRow()}}
}

func (h Headers) Values() []history.Header {
	out := make([]history.Header, 0, len(h.rows))
	for _, r := range h.rows {
		k := strings.TrimSpace(r.keyInput.Value())
		if k == "" {
			continue
		}
		out = append(out, history.Header{Key: k, Value: r.valInput.Value()})
	}
	return out
}

func (h *Headers) Set(values []history.Header) {
	h.rows = h.rows[:0]
	for _, v := range values {
		r := newHeaderRow()
		r.keyInput.SetValue(v.Key)
		r.valInput.SetValue(v.Value)
		h.rows = append(h.rows, r)
	}
	h.rows = append(h.rows, newHeaderRow())
	h.activeRow = 0
	h.activeCol = 0
	// Defensive: prior suggestions are based on the old active row's text and
	// would be misleading after a wholesale replacement.
	h.suggestions = nil
	h.refreshFocus()
}

func (h *Headers) Focus() {
	h.focused = true
	h.refreshFocus()
}

func (h *Headers) Blur() {
	h.focused = false
	for i := range h.rows {
		h.rows[i].keyInput.Blur()
		h.rows[i].valInput.Blur()
	}
	h.suggestions = nil
}

func (h Headers) Focused() bool { return h.focused }

func (h Headers) HasSuggestion() bool {
	return h.focused && len(h.suggestions) > 0
}

func (h *Headers) refreshFocus() {
	for i := range h.rows {
		h.rows[i].keyInput.Blur()
		h.rows[i].valInput.Blur()
	}
	if !h.focused {
		return
	}
	if h.activeRow >= len(h.rows) {
		h.activeRow = len(h.rows) - 1
	}
	row := &h.rows[h.activeRow]
	if h.activeCol == 0 {
		row.keyInput.Focus()
	} else {
		row.valInput.Focus()
	}
}

func (h *Headers) currentSuggestions() []string {
	if h.activeCol != 0 {
		return nil
	}
	prefix := h.rows[h.activeRow].keyInput.Value()
	if prefix == "" {
		return nil
	}
	matches := headers.Suggest(prefix)
	if len(matches) == 0 {
		return nil
	}
	// Hide once the user has typed the exact first match.
	if strings.EqualFold(matches[0], prefix) {
		return nil
	}
	if len(matches) > maxSuggestions {
		matches = matches[:maxSuggestions]
	}
	return matches
}

func (h Headers) Update(msg tea.Msg) (Headers, tea.Cmd) {
	if !h.focused {
		return h, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return h, nil
	}

	switch {
	case key.Matches(km, key.NewBinding(key.WithKeys("up"))):
		if h.activeRow > 0 {
			h.activeRow--
			h.refreshFocus()
		}
		h.suggestions = h.currentSuggestions()
		return h, nil

	case key.Matches(km, key.NewBinding(key.WithKeys("down"))):
		if h.activeRow < len(h.rows)-1 {
			h.activeRow++
			h.refreshFocus()
		}
		h.suggestions = h.currentSuggestions()
		return h, nil

	case km.String() == "right" && h.activeCol == 0 && cursorAtEnd(h.rows[h.activeRow].keyInput):
		h.activeCol = 1
		h.refreshFocus()
		h.suggestions = nil
		return h, nil

	case km.String() == "left" && h.activeCol == 1 && h.rows[h.activeRow].valInput.Position() == 0:
		h.activeCol = 0
		h.refreshFocus()
		h.suggestions = h.currentSuggestions()
		return h, nil

	case km.String() == "tab" && len(h.suggestions) > 0 && h.activeCol == 0:
		// Accept first (best) suggestion.
		first := h.suggestions[0]
		h.rows[h.activeRow].keyInput.SetValue(first)
		h.rows[h.activeRow].keyInput.SetCursor(len(first))
		h.suggestions = nil
		return h, nil

	case km.String() == "enter":
		// Do nothing when the current row has no key — prevents stacking empty
		// rows when the user mashes Enter on a blank line.
		if strings.TrimSpace(h.rows[h.activeRow].keyInput.Value()) == "" {
			return h, nil
		}
		// Add a new row after current and move to it.
		if h.activeRow == len(h.rows)-1 {
			h.rows = append(h.rows, newHeaderRow())
		}
		h.activeRow++
		h.activeCol = 0
		h.refreshFocus()
		h.suggestions = nil
		return h, nil

	case key.Matches(km, key.NewBinding(key.WithKeys("ctrl+d"))):
		// Delete current row (keep at least one).
		if len(h.rows) > 1 {
			h.rows = append(h.rows[:h.activeRow], h.rows[h.activeRow+1:]...)
			if h.activeRow >= len(h.rows) {
				h.activeRow = len(h.rows) - 1
			}
			h.refreshFocus()
		}
		h.suggestions = h.currentSuggestions()
		return h, nil
	}

	// Delegate to the focused input.
	var cmd tea.Cmd
	if h.activeCol == 0 {
		h.rows[h.activeRow].keyInput, cmd = h.rows[h.activeRow].keyInput.Update(msg)
		h.suggestions = h.currentSuggestions()
	} else {
		h.rows[h.activeRow].valInput, cmd = h.rows[h.activeRow].valInput.Update(msg)
	}
	return h, cmd
}

func cursorAtEnd(ti textinput.Model) bool {
	return ti.Position() == len(ti.Value())
}

func (h Headers) View(width int) string {
	inner := width - 2
	if inner < 1 {
		inner = 1
	}
	border := lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		Width(inner)
	if h.focused {
		border = border.BorderForeground(lipgloss.Color("205"))
	} else {
		border = border.BorderForeground(lipgloss.Color("240"))
	}

	// content area: inner - 2 padding; row layout: "> " (2) + key + " : " (3) + value
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
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Headers")
	lines = append(lines, header)

	for i, r := range h.rows {
		r.keyInput.Width = keyWidth
		r.valInput.Width = valWidth
		marker := "  "
		if h.focused && i == h.activeRow {
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("> ")
		}
		row := marker + r.keyInput.View() + " : " + r.valInput.View()
		lines = append(lines, row)
	}

	// Always reserve one hint row so panel height does not change with focus.
	var hintLine string
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	switch {
	case h.focused && len(h.suggestions) > 0:
		hintLine = "  ↹ " + truncate(strings.Join(h.suggestions, "  ·  "), contentW-4)
		hintStyle = hintStyle.Italic(true)
	case h.focused:
		hintLine = "  Enter: new row  ·  Ctrl+D: delete row"
	default:
		hintLine = " "
	}
	lines = append(lines, hintStyle.Render(hintLine))

	return border.Render(strings.Join(lines, "\n"))
}
