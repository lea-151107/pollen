package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea/pollen/internal/history"
)

type History struct {
	entries    []history.Entry
	selected   int // index in the filtered view
	focused    bool
	filter     string // empty = no filter
	filterMode bool   // true while the user is typing in the filter
	pendingG   bool   // true after a single 'g' press, waiting for 'gg'
}

func NewHistory() History {
	return History{}
}

func (h *History) SetEntries(entries []history.Entry) {
	h.pendingG = false
	h.entries = entries
	fe := h.filtered()
	switch {
	case len(fe) == 0:
		h.selected = 0
	case h.selected >= len(fe):
		// Cursor was on the last row that just got deleted — keep it on the
		// new last row instead of jumping to the top.
		h.selected = len(fe) - 1
	case h.selected < 0:
		h.selected = 0
	}
}

// filtered returns the entries matching the current filter (or all entries
// when the filter is empty). Matching is case-insensitive against
// "METHOD URL".
func (h History) filtered() []history.Entry {
	if h.filter == "" {
		return h.entries
	}
	needle := strings.ToLower(h.filter)
	var out []history.Entry
	for _, e := range h.entries {
		hay := strings.ToLower(e.Request.Method + " " + e.Request.URL)
		if strings.Contains(hay, needle) {
			out = append(out, e)
		}
	}
	return out
}

func (h History) Selected() *history.Entry {
	fe := h.filtered()
	if h.selected < 0 || h.selected >= len(fe) {
		return nil
	}
	return &fe[h.selected]
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

func (h *History) Focus() { h.focused = true }
func (h *History) Blur() {
	h.focused = false
	h.filterMode = false
	h.pendingG = false
	// Note: keep `filter` so the same view comes back when the user returns.
}
func (h History) Focused() bool { return h.focused }

// InFilterMode reports whether the user is currently typing a filter. Used
// by the parent model so the global `?` shortcut treats characters like
// "?" as filter input instead of opening help.
func (h History) InFilterMode() bool { return h.filterMode }

type HistorySelectMsg struct{ Entry history.Entry }
type HistoryDeleteMsg struct{ ID string }

func (h History) Update(msg tea.Msg) (History, tea.Cmd) {
	if !h.focused {
		return h, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return h, nil
	}

	if h.filterMode {
		switch km.String() {
		case "esc":
			// Esc: drop the filter entirely and exit filter mode.
			h.filterMode = false
			h.filter = ""
			h.selected = 0
		case "enter":
			// Enter: keep the filter, leave editing mode.
			h.filterMode = false
		case "backspace", "ctrl+h":
			rs := []rune(h.filter)
			if len(rs) > 0 {
				h.filter = string(rs[:len(rs)-1])
				h.selected = 0
			}
		default:
			// Append printable ASCII (plus utf-8 runes via km.String()).
			s := km.String()
			if len(s) >= 1 && s[0] >= ' ' && s != "tab" {
				h.filter += s
				h.selected = 0
			}
		}
		return h, nil
	}

	switch {
	case km.String() == "/":
		h.pendingG = false
		h.filterMode = true
		return h, nil
	case km.String() == "esc" && h.filter != "":
		// Outside filter mode, Esc still clears an active filter.
		h.pendingG = false
		h.filter = ""
		h.selected = 0
		return h, nil
	case km.String() == "G":
		h.pendingG = false
		if n := len(h.filtered()); n > 0 {
			h.selected = n - 1
		}
		return h, nil
	case km.String() == "g":
		if h.pendingG {
			h.pendingG = false
			h.selected = 0
		} else {
			h.pendingG = true
		}
		return h, nil
	case key.Matches(km, key.NewBinding(key.WithKeys("up", "k"))):
		h.pendingG = false
		if h.selected > 0 {
			h.selected--
		}
	case key.Matches(km, key.NewBinding(key.WithKeys("down", "j"))):
		h.pendingG = false
		if h.selected < len(h.filtered())-1 {
			h.selected++
		}
	case key.Matches(km, key.NewBinding(key.WithKeys("enter"))):
		h.pendingG = false
		if e := h.Selected(); e != nil {
			entry := *e
			return h, func() tea.Msg { return HistorySelectMsg{Entry: entry} }
		}
	case km.String() == "d":
		h.pendingG = false
		if e := h.Selected(); e != nil {
			id := e.ID
			return h, func() tea.Msg { return HistoryDeleteMsg{ID: id} }
		}
	default:
		h.pendingG = false
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

	// Optional filter line right under the title.
	filterLine := ""
	titleH := 1
	if h.filterMode || h.filter != "" {
		filterLine = renderFilterLine(h.filter, h.filterMode)
		titleH = 2
	}

	entries := h.filtered()
	if len(entries) == 0 {
		var msg string
		if h.filter != "" {
			msg = "(no matches)"
		} else {
			msg = "(no entries)"
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
	if h.selected >= maxRows {
		start = h.selected - maxRows + 1
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
	innerWidth := inner - 2 // -2 padding
	for i := start; i < end; i++ {
		selected := h.focused && i == h.selected
		lines = append(lines, renderHistoryRow(entries[i], innerWidth, selected, h.filter))
	}
	return border.Render(joinLines(lines))
}

func renderFilterLine(filter string, editing bool) string {
	prefix := "/"
	color := lipgloss.Color("244")
	if editing {
		color = lipgloss.Color("205")
	}
	cursor := ""
	if editing {
		cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("█")
	}
	return lipgloss.NewStyle().Foreground(color).Render(prefix+filter) + cursor
}

// renderHistoryRow lays out "STAT METH URL ... TIME" within width chars.
// Sections drop in order time → status when space gets tight.
// When filter is non-empty and the row is not selected, the matching substring
// is highlighted in the URL column.
func renderHistoryRow(e history.Entry, width int, selected bool, filter string) string {
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
	statW := 3
	methW := 6
	timeW := len(timeStr)

	const minURLW = 8
	full := statW + 1 + methW + 1 + minURLW + 1 + timeW
	noTime := statW + 1 + methW + 1 + minURLW
	noStatus := methW + 1 + minURLW

	switch {
	case width >= full:
		urlSpace := width - statW - 1 - methW - 1 - timeW - 1
		urlTrunc := truncate(url, urlSpace)
		raw := fmt.Sprintf("%s %-6s %s %s", status, method, padRight(urlTrunc, urlSpace), timeStr)
		if selected {
			return selectedStyle().Render(raw)
		}
		statusStyled := lipgloss.NewStyle().Foreground(statusColor).Render(status)
		timeStyled := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(timeStr)
		urlPart := padRightANSI(highlightMatch(urlTrunc, filter), urlSpace)
		return fmt.Sprintf("%s %-6s %s %s", statusStyled, method, urlPart, timeStyled)

	case width >= noTime:
		urlSpace := width - statW - 1 - methW - 1
		urlTrunc := truncate(url, urlSpace)
		raw := fmt.Sprintf("%s %-6s %s", status, method, urlTrunc)
		if selected {
			return selectedStyle().Render(padRight(raw, width))
		}
		statusStyled := lipgloss.NewStyle().Foreground(statusColor).Render(status)
		return fmt.Sprintf("%s %-6s %s", statusStyled, method, highlightMatch(urlTrunc, filter))

	case width >= noStatus:
		urlSpace := width - methW - 1
		urlTrunc := truncate(url, urlSpace)
		raw := fmt.Sprintf("%-6s %s", method, urlTrunc)
		if selected {
			return selectedStyle().Render(padRight(raw, width))
		}
		return fmt.Sprintf("%-6s %s", method, highlightMatch(urlTrunc, filter))

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

// highlightMatchColored applies color to each segment (before/match/after) of
// text individually, so the base color is never lost due to ANSI nesting. The
// match segment additionally gets Bold+Underline. The result is padded to padW
// visible columns using rune count.
func highlightMatchColored(text string, padW int, needle string, color lipgloss.Color) string {
	base := lipgloss.NewStyle().Foreground(color)
	textRunes := len([]rune(text))
	pad := ""
	if padW > textRunes {
		pad = strings.Repeat(" ", padW-textRunes)
	}
	if needle == "" {
		return base.Render(text + pad)
	}
	lower := strings.ToLower(text)
	idx := strings.Index(lower, strings.ToLower(needle))
	if idx < 0 {
		return base.Render(text + pad)
	}
	before := text[:idx]
	match := text[idx : idx+len(needle)]
	after := text[idx+len(needle):]
	return base.Render(before) + base.Bold(true).Underline(true).Render(match) + base.Render(after+pad)
}

// highlightMatch wraps the first case-insensitive occurrence of needle in bold
// underline. Returns text unchanged when needle is empty or not found.
func highlightMatch(text, needle string) string {
	if needle == "" {
		return text
	}
	lower := strings.ToLower(text)
	idx := strings.Index(lower, strings.ToLower(needle))
	if idx < 0 {
		return text
	}
	before := text[:idx]
	match := text[idx : idx+len(needle)]
	after := text[idx+len(needle):]
	return before + lipgloss.NewStyle().Bold(true).Underline(true).Render(match) + after
}

// padRightANSI pads s to at least w visible columns, using lipgloss.Width to
// measure correctly even when s contains ANSI escape codes.
func padRightANSI(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vis)
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
