package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lea-151107/pollen/internal/ui"
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

	sidebarW := 0
	showSidebar := m.showHistory || m.showCollections
	if showSidebar {
		sidebarW = m.width / 4
		if sidebarW < 20 {
			sidebarW = 20
		}
		if sidebarW > m.sidebarMaxWidth {
			sidebarW = m.sidebarMaxWidth
		}
	}

	// Split remaining width between request and response panels.
	available := m.width - sidebarW
	if available < 1 {
		available = 1
	}
	responseW := int(float64(available) * m.responsePanelRatio)
	if responseW < 30 {
		responseW = 30
	}
	maxResponseW := available - 20
	if maxResponseW < 1 {
		maxResponseW = 1
	}
	if responseW > maxResponseW {
		responseW = maxResponseW
	}
	requestW := available - responseW
	if requestW < 1 {
		requestW = 1
	}

	reqPanel := m.renderRequest(requestW, contentH)
	respPanel := m.response.View(responseW, contentH)
	main := lipgloss.JoinHorizontal(lipgloss.Top, reqPanel, respPanel)

	var top string
	if m.showHistory {
		hist := m.history.View(sidebarW, contentH)
		top = lipgloss.JoinHorizontal(lipgloss.Top, hist, main)
	} else if m.showCollections {
		coll := m.collUI.View(sidebarW, contentH)
		top = lipgloss.JoinHorizontal(lipgloss.Top, coll, main)
	} else {
		top = main
	}

	view := lipgloss.JoinVertical(lipgloss.Left, top, statusBar)

	if m.intruder.State() != ui.IntruderHidden {
		return m.intruder.View()
	}
	if m.copyMenuOpen {
		return copyMenuView(m.width, m.height)
	}
	if m.helpOpen {
		return helpView(m.keys, m.width, m.height)
	}
	if m.envSwitcherOpen {
		return envSwitcherView(m.env.Names(), m.envSwitcherCursor, m.env.Current, m.width, m.height)
	}
	if m.renamingColl {
		return renameCollectionView(m.renameInput.View(), m.width, m.height)
	}
	if m.collUpdatePromptOpen {
		return collUpdatePromptView(m.collUpdateTargetName, m.width, m.height)
	}
	if m.savingToCollection {
		return saveCollectionView(m.saveCollInput.View(), m.width, m.height)
	}
	if m.importingFile {
		return importFileView(m.importInput.View(), m.width, m.height)
	}

	return view
}

func (m Model) renderRequest(width, height int) string {
	methodView := m.method.View()
	methodW := lipgloss.Width(methodView)
	urlView := m.urlBar.View(width - methodW - 1)
	requestLine := lipgloss.JoinHorizontal(lipgloss.Top, methodView, " ", urlView)

	queryView := m.query.View(width)
	authView := m.auth.View(width)
	headersView := m.headers.View(width)

	// Body takes all remaining vertical space (response is now in the right panel).
	used := lipgloss.Height(requestLine) + lipgloss.Height(queryView) +
		lipgloss.Height(authView) + lipgloss.Height(headersView)
	bodyH := height - used
	if bodyH < 4 {
		bodyH = 4
	}

	bodyView := m.body.View(width, bodyH)

	return lipgloss.JoinVertical(lipgloss.Left, requestLine, queryView, authView, headersView, bodyView)
}

func (m Model) renderStatusBar() string {
	style := lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("252")).Padding(0, 1).Width(m.width)
	parts := []string{
		"Tab: focus",
		"Ctrl+S: send",
		"Ctrl+Y: copy",
		"Ctrl+H: history",
		"Ctrl+K: collections",
		"Ctrl+B: save",
		"Ctrl+I: import",
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
	if m.tlsInsecure {
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
	sb.WriteString("\n↑/↓: select  ·  Enter: confirm  ·  Esc: cancel")
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(sb.String())
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func importFileView(inputView string, w, h int) string {
	body := "Import from file\n\nOpenAPI 3.x (JSON/YAML)  ·  Postman v2.1 (JSON)\n\n  " +
		inputView + "\n\n  Enter: import  ·  Esc: cancel"
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(body)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func renameCollectionView(inputView string, w, h int) string {
	body := "Rename collection entry\n\n  " + inputView + "\n\n  Enter: rename  ·  Esc: cancel"
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(body)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func collUpdatePromptView(name string, w, h int) string {
	body := "Update collection entry\n\n  " + name + "\n\n  Enter: update in-place  ·  N: save as new  ·  Esc: cancel"
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(body)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func saveCollectionView(inputView string, w, h int) string {
	body := "Save request to collection\n\n  " + inputView + "\n\n  Enter: save  ·  Esc: cancel"
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(body)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func copyMenuView(w, h int) string {
	menu := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render("Copy request\n\n  [c] cURL  (POSIX)\n  [f] fetch (JavaScript)\n\n  Esc: cancel")
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, menu)
}
