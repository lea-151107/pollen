// Help is the scrollable, accordion-style keybinding overlay opened
// with Ctrl+/. Sections are passed in from the app layer (since the
// content references the KeyMap), but all navigation state and
// rendering lives here.
//
// Layout is a single vertical column so the same code path serves
// any terminal width (tmux splits, wide laptops). Sections render
// collapsed by default with `▶ Title`; pressing Enter on the
// focused section toggles to `▼ Title` + indented item rows. Any
// number of sections may be expanded at once. The cursor (focused
// section header) is always kept visible inside the viewport.
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpItem is one row inside a HelpSection.
type HelpItem struct {
	Keys string
	Desc string
}

// HelpSection groups related HelpItems under a title.
type HelpSection struct {
	Title string
	Items []HelpItem
}

// Help is the help overlay's state and rendering.
type Help struct {
	sections []HelpSection
	cursor   int
	expanded []bool
	scroll   int
	width    int
	height   int
	open     bool
}

// NewHelp returns a closed Help overlay.
func NewHelp() Help { return Help{} }

// Open sets the content, focuses the first section, pre-expands it,
// and marks the overlay open. Calling Open again resets cursor and
// scroll but preserves expansion state of sections by index when
// the section count is unchanged (so the same Help instance can
// re-open without losing the user's collapse choices).
func (h *Help) Open(sections []HelpSection) {
	h.sections = sections
	if len(h.expanded) != len(sections) {
		h.expanded = make([]bool, len(sections))
		if len(h.expanded) > 0 {
			h.expanded[0] = true
		}
	}
	h.cursor = 0
	h.scroll = 0
	h.open = true
}

// Close marks the overlay closed but keeps state for the next open.
func (h *Help) Close() { h.open = false }

// IsOpen reports whether the overlay is currently visible.
func (h Help) IsOpen() bool { return h.open }

// SetSize records the parent terminal dimensions so View() can clip
// the rendered body to the visible viewport.
func (h *Help) SetSize(w, height int) {
	h.width = w
	h.height = height
}

// Update handles navigation and close keys. Returns the updated
// Help and any tea.Cmd (currently always nil).
func (h Help) Update(msg tea.Msg) (Help, tea.Cmd) {
	if !h.open {
		return h, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return h, nil
	}
	if len(h.sections) == 0 {
		return h, nil
	}
	switch km.String() {
	case "esc", "q":
		h.open = false
	case "up", "k":
		if h.cursor > 0 {
			h.cursor--
		}
	case "down", "j":
		if h.cursor < len(h.sections)-1 {
			h.cursor++
		}
	case "g", "home":
		h.cursor = 0
	case "G", "end":
		h.cursor = len(h.sections) - 1
	case "pgup":
		h.cursor -= 5
		if h.cursor < 0 {
			h.cursor = 0
		}
	case "pgdown":
		h.cursor += 5
		if h.cursor > len(h.sections)-1 {
			h.cursor = len(h.sections) - 1
		}
	case "enter", " ":
		h.expanded[h.cursor] = !h.expanded[h.cursor]
	}
	return h, nil
}

// View renders the centered overlay box, clipping the section list
// to the available height and keeping the focused section header
// inside the viewport.
func (h Help) View() string {
	if !h.open || len(h.sections) == 0 {
		return ""
	}

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	focus := lipgloss.NewStyle().Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0"))

	// Build the flat line list and remember each section header's
	// starting line so the scroll calculation below can keep the
	// focused header inside the viewport.
	var lines []string
	sectionLineStart := make([]int, len(h.sections))
	for i, sec := range h.sections {
		sectionLineStart[i] = len(lines)
		glyph := "▶"
		if h.expanded[i] {
			glyph = "▼"
		}
		header := glyph + " " + sec.Title
		if i == h.cursor {
			header = focus.Render(header)
		}
		lines = append(lines, header)
		if h.expanded[i] {
			keyW := longestKey(sec)
			for _, it := range sec.Items {
				lines = append(lines, "    "+padRightRune(it.Keys, keyW)+"  "+it.Desc)
			}
		}
	}

	// Available content height inside the box. Borders + padding +
	// title + footer = ~6 rows of chrome.
	viewportH := h.height - 6
	if viewportH < 3 {
		viewportH = 3
	}

	scroll := h.scroll
	// Keep focused header in view.
	hdr := sectionLineStart[h.cursor]
	if hdr < scroll {
		scroll = hdr
	}
	if hdr >= scroll+viewportH {
		scroll = hdr - viewportH + 1
	}
	maxScroll := len(lines) - viewportH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	end := scroll + viewportH
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[scroll:end]

	var sb strings.Builder
	sb.WriteString("Keybindings")
	if len(lines) > viewportH {
		sb.WriteString(dim.Render(fmt.Sprintf("  (%d/%d)", scroll+len(visible), len(lines))))
	}
	sb.WriteString("\n\n")
	sb.WriteString(strings.Join(visible, "\n"))
	sb.WriteString("\n\n")
	sb.WriteString(dim.Render("↑/↓ j/k: section  ·  Enter/Space: expand/collapse  ·  g/G: first/last  ·  Esc / Ctrl+/: close"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(sb.String())
	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, box)
}

func longestKey(sec HelpSection) int {
	n := 0
	for _, it := range sec.Items {
		if r := runeLen(it.Keys); r > n {
			n = r
		}
	}
	return n
}

func runeLen(s string) int { return len([]rune(s)) }

func padRightRune(s string, w int) string {
	r := runeLen(s)
	if r >= w {
		return s
	}
	return s + strings.Repeat(" ", w-r)
}
