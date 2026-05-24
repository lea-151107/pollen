package app

import (
	"runtime"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/lea/pollen/internal/history"
	"github.com/lea/pollen/internal/httpx"
	"github.com/lea/pollen/internal/settings"
	"github.com/lea/pollen/internal/ui"
)

// isTextEditingFocus reports whether the currently focused panel is actively
// accepting character input (so global single-key shortcuts like `?` should be
// treated as ordinary input instead of triggering a global action).
func isTextEditingFocus(f focusArea, bodyInEditor bool) bool {
	switch f {
	case focusURL, focusHeaders:
		return true
	case focusBody:
		return bodyInEditor
	}
	return false
}

// clipboardHint returns a platform-specific install suggestion to append to a
// failed-copy message. atotto/clipboard shells out to xclip/wl-copy on Linux.
func clipboardHint() string {
	if runtime.GOOS == "linux" {
		return " (install xclip or wl-clipboard)"
	}
	return ""
}

type sendResultMsg struct {
	entry history.Entry
}

type clearStatusMsg struct{ gen int }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sendResultMsg:
		m.response.SetLoading(false)
		if msg.entry.Error != "" {
			m.response.SetError(msg.entry.Error)
		} else if msg.entry.Response != nil {
			m.response.SetResponse(msg.entry.Response)
		}
		m.store.Prepend(msg.entry)
		_ = m.store.Save()
		m.history.SetEntries(m.store.Entries())
		return m, nil

	case ui.HistorySelectMsg:
		m.applyEntry(msg.Entry)
		m.focus = focusURL
		m.applyFocus()
		return m, nil

	case ui.HistoryDeleteMsg:
		entries := m.store.Entries()
		if msg.Index < 0 || msg.Index >= len(entries) {
			return m, nil
		}
		// Snapshot before delete so `u` can restore it.
		snapshot := entries[msg.Index]
		if !m.store.DeleteAt(msg.Index) {
			return m, nil
		}
		_ = m.store.Save()
		m.history.SetEntries(m.store.Entries())
		m.setStatus(statusOK, "deleted (u to undo)")
		m.pendingUndo = &pendingUndo{entry: snapshot, index: msg.Index, gen: m.statusGen}
		return m, m.statusTick(5 * time.Second)

	case clearStatusMsg:
		// Ignore stale Ticks scheduled for an earlier message.
		if msg.gen == m.statusGen {
			m.statusMsg = ""
			m.statusKind = statusOK
			// Undo window expires together with the toast.
			if m.pendingUndo != nil && m.pendingUndo.gen == msg.gen {
				m.pendingUndo = nil
			}
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
		switch km.String() {
		case "?", "esc", "q":
			m.helpOpen = false
		}
		return m, nil

	case km.String() == "?" && !isTextEditingFocus(m.focus, m.body.InEditorMode()):
		m.helpOpen = true
		return m, nil

	case km.String() == "u" && m.pendingUndo != nil && !isTextEditingFocus(m.focus, m.body.InEditorMode()):
		u := m.pendingUndo
		m.store.InsertAt(u.index, u.entry)
		_ = m.store.Save()
		m.history.SetEntries(m.store.Entries())
		m.pendingUndo = nil
		m.setStatus(statusOK, "restored")
		return m, m.statusTick(2 * time.Second)

	case key.Matches(km, m.keys.Send):
		return m, m.sendRequest()

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

	case key.Matches(km, m.keys.ToggleTLS):
		newVal := !httpx.SkipTLSVerify.Load()
		httpx.SkipTLSVerify.Store(newVal)
		// Persist so the preference survives restarts; failure is non-fatal.
		_ = (&settings.Settings{SkipTLSVerify: newVal}).Save()
		if newVal {
			m.setStatus(statusWarn, "TLS verification: OFF (insecure)")
		} else {
			m.setStatus(statusOK, "TLS verification: ON")
		}
		return m, m.statusTick(2 * time.Second)

	case m.focus == focusResponse && km.String() == "s":
		return m, m.saveResponse()

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
	case focusHeaders:
		m.headers, cmd = m.headers.Update(km)
	case focusBody:
		m.body, cmd = m.body.Update(km)
	case focusResponse:
		m.response, cmd = m.response.Update(km)
	}
	return m, cmd
}

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

func (m Model) currentRequest() history.Request {
	return history.Request{
		Method:   m.method.Value(),
		URL:      m.urlBar.Value(),
		Headers:  m.headers.Values(),
		Body:     m.body.Value(),
		BodyType: m.body.Type(),
	}
}

func (m *Model) applyEntry(e history.Entry) {
	m.method.Set(e.Request.Method)
	m.urlBar.SetValue(e.Request.URL)
	m.headers.Set(e.Request.Headers)
	m.body.Set(e.Request.BodyType, e.Request.Body)
	if e.Response != nil {
		m.response.SetResponse(e.Response)
	} else if e.Error != "" {
		m.response.SetError(e.Error)
	}
}

// deliverCopy puts `content` on the clipboard, or on disk if clipboard fails,
// and reports the outcome through statusMsg with appropriate kind.
func (m *Model) deliverCopy(content, label string) {
	mode, path, err := copyOrFallback(content)
	switch {
	case err != nil:
		m.setStatus(statusError, "copy failed: "+err.Error()+clipboardHint())
	case mode == copyClipboard:
		m.setStatus(statusOK, "copied as "+label)
	case mode == copyFile:
		m.setStatus(statusWarn, "clipboard unavailable - wrote "+label+" to "+path)
	}
}

func (m *Model) saveResponse() tea.Cmd {
	bytes := m.response.CurrentBytes()
	resp := m.response.CurrentResponse()
	if len(bytes) == 0 {
		m.setStatus(statusError, "no body to save")
		return m.statusTick(2 * time.Second)
	}
	dest, err := saveResponseBytes(bytes, resp, m.urlBar.Value())
	if err != nil {
		m.setStatus(statusError, "save failed: "+err.Error())
	} else {
		m.setStatus(statusOK, "saved to "+dest)
	}
	return m.statusTick(2 * time.Second)
}

func (m *Model) sendRequest() tea.Cmd {
	req := m.currentRequest()
	if req.URL == "" {
		m.response.SetError("URL is empty")
		return nil
	}
	m.response.SetLoading(true)
	return func() tea.Msg {
		entry := history.Entry{
			ID:        uuid.NewString(),
			Timestamp: time.Now().UTC(),
			Request:   req,
		}
		resp, err := httpx.Do(req)
		if err != nil {
			entry.Error = err.Error()
		} else {
			entry.Response = resp
		}
		return sendResultMsg{entry: entry}
	}
}
