package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/lea/pollen/internal/history"
	"github.com/lea/pollen/internal/httpx"
	"github.com/lea/pollen/internal/ui"
)

// sendResultMsg carries the outcome of an asynchronous request back to the
// main loop. `gen` matches Model.requestGen at the moment the request was
// dispatched — responses with a stale gen are discarded so a slower older
// request can't overwrite a newer one.
type sendResultMsg struct {
	entry history.Entry
	gen   int
}

// currentRequest snapshots all input panels into a single history.Request.
// Authorization is built from the Auth panel unless an explicit Authorization
// header already exists in Headers (user-supplied entries win).
func (m Model) currentRequest() history.Request {
	headers := m.headers.Values()
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

// applyEntry restores a history entry into the editor panels. Auth state is
// cleared on restore — the entry's Authorization (if any) is already in
// Headers, so injecting more would double up.
func (m *Model) applyEntry(e history.Entry) {
	m.method.Set(e.Request.Method)
	urlOnly, params := splitURL(e.Request.URL)
	m.urlBar.SetValue(urlOnly)
	m.query.Set(params)
	m.auth.Reset()
	m.headers.Set(e.Request.Headers)
	m.body.Set(e.Request.BodyType, e.Request.Body)
	if e.Response != nil {
		m.response.SetResponse(e.Response, e.Request.URL)
	} else if e.Error != "" {
		m.response.SetError(e.Error)
	}
}

// sendRequest builds the request, expands {{vars}}, then dispatches the
// actual HTTP call as a tea.Cmd. Bumps requestGen so concurrent Sends can
// be disambiguated when their results arrive.
func (m *Model) sendRequest() tea.Cmd {
	req := m.currentRequest()
	// Expand {{varName}} tokens before sending. Both the actual HTTP request
	// and the history entry use the expanded form so the user always sees
	// "what we sent" verbatim. (Trade-off: secrets stored in env leak to
	// history.json — documented in README.)
	lastResp := m.response.CurrentResponse()
	expand := func(s string) string {
		return expandResponseVars(m.env.Expand(s), lastResp)
	}
	req.URL = expand(req.URL)
	req.Body = expand(req.Body)
	for i := range req.Headers {
		req.Headers[i].Value = expand(req.Headers[i].Value)
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
