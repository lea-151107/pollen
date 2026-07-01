package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/wsconn"
)

// The WebSocket async messages are app-internal (they carry a live
// *wsconn.Conn / a wsconn.Event and a generation tag, so they can't live in
// package ui without a dependency cycle). Each carries the wsGen that was
// current when the work was dispatched; the handlers discard any result whose
// gen no longer matches Model.wsGen, so a cancelled or superseded connection
// attempt can't leak a socket or corrupt a newer session.

type wsConnectedMsg struct {
	conn *wsconn.Conn
	gen  int
}

type wsEventMsg struct {
	gen  int
	kind wsconn.EventKind
	text string
	err  string
}

type wsClosedMsg struct {
	gen int
}

type wsErrorMsg struct {
	gen int
	err string
}

// startWebSocket reads the connect form, expands variables and auth the same
// way an HTTP send does, and dials asynchronously. The overlay switches to
// the session view immediately (connecting…) so the handshake latency is
// visible.
func (m *Model) startWebSocket() tea.Cmd {
	rawURL := m.ws.ConfigURL()
	if rawURL == "" {
		return nil
	}
	lastResp := m.response.CurrentResponse()
	expand := func(s string) string {
		return expandResponseVars(m.env.Expand(s), lastResp)
	}
	url := expand(rawURL)

	// Reuse the request editor's headers + auth for the handshake.
	req := m.currentRequest()
	headers := make([]history.Header, 0, len(req.Headers))
	for _, h := range req.Headers {
		headers = append(headers, history.Header{Key: h.Key, Value: expand(h.Value)})
	}

	m.wsGen++
	gen := m.wsGen
	m.ws.StartConnecting(url)
	cfg := wsconn.Config{URL: url, Headers: headers}
	return func() tea.Msg {
		conn, err := wsconn.Dial(cfg)
		if err != nil {
			return wsErrorMsg{gen: gen, err: err.Error()}
		}
		return wsConnectedMsg{conn: conn, gen: gen}
	}
}

// wsSend dispatches one text frame on the live connection.
func (m *Model) wsSend(text string) tea.Cmd {
	conn := m.wsConn
	gen := m.wsGen
	return func() tea.Msg {
		if conn == nil {
			return wsErrorMsg{gen: gen, err: "not connected"}
		}
		if err := conn.Send(text); err != nil {
			return wsErrorMsg{gen: gen, err: err.Error()}
		}
		return nil
	}
}

// closeWebSocket tears down any live connection and hides the overlay. It
// bumps wsGen so any dial or read still in flight is treated as stale. The
// actual network close runs off the UI goroutine — coder/websocket's Close
// does a close handshake that can block for seconds on an unresponsive peer,
// which would otherwise freeze the TUI.
func (m *Model) closeWebSocket() tea.Cmd {
	m.wsGen++
	conn := m.wsConn
	m.wsConn = nil
	m.wsCh = nil
	m.ws.Close()
	return closeConnCmd(conn)
}

// closeConnCmd closes conn on a background goroutine (via a tea.Cmd), so the
// close handshake never blocks Update. Returns nil for a nil conn.
func closeConnCmd(conn *wsconn.Conn) tea.Cmd {
	if conn == nil {
		return nil
	}
	return func() tea.Msg {
		_ = conn.Close()
		return nil
	}
}

// nextWSEventCmd reads one Event from the read pump and maps it to a tea.Msg,
// tagged with gen. A closed channel becomes wsClosedMsg. Mirrors
// nextIntruderResultCmd.
func nextWSEventCmd(ch <-chan wsconn.Event, gen int) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return wsClosedMsg{gen: gen}
		}
		return wsEventMsg{
			gen:  gen,
			kind: ev.Kind,
			text: ev.Text,
			err:  errString(ev.Err),
		}
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
