package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/ui"
	"github.com/lea-151107/pollen/internal/wsconn"
)

// wsConnectedMsg is returned by the dial Cmd once the handshake succeeds. It
// is internal to the app layer (it carries a live *wsconn.Conn, so it can't
// live in package ui without a dependency cycle).
type wsConnectedMsg struct {
	conn *wsconn.Conn
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

	m.ws.StartConnecting(url)
	cfg := wsconn.Config{URL: url, Headers: headers}
	return func() tea.Msg {
		conn, err := wsconn.Dial(cfg)
		if err != nil {
			return ui.WSErrorMsg{Err: err.Error()}
		}
		return wsConnectedMsg{conn: conn}
	}
}

// wsSend dispatches one text frame on the live connection.
func (m *Model) wsSend(text string) tea.Cmd {
	conn := m.wsConn
	return func() tea.Msg {
		if conn == nil {
			return ui.WSErrorMsg{Err: "not connected"}
		}
		if err := conn.Send(text); err != nil {
			return ui.WSErrorMsg{Err: err.Error()}
		}
		return nil
	}
}

// closeWebSocket tears down any live connection and hides the overlay.
func (m *Model) closeWebSocket() tea.Cmd {
	if m.wsConn != nil {
		_ = m.wsConn.Close()
		m.wsConn = nil
	}
	m.wsCh = nil
	m.ws.Close()
	return nil
}

// nextWSEventCmd reads one Event from the read pump and maps it to a tea.Msg.
// A closed channel becomes WSClosedMsg. Mirrors nextIntruderResultCmd.
func nextWSEventCmd(ch <-chan wsconn.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return ui.WSClosedMsg{}
		}
		return ui.WSEventMsg{
			Kind: int(ev.Kind),
			Text: ev.Text,
			Err:  errString(ev.Err),
		}
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
