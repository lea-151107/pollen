package app

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea/pollen/internal/httpx"
	"github.com/lea/pollen/internal/settings"
	"github.com/lea/pollen/internal/ui"
)

// clearStatusMsg fires when a transient toast should be cleared. The gen
// field is checked against Model.statusGen so a stale tick (scheduled before
// a newer setStatus) doesn't wipe the newer message.
type clearStatusMsg struct{ gen int }

// isTextEditingFocus reports whether the currently focused panel is actively
// accepting character input. Used to gate single-letter global shortcuts
// (currently `u` for undo) so they don't swallow real input.
func isTextEditingFocus(f focusArea, bodyInEditor, historyFilterMode bool) bool {
	switch f {
	case focusURL, focusQuery, focusAuth, focusHeaders:
		return true
	case focusBody:
		return bodyInEditor
	case focusHistory:
		return historyFilterMode
	}
	return false
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sendResultMsg:
		// Discard out-of-order results from older Send presses; only the
		// latest in-flight request's response should land in the UI.
		if msg.gen != m.requestGen {
			return m, nil
		}
		m.response.SetLoading(false)
		if msg.entry.Error != "" {
			m.response.SetError(msg.entry.Error)
		} else if msg.entry.Response != nil {
			m.response.SetResponse(msg.entry.Response, msg.entry.Request.URL)
		}
		m.store.Prepend(msg.entry)
		_ = m.store.Save()
		// Prepend shifts every existing entry by 1 — slide the cursor too so
		// it keeps pointing at the same entry the user was looking at.
		m.history.Shift(1)
		m.history.SetEntries(m.store.Entries())
		// Any history mutation invalidates a pending undo (indices have shifted).
		m.pendingUndo = nil
		return m, nil

	case ui.HistorySelectMsg:
		m.applyEntry(msg.Entry)
		m.focus = focusURL
		m.applyFocus()
		return m, nil

	case ui.HistoryDeleteMsg:
		// Look up by ID so the operation works regardless of any active
		// history filter (the filter shifts UI indices but not store indices).
		idx := m.store.IndexOf(msg.ID)
		if idx < 0 {
			return m, nil
		}
		snapshot := m.store.Entries()[idx]
		if !m.store.DeleteAt(idx) {
			return m, nil
		}
		_ = m.store.Save()
		m.history.SetEntries(m.store.Entries())
		m.setStatus(statusOK, "deleted (u to undo)")
		m.pendingUndo = &pendingUndo{entry: snapshot, index: idx, gen: m.statusGen}
		return m, m.statusTick(5 * time.Second)

	case clearStatusMsg:
		// Ignore stale Ticks scheduled for an earlier message.
		if msg.gen == m.statusGen {
			m.statusMsg = ""
			m.statusKind = statusOK
			// Undo window expires together with the toast that announced it.
			m.pendingUndo = nil
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys take precedence except where noted.
	switch {
	case key.Matches(km, m.keys.Quit):
		return m, tea.Quit

	case m.copyMenuOpen:
		return m.handleCopyMenu(km)

	case m.helpOpen:
		if key.Matches(km, m.keys.Help) || km.String() == "esc" || km.String() == "q" {
			m.helpOpen = false
		}
		return m, nil

	case m.envSwitcherOpen:
		return m.handleEnvSwitcher(km)

	case key.Matches(km, m.keys.Help):
		// Ctrl+/ is a non-printing key, so it doesn't conflict with text
		// input — no isTextEditingFocus guard needed.
		m.helpOpen = true
		return m, nil

	case km.String() == "u" && m.pendingUndo != nil && !isTextEditingFocus(m.focus, m.body.InEditorMode(), m.history.InFilterMode()):
		u := m.pendingUndo
		m.store.InsertAt(u.index, u.entry)
		_ = m.store.Save()
		m.history.SetEntries(m.store.Entries())
		m.pendingUndo = nil
		m.setStatus(statusOK, "restored")
		return m, m.statusTick(2 * time.Second)

	case key.Matches(km, m.keys.Send):
		// Split into two statements: `return m, m.sendRequest()` would rely on
		// undefined evaluation order between `m` and the pointer-receiver call
		// that mutates m. Current gc happens to evaluate the call first, but
		// the Go spec leaves this unspecified.
		cmd := m.sendRequest()
		return m, cmd

	case key.Matches(km, m.keys.Copy):
		m.copyMenuOpen = true
		return m, nil

	case key.Matches(km, m.keys.ToggleHist):
		m.showHistory = !m.showHistory
		if !m.showHistory && m.focus == focusHistory {
			m.focus = focusURL
			m.applyFocus()
		}
		return m, nil

	case key.Matches(km, m.keys.SwitchEnv):
		names := m.env.Names()
		if len(names) == 0 {
			m.setStatus(statusWarn, "no environments defined in env.json")
			return m, m.statusTick(2 * time.Second)
		}
		m.envSwitcherOpen = true
		// Start cursor at current selection if it exists.
		m.envSwitcherCursor = 0
		for i, n := range names {
			if n == m.env.Current {
				m.envSwitcherCursor = i
				break
			}
		}
		return m, nil

	case key.Matches(km, m.keys.ToggleTLS):
		newVal := !httpx.SkipTLSVerify.Load()
		httpx.SkipTLSVerify.Store(newVal)
		m.tlsInsecure = newVal
		// Persist; surface persistence failures so the user knows the
		// preference won't survive a restart.
		saveErr := (&settings.Settings{SkipTLSVerify: newVal}).Save()
		switch {
		case saveErr != nil:
			m.setStatus(statusWarn, fmt.Sprintf("TLS toggled (settings save failed: %v)", saveErr))
		case newVal:
			m.setStatus(statusWarn, "TLS verification: OFF (insecure)")
		default:
			m.setStatus(statusOK, "TLS verification: ON")
		}
		return m, m.statusTick(2 * time.Second)

	case m.focus == focusResponse && km.String() == "s":
		// See note above on sendRequest — avoid relying on undefined eval order.
		cmd := m.saveResponse()
		return m, cmd

	case key.Matches(km, m.keys.NextFocus):
		// Headers consumes Tab when it has an active suggestion (autocomplete).
		if m.focus == focusHeaders && m.headers.HasSuggestion() {
			var cmd tea.Cmd
			m.headers, cmd = m.headers.Update(km)
			return m, cmd
		}
		// Body editor consumes Tab while typing to insert indentation.
		if m.focus == focusBody && m.body.InEditorMode() {
			var cmd tea.Cmd
			m.body, cmd = m.body.Update(km)
			return m, cmd
		}
		m.cycleFocus(true)
		return m, nil

	case key.Matches(km, m.keys.PrevFocus):
		m.cycleFocus(false)
		return m, nil
	}

	// Delegate to focused component.
	var cmd tea.Cmd
	switch m.focus {
	case focusHistory:
		m.history, cmd = m.history.Update(km)
	case focusMethod:
		m.method, cmd = m.method.Update(km)
	case focusURL:
		m.urlBar, cmd = m.urlBar.Update(km)
	case focusQuery:
		m.query, cmd = m.query.Update(km)
	case focusAuth:
		m.auth, cmd = m.auth.Update(km)
	case focusHeaders:
		m.headers, cmd = m.headers.Update(km)
	case focusBody:
		m.body, cmd = m.body.Update(km)
	case focusResponse:
		m.response, cmd = m.response.Update(km)
	}
	return m, cmd
}
