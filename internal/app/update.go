package app

import (
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/lea/pollen/internal/history"
	"github.com/lea/pollen/internal/httpx"
	"github.com/lea/pollen/internal/ui"
)

type sendResultMsg struct {
	entry history.Entry
}

type clearStatusMsg struct{}

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

	case clearStatusMsg:
		m.statusMsg = ""
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
		s := httpx.ToCurl(req)
		if err := clipboard.WriteAll(s); err != nil {
			m.statusMsg = "copy failed: " + err.Error()
		} else {
			m.statusMsg = "copied as cURL"
		}
	case "f", "F":
		s := httpx.ToFetch(req)
		if err := clipboard.WriteAll(s); err != nil {
			m.statusMsg = "copy failed: " + err.Error()
		} else {
			m.statusMsg = "copied as fetch"
		}
	case "esc", "q":
		// just close
	default:
		return m, nil
	}
	m.copyMenuOpen = false
	if m.statusMsg == "" {
		return m, nil
	}
	return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
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

func (m *Model) saveResponse() tea.Cmd {
	bytes := m.response.CurrentBytes()
	resp := m.response.CurrentResponse()
	if len(bytes) == 0 {
		m.statusMsg = "no body to save"
		return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
	}
	dest, err := saveResponseBytes(bytes, resp, m.urlBar.Value())
	if err != nil {
		m.statusMsg = "save failed: " + err.Error()
	} else {
		m.statusMsg = "saved to " + dest
	}
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
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
