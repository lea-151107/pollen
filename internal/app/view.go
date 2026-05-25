package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lea/pollen/internal/httpx"
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}

	statusBar := m.renderStatusBar()
	statusH := lipgloss.Height(statusBar)

	contentH := m.height - statusH
	if contentH < 5 {
		contentH = 5
	}

	historyW := 0
	if m.showHistory {
		historyW = m.width / 4
		if historyW < 20 {
			historyW = 20
		}
		if historyW > 40 {
			historyW = 40
		}
	}
	mainW := m.width - historyW
	if mainW < 30 {
		mainW = 30
	}

	main := m.renderMain(mainW, contentH)

	var top string
	if m.showHistory {
		hist := m.history.View(historyW, contentH)
		top = lipgloss.JoinHorizontal(lipgloss.Top, hist, main)
	} else {
		top = main
	}

	view := lipgloss.JoinVertical(lipgloss.Left, top, statusBar)

	if m.copyMenuOpen {
		return copyMenuView(m.width, m.height)
	}
	if m.helpOpen {
		return helpView(m.keys, m.width, m.height)
	}
	if m.envSwitcherOpen {
		return envSwitcherView(m.env.Names(), m.envSwitcherCursor, m.env.Current, m.width, m.height)
	}

	return view
}

func (m Model) renderMain(width, height int) string {
	methodView := m.method.View()
	methodW := lipgloss.Width(methodView)
	urlView := m.urlBar.View(width - methodW - 1)
	requestLine := lipgloss.JoinHorizontal(lipgloss.Top, methodView, " ", urlView)

	queryView := m.query.View(width)
	authView := m.auth.View(width)
	headersView := m.headers.View(width)

	// Compute remaining space: split between body and response.
	used := lipgloss.Height(requestLine) + lipgloss.Height(queryView) +
		lipgloss.Height(authView) + lipgloss.Height(headersView)
	remaining := height - used
	if remaining < 6 {
		remaining = 6
	}
	bodyH := remaining / 2
	respH := remaining - bodyH
	if bodyH < 4 {
		bodyH = 4
	}
	if respH < 4 {
		respH = 4
	}

	bodyView := m.body.View(width, bodyH)
	respView := m.response.View(width, respH)

	return lipgloss.JoinVertical(lipgloss.Left, requestLine, queryView, authView, headersView, bodyView, respView)
}

func (m Model) renderStatusBar() string {
	style := lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("252")).Padding(0, 1).Width(m.width)
	parts := []string{
		"Tab: focus",
		"Ctrl+S: send",
		"Ctrl+Y: copy",
		"Ctrl+H: history",
		"Ctrl+C: quit",
	}
	switch m.focus {
	case focusMethod:
		parts = append(parts, "↑↓: cycle method")
	case focusResponse:
		if m.response.CurrentBytes() != nil {
			parts = append(parts, "s: save")
		}
	}
	parts = append(parts, "Ctrl+/: help")
	left := strings.Join(parts, "  ·  ")

	// Right side: env name, optional TLS warning, then transient status message.
	right := ""
	if name := m.env.Current; name != "" {
		right += lipgloss.NewStyle().Foreground(lipgloss.Color("44")).Bold(true).
			Render("[env: " + name + "]")
	}
	if httpx.SkipTLSVerify.Load() {
		if right != "" {
			right += " "
		}
		right += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true).
			Render("[TLS: insecure]")
	}
	if m.statusMsg != "" {
		if right != "" {
			right += "  "
		}
		color := lipgloss.Color("10")
		switch m.statusKind {
		case statusWarn:
			color = lipgloss.Color("214")
		case statusError:
			color = lipgloss.Color("9")
		}
		right += lipgloss.NewStyle().Foreground(color).Render(m.statusMsg)
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	content := left + strings.Repeat(" ", gap) + right
	return style.Render(content)
}

func helpView(km KeyMap, w, h int) string {
	body := buildHelpBody(km.HelpSections(), w)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(body)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

// buildHelpBody renders the help sections, switching to a compact column-aligned
// layout when the terminal is too narrow for the wide format.
func buildHelpBody(sections []HelpSection, termWidth int) string {
	var sb strings.Builder
	sb.WriteString("Keybindings\n\n")

	if termWidth < 70 {
		// Compact: each section as "Title:\n  keys  desc" lines, key col 14.
		for _, sec := range sections {
			sb.WriteString(sec.Title)
			sb.WriteString("\n")
			for _, it := range sec.Items {
				sb.WriteString("  ")
				sb.WriteString(padRightHelp(it.Keys, 14))
				sb.WriteString(it.Desc)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	} else {
		// Wide: Global section vertical, others as single-line "Title  k1 d1 · k2 d2".
		for _, sec := range sections {
			if sec.Title == "Global" {
				sb.WriteString("Global\n")
				for _, it := range sec.Items {
					sb.WriteString("  ")
					sb.WriteString(padRightHelp(it.Keys, 22))
					sb.WriteString(it.Desc)
					sb.WriteString("\n")
				}
				sb.WriteString("\n")
				continue
			}
			sb.WriteString(padRightHelp(sec.Title, 10))
			parts := make([]string, 0, len(sec.Items))
			for _, it := range sec.Items {
				parts = append(parts, it.Keys+" "+it.Desc)
			}
			sb.WriteString(strings.Join(parts, "  ·  "))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\nPress Ctrl+/ or Esc to close")
	return sb.String()
}

func padRightHelp(s string, w int) string {
	rs := []rune(s)
	if len(rs) >= w {
		return s + " "
	}
	return s + strings.Repeat(" ", w-len(rs))
}

func envSwitcherView(names []string, cursor int, current string, w, h int) string {
	var sb strings.Builder
	sb.WriteString("Switch environment\n\n")
	for i, n := range names {
		marker := "  "
		line := n
		if n == current {
			line += " (current)"
		}
		if i == cursor {
			marker = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("> ")
			line = lipgloss.NewStyle().Background(lipgloss.Color("205")).
				Foreground(lipgloss.Color("0")).Render(line)
		}
		sb.WriteString(marker)
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("\n↑/↓ select  ·  Enter confirm  ·  Esc cancel")
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(sb.String())
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func copyMenuView(w, h int) string {
	menu := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render("Copy request as:\n\n  [c] cURL  (POSIX)\n  [f] fetch (JavaScript)\n\n  Esc to cancel")
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, menu)
}
