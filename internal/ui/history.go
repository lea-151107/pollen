package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea/pollen/internal/history"
)

type History struct {
	entries  []history.Entry
	selected int
	focused  bool
}

func NewHistory() History {
	return History{}
}

func (h *History) SetEntries(entries []history.Entry) {
	h.entries = entries
	if h.selected >= len(entries) {
		h.selected = 0
	}
}

func (h History) Selected() *history.Entry {
	if h.selected < 0 || h.selected >= len(h.entries) {
		return nil
	}
	return &h.entries[h.selected]
}

func (h *History) Focus() { h.focused = true }
func (h *History) Blur()  { h.focused = false }
func (h History) Focused() bool { return h.focused }

type HistorySelectMsg struct{ Entry history.Entry }

func (h History) Update(msg tea.Msg) (History, tea.Cmd) {
	if !h.focused {
		return h, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return h, nil
	}
	switch {
	case key.Matches(km, key.NewBinding(key.WithKeys("up", "k"))):
		if h.selected > 0 {
			h.selected--
		}
	case key.Matches(km, key.NewBinding(key.WithKeys("down", "j"))):
		if h.selected < len(h.entries)-1 {
			h.selected++
		}
	case key.Matches(km, key.NewBinding(key.WithKeys("enter"))):
		if e := h.Selected(); e != nil {
			entry := *e
			return h, func() tea.Msg { return HistorySelectMsg{Entry: entry} }
		}
	}
	return h, nil
}

func (h History) View(width, height int) string {
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
	if h.focused {
		border = border.BorderForeground(lipgloss.Color("205"))
	} else {
		border = border.BorderForeground(lipgloss.Color("240"))
	}

	title := lipgloss.NewStyle().Bold(true).Render("History")
	if len(h.entries) == 0 {
		empty := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("(no entries)")
		return border.Render(title + "\n" + empty)
	}

	maxRows := innerH - 1 // -1 for title
	if maxRows < 1 {
		maxRows = 1
	}

	start := 0
	if h.selected >= maxRows {
		start = h.selected - maxRows + 1
	}
	end := start + maxRows
	if end > len(h.entries) {
		end = len(h.entries)
	}

	var lines []string
	lines = append(lines, title)
	innerWidth := inner - 2 // -2 padding
	for i := start; i < end; i++ {
		e := h.entries[i]
		line := fmt.Sprintf("%-6s %s", e.Request.Method, e.Request.URL)
		line = truncate(line, innerWidth)
		if h.focused && i == h.selected {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("205")).
				Foreground(lipgloss.Color("0")).
				Render(line)
		}
		lines = append(lines, line)
	}
	return border.Render(joinLines(lines))
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= w {
		return s
	}
	if w <= 1 {
		return "…"
	}
	return string(rs[:w-1]) + "…"
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
