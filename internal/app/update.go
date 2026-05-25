package app

import (
	"fmt"
	"net/url"
	"runtime"
	"sort"
	"strings"
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
	gen   int // matches Model.requestGen at the time of dispatch
}

type clearStatusMsg struct{ gen int }

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
	headers := m.headers.Values()
	// Build the Authorization header from the Auth panel's raw inputs.
	// Explicit Headers entries take precedence — we don't override what the
	// user wrote manually.
	if authVal := buildAuthFromPanel(m.auth); authVal != "" && !hasHeader(headers, "Authorization") {
		headers = append(headers, history.Header{Key: "Authorization", Value: authVal})
	}
	return history.Request{
		Method:   m.method.Value(),
		URL:      composeURL(m.urlBar.Value(), m.query.Values()),
		Headers:  headers,
		Body:     m.body.Value(),
		BodyType: m.body.Type(),
	}
}

// buildAuthFromPanel maps the UI Auth panel's selection to an HTTP
// Authorization header value via httpx.BuildAuthHeader.
func buildAuthFromPanel(a ui.Auth) string {
	switch a.Type() {
	case ui.AuthBearer:
		return httpx.BuildAuthHeader(httpx.AuthBearer, a.Token(), "", "")
	case ui.AuthBasic:
		u, p := a.Credentials()
		return httpx.BuildAuthHeader(httpx.AuthBasic, "", u, p)
	}
	return ""
}

func hasHeader(headers []history.Header, key string) bool {
	for _, h := range headers {
		if strings.EqualFold(h.Key, key) {
			return true
		}
	}
	return false
}

// composeURL merges the query parameters from the Query panel into the URL.
// Uses net/url when the URL is parseable; falls back to plain concatenation
// when the URL contains `{{var}}` tokens (env expansion happens later).
func composeURL(rawURL string, params []ui.Param) string {
	if len(params) == 0 {
		return rawURL
	}
	if !strings.Contains(rawURL, "{{") {
		if u, err := url.Parse(rawURL); err == nil {
			q := u.Query()
			for _, p := range params {
				q.Add(p.Key, p.Value)
			}
			u.RawQuery = q.Encode()
			return u.String()
		}
	}
	// Fallback: simple concat with proper escaping. {{...}} tokens stay intact.
	var b strings.Builder
	b.WriteString(rawURL)
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	for i, p := range params {
		if i == 0 {
			b.WriteString(sep)
		} else {
			b.WriteString("&")
		}
		b.WriteString(url.QueryEscape(p.Key))
		b.WriteString("=")
		b.WriteString(url.QueryEscape(p.Value))
	}
	return b.String()
}

func (m *Model) applyEntry(e history.Entry) {
	m.method.Set(e.Request.Method)
	urlOnly, params := splitURL(e.Request.URL)
	m.urlBar.SetValue(urlOnly)
	m.query.Set(params)
	// Restored entries already carry Authorization in Headers; reset the Auth
	// panel so it doesn't double-inject a different value next time.
	m.auth.Reset()
	m.headers.Set(e.Request.Headers)
	m.body.Set(e.Request.BodyType, e.Request.Body)
	if e.Response != nil {
		m.response.SetResponse(e.Response, e.Request.URL)
	} else if e.Error != "" {
		m.response.SetError(e.Error)
	}
}

// splitURL separates a full URL into the URL-without-query and a slice of
// query parameters, sorted by key for stable display order. If the URL can't
// be parsed (e.g. it contains {{var}} tokens) the full URL is returned as-is
// and no params are extracted.
func splitURL(rawURL string) (string, []ui.Param) {
	if strings.Contains(rawURL, "{{") {
		return rawURL, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.RawQuery == "" {
		return rawURL, nil
	}
	values := u.Query()
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var params []ui.Param
	for _, k := range keys {
		for _, v := range values[k] {
			params = append(params, ui.Param{Key: k, Value: v})
		}
	}
	u.RawQuery = ""
	return u.String(), params
}

// deliverCopy puts `content` on the clipboard, or on disk if clipboard fails,
// and reports the outcome through statusMsg with appropriate kind.
func (m *Model) deliverCopy(content, label string) {
	mode, path, err := copyOrFallback(content)
	switch {
	case err != nil:
		// err means BOTH the system clipboard AND the file fallback failed.
		// The xclip-install hint isn't useful here — the real problem is
		// likely disk/permission, so spell that out instead.
		m.setStatus(statusError, "copy failed (clipboard and file fallback): "+err.Error())
	case mode == copyClipboard:
		m.setStatus(statusOK, "copied as "+label)
	case mode == copyFile:
		m.setStatus(statusWarn, "clipboard unavailable - wrote "+label+" to "+path+clipboardHint())
	}
}

func (m *Model) saveResponse() tea.Cmd {
	bytes := m.response.CurrentBytes()
	resp := m.response.CurrentResponse()
	if len(bytes) == 0 {
		m.setStatus(statusError, "no body to save")
		return m.statusTick(2 * time.Second)
	}
	// Use the URL of the request that produced this response, NOT m.urlBar
	// which the user may have edited since the response arrived.
	dest, err := saveResponseBytes(bytes, resp, m.response.RequestURL())
	if err != nil {
		m.setStatus(statusError, "save failed: "+err.Error())
	} else {
		m.setStatus(statusOK, "saved to "+dest)
	}
	return m.statusTick(2 * time.Second)
}

func (m *Model) sendRequest() tea.Cmd {
	req := m.currentRequest()
	// Expand {{varName}} tokens before sending. Both the actual HTTP request
	// and the history entry use the expanded form so the user always sees
	// "what we sent" verbatim. (Trade-off: secrets stored in env leak to
	// history.json — documented in README.)
	req.URL = m.env.Expand(req.URL)
	req.Body = m.env.Expand(req.Body)
	for i := range req.Headers {
		req.Headers[i].Value = m.env.Expand(req.Headers[i].Value)
	}
	if req.URL == "" {
		m.response.SetError("URL is empty")
		return nil
	}
	m.response.SetLoading(true)
	m.requestGen++
	gen := m.requestGen
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
		return sendResultMsg{entry: entry, gen: gen}
	}
}
