// Help is the scrollable, accordion-style keybinding overlay opened
// with Ctrl+/. Sections are passed in from the app layer (since the
// content references the KeyMap), but all navigation state and
// rendering lives here.
//
// Layout is a single vertical column so the same code path serves
// any terminal width (tmux splits, wide laptops). A small row of
// action buttons sits above the section list and is part of the
// same up/down focus cycle; pressing Enter on a button fires the
// associated Msg (and, for the destructive reset action, switches
// the overlay into a y/n confirmation view first). Section headers
// render `▶ Title` collapsed, `▼ Title` when expanded. The cursor
// (focused row, button or section) is always kept inside the
// viewport.
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

// HelpOpenSettingsMsg is emitted when the "Open Settings" help button
// is activated. The app handles it by closing the help and opening the
// Settings overlay (same path as the global Ctrl+P binding).
type HelpOpenSettingsMsg struct{}

// HelpResetSettingsMsg is emitted when the user confirms the
// "Reset settings to defaults" action from the help overlay. The app
// handles it by calling applySettings on settings.Defaults().
type HelpResetSettingsMsg struct{}

// helpButton is one of the action buttons rendered at the top of the
// help overlay. If confirm is true, activating the button switches the
// overlay into a y/n confirmation view instead of emitting msg
// immediately; the msg fires only after the user presses y.
type helpButton struct {
	label   string
	msg     tea.Msg
	confirm bool
}

// Help is the help overlay's state and rendering.
type Help struct {
	sections   []HelpSection
	buttons    []helpButton
	cursor     int
	expanded   []bool
	scroll     int
	width      int
	height     int
	open       bool
	confirming bool
	pendingMsg tea.Msg
}

// NewHelp returns a closed Help overlay with the standard action
// buttons preconfigured.
func NewHelp() Help {
	return Help{
		buttons: []helpButton{
			{label: "Open Settings", msg: HelpOpenSettingsMsg{}},
			{label: "Reset settings to defaults", msg: HelpResetSettingsMsg{}, confirm: true},
		},
	}
}

// Open sets the content, focuses the first button, pre-expands the
// first section, and marks the overlay open. Calling Open again resets
// cursor and scroll but preserves expansion state of sections by index
// when the section count is unchanged (so the same Help instance can
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
	h.confirming = false
	h.pendingMsg = nil
}

// Close marks the overlay closed but keeps state for the next open.
func (h *Help) Close() {
	h.open = false
	h.confirming = false
	h.pendingMsg = nil
}

// IsOpen reports whether the overlay is currently visible.
func (h Help) IsOpen() bool { return h.open }

// SetSize records the parent terminal dimensions so View() can clip
// the rendered body to the visible viewport.
func (h *Help) SetSize(w, height int) {
	h.width = w
	h.height = height
}

// Update handles navigation, button activation, and the confirm-view
// y/n prompt.
func (h Help) Update(msg tea.Msg) (Help, tea.Cmd) {
	if !h.open {
		return h, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return h, nil
	}

	if h.confirming {
		switch km.String() {
		case "y", "Y":
			pending := h.pendingMsg
			h.confirming = false
			h.pendingMsg = nil
			h.open = false
			if pending != nil {
				return h, func() tea.Msg { return pending }
			}
			return h, nil
		case "n", "N", "esc":
			h.confirming = false
			h.pendingMsg = nil
		}
		return h, nil
	}

	total := len(h.buttons) + len(h.sections)
	if total == 0 {
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
		if h.cursor < total-1 {
			h.cursor++
		}
	case "g", "home":
		h.cursor = 0
	case "G", "end":
		h.cursor = total - 1
	case "pgup":
		h.cursor -= 5
		if h.cursor < 0 {
			h.cursor = 0
		}
	case "pgdown":
		h.cursor += 5
		if h.cursor > total-1 {
			h.cursor = total - 1
		}
	case "enter", " ":
		if h.cursor < len(h.buttons) {
			btn := h.buttons[h.cursor]
			if btn.confirm {
				h.confirming = true
				h.pendingMsg = btn.msg
				return h, nil
			}
			h.open = false
			pending := btn.msg
			if pending != nil {
				return h, func() tea.Msg { return pending }
			}
			return h, nil
		}
		secIdx := h.cursor - len(h.buttons)
		h.expanded[secIdx] = !h.expanded[secIdx]
	}
	return h, nil
}

// View renders the centered overlay box. In normal mode it shows the
// button row + the section accordion, clipping to the available
// height and keeping the focused row visible. In confirming mode it
// replaces the body with a y/n prompt.
func (h Help) View() string {
	if !h.open {
		return ""
	}

	if h.confirming {
		return h.confirmView()
	}

	if len(h.sections) == 0 && len(h.buttons) == 0 {
		return ""
	}

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	focus := lipgloss.NewStyle().Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0"))
	btnIdle := lipgloss.NewStyle().Background(lipgloss.Color("238")).Foreground(lipgloss.Color("230"))

	// Build the flat line list and remember each focusable row's
	// starting line so the scroll calculation below can keep the
	// focused row inside the viewport. focusLineStart is indexed
	// the same way as h.cursor: [buttons..., sections...].
	var lines []string
	focusLineStart := make([]int, len(h.buttons)+len(h.sections))

	for i, btn := range h.buttons {
		focusLineStart[i] = len(lines)
		label := "[ " + btn.label + " ]"
		if i == h.cursor {
			label = focus.Render(label)
		} else {
			label = btnIdle.Render(label)
		}
		lines = append(lines, label)
	}
	if len(h.buttons) > 0 {
		lines = append(lines, "")
	}
	for i, sec := range h.sections {
		focusLineStart[len(h.buttons)+i] = len(lines)
		glyph := "▶"
		if h.expanded[i] {
			glyph = "▼"
		}
		header := glyph + " " + sec.Title
		if len(h.buttons)+i == h.cursor {
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
	// Keep focused row in view.
	hdr := focusLineStart[h.cursor]
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
	sb.WriteString(dim.Render("↑/↓ j/k: move  ·  Enter: run / expand  ·  g/G: first/last  ·  Esc / Ctrl+/: close"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(sb.String())
	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, box)
}

// confirmView renders the y/n prompt that appears after activating a
// destructive action button (currently only "reset settings").
func (h Help) confirmView() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	var sb strings.Builder
	sb.WriteString("Reset settings")
	sb.WriteString("\n\n")
	sb.WriteString("Reset all settings to their default values?")
	sb.WriteString("\n\n")
	sb.WriteString(dim.Render("Y: confirm  ·  N / Esc: cancel"))

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
