package ui

import (
	"fmt"
	"time"

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
	switch {
	case len(entries) == 0:
		h.selected = 0
	case h.selected >= len(entries):
		// Cursor was on the last row that just got deleted — keep it on the
		// new last row instead of jumping to the top.
		h.selected = len(entries) - 1
	case h.selected < 0:
		h.selected = 0
	}
}

func (h History) Selected() *history.Entry {
	if h.selected < 0 || h.selected >= len(h.entries) {
		return nil
	}
	return &h.entries[h.selected]
}

func (h History) SelectedIndex() int { return h.selected }

// Shift advances the cursor by delta. Used when an entry is Prepended to the
// underlying store so the cursor keeps pointing at the same logical entry
// instead of "moving" by one. SetEntries clamps the value afterwards.
func (h *History) Shift(delta int) {
	h.selected += delta
	if h.selected < 0 {
		h.selected = 0
	}
}

func (h *History) Focus()       { h.focused = true }
func (h *History) Blur()        { h.focused = false }
func (h History) Focused() bool { return h.focused }

type HistorySelectMsg struct{ Entry history.Entry }
type HistoryDeleteMsg struct{ Index int }

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
	case km.String() == "d":
		if h.Selected() != nil {
			idx := h.selected
			return h, func() tea.Msg { return HistoryDeleteMsg{Index: idx} }
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
		selected := h.focused && i == h.selected
		lines = append(lines, renderHistoryRow(h.entries[i], innerWidth, selected))
	}
	return border.Render(joinLines(lines))
}

// renderHistoryRow lays out "STAT METH URL ... TIME" within width chars.
// Sections drop in order time → status when space gets tight.
func renderHistoryRow(e history.Entry, width int, selected bool) string {
	if width <= 0 {
		return ""
	}
	status, statusColor := statusBadge(e)
	method := e.Request.Method
	if len(method) > 6 {
		method = method[:6]
	}
	url := e.Request.URL
	timeStr := formatRelative(e.Timestamp)

	// Try full layout: "STA METH   URL...   TIME"
	// "STA" (3) + " " + "METH" (left-padded to 6) + " " + URL... + " " + TIME
	const sep = "  "
	statW := 3
	methW := 6
	timeW := len(timeStr)

	const minURLW = 8
	// All sections fit?
	full := statW + 1 + methW + 1 + minURLW + 1 + timeW
	noTime := statW + 1 + methW + 1 + minURLW
	noStatus := methW + 1 + minURLW

	switch {
	case width >= full:
		urlSpace := width - statW - 1 - methW - 1 - timeW - 1
		urlPadded := padRight(truncate(url, urlSpace), urlSpace)
		raw := fmt.Sprintf("%s %-6s %s %s", status, method, urlPadded, timeStr)
		if selected {
			return selectedStyle().Render(raw)
		}
		// Colorize only the status badge; rest is default.
		statusStyled := lipgloss.NewStyle().Foreground(statusColor).Render(status)
		timeStyled := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(timeStr)
		return fmt.Sprintf("%s %-6s %s %s", statusStyled, method, urlPadded, timeStyled)

	case width >= noTime:
		urlSpace := width - statW - 1 - methW - 1
		raw := fmt.Sprintf("%s %-6s %s", status, method, truncate(url, urlSpace))
		if selected {
			return selectedStyle().Render(padRight(raw, width))
		}
		statusStyled := lipgloss.NewStyle().Foreground(statusColor).Render(status)
		return fmt.Sprintf("%s %-6s %s", statusStyled, method, truncate(url, urlSpace))

	case width >= noStatus:
		urlSpace := width - methW - 1
		raw := fmt.Sprintf("%-6s %s", method, truncate(url, urlSpace))
		if selected {
			return selectedStyle().Render(padRight(raw, width))
		}
		return raw

	default:
		raw := truncate(method+" "+url, width)
		if selected {
			return selectedStyle().Render(padRight(raw, width))
		}
		return raw
	}
}

func selectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color("205")).
		Foreground(lipgloss.Color("0"))
}

func statusBadge(e history.Entry) (string, lipgloss.Color) {
	if e.Response == nil {
		return "ERR", lipgloss.Color("8")
	}
	s := e.Response.Status
	text := fmt.Sprintf("%-3d", s)
	if s >= 1000 {
		text = "???"
	}
	switch {
	case s >= 200 && s < 300:
		return text, lipgloss.Color("10")
	case s >= 300 && s < 400:
		return text, lipgloss.Color("214")
	case s >= 400 && s < 500:
		return text, lipgloss.Color("9")
	case s >= 500:
		return text, lipgloss.Color("13")
	}
	return text, lipgloss.Color("8")
}

func formatRelative(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < 0:
		return "soon" // clock skew safety
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func padRight(s string, w int) string {
	rs := []rune(s)
	if len(rs) >= w {
		return s
	}
	pad := make([]rune, w-len(rs))
	for i := range pad {
		pad[i] = ' '
	}
	return s + string(pad)
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
