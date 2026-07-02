package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/ui"
)

// layoutDims computes the main region's column widths and content height. It is
// the single source of truth for the top-region geometry, shared by View (to
// render) and handleMouse (to hit-test), so the two can't drift. It mirrors the
// math at the top of View.
func (m Model) layoutDims() (sidebarW, requestW, responseW, contentH int) {
	statusH := lipgloss.Height(m.renderStatusBar())
	contentH = m.height - statusH
	if contentH < 5 {
		contentH = 5
	}

	if m.showHistory || m.showCollections {
		sidebarW = m.width / 4
		if sidebarW < 20 {
			sidebarW = 20
		}
		if sidebarW > m.sidebarMaxWidth {
			sidebarW = m.sidebarMaxWidth
		}
	}

	available := m.width - sidebarW
	if available < 1 {
		available = 1
	}
	responseW = int(float64(available) * m.responsePanelRatio)
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
	requestW = available - responseW
	if requestW < 1 {
		requestW = 1
	}
	return sidebarW, requestW, responseW, contentH
}

// anyOverlayOpen reports whether a modal overlay is currently capturing the
// screen. Mouse events are ignored while one is open so clicks don't leak
// through to the panels behind it.
func (m Model) anyOverlayOpen() bool {
	return m.intruder.State() != ui.IntruderHidden ||
		m.ws.State() != ui.WSHidden ||
		m.scenario.State() != ui.ScenHidden ||
		m.copyMenuOpen || m.help.IsOpen() || m.settingsPanel.IsOpen() ||
		m.envSwitcherOpen || m.renamingColl || m.collUpdatePromptOpen ||
		m.savingToCollection || m.importingFile
}

// handleMouse routes a mouse event to the panel under the cursor: left-click
// focuses a panel (and loads the clicked sidebar row), the wheel scrolls the
// response body or moves the sidebar selection. Only fires when mouse support
// is enabled (main.go started the program with mouse reporting) and no overlay
// is open.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !m.mouseEnabled || m.anyOverlayOpen() {
		return m, nil
	}
	sidebarW, requestW, _, contentH := m.layoutDims()
	x, y := msg.X, msg.Y
	// Below the content region is the status bar — not interactive.
	if y >= contentH {
		return m, nil
	}

	hasSidebar := m.showHistory || m.showCollections
	inSidebar := hasSidebar && x < sidebarW
	inRequest := x >= sidebarW && x < sidebarW+requestW
	inResponse := x >= sidebarW+requestW

	// Mouse wheel: scroll the response body, or move the sidebar selection.
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		up := msg.Button == tea.MouseButtonWheelUp
		switch {
		case inResponse:
			if up {
				m.response.ScrollUp(3)
			} else {
				m.response.ScrollDown(3)
			}
		case inSidebar && m.showHistory:
			m.focus = focusHistory
			m.applyFocus()
			var cmd tea.Cmd
			m.history, cmd = m.history.Update(wheelKey(up))
			return m, cmd
		case inSidebar && m.showCollections:
			m.focus = focusCollections
			m.applyFocus()
			var cmd tea.Cmd
			m.collUI, cmd = m.collUI.Update(wheelKey(up))
			return m, cmd
		}
		return m, nil
	}

	// Left click: focus the panel, and load a clicked sidebar row.
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		switch {
		case inSidebar && m.showHistory:
			m.focus = focusHistory
			if idx, ok := m.history.RowAt(contentH, y); ok {
				m.history.SelectIndex(idx)
				if e := m.history.Selected(); e != nil {
					m.applyEntry(*e)
					m.focus = focusURL
				}
			}
			m.applyFocus()
		case inSidebar && m.showCollections:
			m.focus = focusCollections
			if idx, ok := m.collUI.RowAt(contentH, y); ok {
				m.collUI.SelectIndex(idx)
				if e := m.collUI.Selected(); e != nil {
					m.applyEntry(history.Entry{Request: e.Request})
					m.lastLoadedCollID = e.ID
					m.focus = focusURL
				}
			}
			m.applyFocus()
		case inResponse:
			m.focus = focusResponse
			m.applyFocus()
		case inRequest:
			m.focus = focusURL
			m.applyFocus()
		}
	}
	return m, nil
}

// wheelKey turns a wheel direction into the up/down KeyMsg the list components
// already understand, so wheel scrolling reuses their selection-movement logic.
func wheelKey(up bool) tea.KeyMsg {
	if up {
		return tea.KeyMsg{Type: tea.KeyUp}
	}
	return tea.KeyMsg{Type: tea.KeyDown}
}
