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
		return helpView(m.width, m.height)
	}

	return view
}

func (m Model) renderMain(width, height int) string {
	methodView := m.method.View()
	methodW := lipgloss.Width(methodView)
	urlView := m.urlBar.View(width - methodW - 1)
	requestLine := lipgloss.JoinHorizontal(lipgloss.Top, methodView, " ", urlView)

	headersView := m.headers.View(width)

	// Compute remaining space: split between body and response.
	used := lipgloss.Height(requestLine) + lipgloss.Height(headersView)
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

	return lipgloss.JoinVertical(lipgloss.Left, requestLine, headersView, bodyView, respView)
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
	if m.focus == focusResponse && m.response.CurrentBytes() != nil {
		parts = append(parts, "s: save")
	}
	parts = append(parts, "?: help")
	left := strings.Join(parts, "  ·  ")

	// Right side: optional TLS warning, then transient status message.
	right := ""
	if httpx.SkipTLSVerify.Load() {
		right += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true).
			Render("[TLS: insecure]")
	}
	if m.statusMsg != "" {
		if right != "" {
			right += "  "
		}
		right += lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(m.statusMsg)
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	content := left + strings.Repeat(" ", gap) + right
	return style.Render(content)
}

func helpView(w, h int) string {
	body := `Keybindings

Global
  Tab / Shift+Tab        Move focus
  Ctrl+S                 Send request
  Ctrl+Y                 Copy as cURL / fetch
  Ctrl+H                 Toggle history panel
  Ctrl+T                 Toggle TLS verification skip
  Ctrl+C                 Quit
  ?                      This help

History    ↑/↓ move  ·  Enter load  ·  d delete
Method     ↑/↓ cycle
Headers    ↑/↓ ←/→ navigate  ·  Enter new row  ·  Ctrl+D delete  ·  Tab accept
Body       ←/→ tab  ·  Enter edit  ·  Tab indent  ·  Esc leave
Response   ↑/↓ PgUp/PgDn scroll  ·  s save

Press ? or Esc to close`
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
		Render("Copy request as:\n\n  [c] cURL  (POSIX)\n  [f] fetch (JavaScript)\n\n  Esc to cancel")
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, menu)
}
