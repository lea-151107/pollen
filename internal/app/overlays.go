package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea/pollen/internal/httpx"
)

// handleCopyMenu dispatches keypresses while the copy menu overlay is open.
func (m Model) handleCopyMenu(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	req := m.currentRequest()
	switch km.String() {
	case "c", "C":
		m.deliverCopy(httpx.ToCurl(req), "cURL")
	case "f", "F":
		m.deliverCopy(httpx.ToFetch(req), "fetch")
	case "esc", "q":
		// just close
	default:
		return m, nil
	}
	m.copyMenuOpen = false
	if m.statusMsg == "" {
		return m, nil
	}
	return m, m.statusTick(2 * time.Second)
}

// handleEnvSwitcher dispatches keypresses while the env switcher overlay
// is open.
func (m Model) handleEnvSwitcher(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	names := m.env.Names()
	switch km.String() {
	case "up", "k":
		if m.envSwitcherCursor > 0 {
			m.envSwitcherCursor--
		}
		return m, nil
	case "down", "j":
		if m.envSwitcherCursor < len(names)-1 {
			m.envSwitcherCursor++
		}
		return m, nil
	case "enter":
		if m.envSwitcherCursor >= 0 && m.envSwitcherCursor < len(names) {
			chosen := names[m.envSwitcherCursor]
			if err := m.env.SetCurrent(chosen); err == nil {
				// Persist the selection so it survives a restart.
				_ = m.env.Save()
				m.setStatus(statusOK, "switched to env: "+chosen)
				m.envSwitcherOpen = false
				return m, m.statusTick(2 * time.Second)
			}
		}
		m.envSwitcherOpen = false
		return m, nil
	case "esc", "q":
		m.envSwitcherOpen = false
		return m, nil
	}
	return m, nil
}
