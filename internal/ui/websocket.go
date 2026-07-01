package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// WSState selects which view the WebSocket overlay is rendering.
//
//	WSHidden     the overlay is not shown
//	WSConfig     the connect form (a single ws:// URL input)
//	WSConnected  the live session: message log + send box
//
// There is no separate "connecting" state; the overlay enters WSConnected
// immediately on connect with connected=false and a "connecting…" system
// line, flipping to connected=true when the handshake completes.
type WSState int

const (
	WSHidden WSState = iota
	WSConfig
	WSConnected
)

// wsLineKind tags a log entry so the view can colour and prefix it.
type wsLineKind int

const (
	wsSent wsLineKind = iota // outgoing frame
	wsRecv                   // incoming frame
	wsSys                    // connect / disconnect / info
	wsErr                    // error
)

// wsMessage is one entry in the session log.
type wsMessage struct {
	kind wsLineKind
	text string
	at   time.Time
}

// WebSocket owns the WebSocket overlay: a connect form popped up on Ctrl+W,
// followed by a fullscreen session view that logs frames as they arrive and
// offers a send box. The network side (dial, read pump, send, close) lives in
// the app layer; this component only collects input and renders state.
type WebSocket struct {
	state WSState

	urlInput  textinput.Model
	sendInput textinput.Model

	messages     []wsMessage
	scrollOffset int
	// autoScroll keeps the log pinned to the newest line until the user
	// scrolls up; any manual scroll turns it off, and jumping to the bottom
	// (End / new send) turns it back on.
	autoScroll bool

	connected    bool
	closed       bool
	closedReason string

	width, height int
}

// WSConnectMsg / WSEventMsg / WSClosedMsg / WSErrorMsg are the tea.Msg types
// the app layer exchanges with the network side. They live here so both the
// component and the app import them from one place.

// WSEventMsg carries one incoming frame or connection event. The concrete
// Event type lives in internal/wsconn; the app translates it into log
// appends, so this package stays dependency-free of wsconn.
type WSEventMsg struct {
	// Kind mirrors wsconn.EventKind; Text/Data/Err carry the payload. The
	// app fills these in from a wsconn.Event.
	Kind int
	Text string
	Err  string
}

// WSClosedMsg signals the read pump's channel closed (connection ended).
type WSClosedMsg struct{}

// WSErrorMsg reports a dial or send failure.
type WSErrorMsg struct {
	Err string
}

// NewWebSocket constructs a hidden WebSocket overlay.
func NewWebSocket() WebSocket {
	url := textinput.New()
	url.Placeholder = "wss://example.com/socket"
	url.CharLimit = 2048
	url.Width = 60

	send := textinput.New()
	send.Placeholder = "message to send"
	send.CharLimit = 65536
	send.Width = 60

	return WebSocket{
		state:      WSHidden,
		urlInput:   url,
		sendInput:  send,
		autoScroll: true,
	}
}

func (m WebSocket) State() WSState    { return m.state }
func (m *WebSocket) SetSize(w, h int) { m.width = w; m.height = h }

// OpenConfig shows the connect form, prefilling the URL (typically the app's
// URL bar value). Resets any prior session.
func (m *WebSocket) OpenConfig(url string) {
	m.state = WSConfig
	m.messages = nil
	m.scrollOffset = 0
	m.autoScroll = true
	m.connected = false
	m.closed = false
	m.closedReason = ""
	m.sendInput.Reset()
	m.urlInput.SetValue(url)
	m.urlInput.CursorEnd()
	m.urlInput.Focus()
}

// Close hides the overlay. The app is responsible for tearing down any live
// connection before calling this.
func (m *WebSocket) Close() {
	m.state = WSHidden
	m.urlInput.Blur()
	m.sendInput.Blur()
}

// ConfigURL returns the trimmed URL from the connect form.
func (m WebSocket) ConfigURL() string { return strings.TrimSpace(m.urlInput.Value()) }

// SendInput returns the current send-box text.
func (m WebSocket) SendInput() string { return m.sendInput.Value() }

// ClearSendInput empties the send box after a message is dispatched.
func (m *WebSocket) ClearSendInput() { m.sendInput.SetValue("") }

// StartConnecting transitions to the session view in the not-yet-connected
// state, logging the dial target. Send is disabled until MarkConnected.
func (m *WebSocket) StartConnecting(url string) {
	m.state = WSConnected
	m.connected = false
	m.closed = false
	m.closedReason = ""
	m.urlInput.Blur()
	m.append(wsSys, "connecting to "+url+" …")
}

// MarkConnected flips the session to connected and focuses the send box.
func (m *WebSocket) MarkConnected() {
	m.connected = true
	m.append(wsSys, "connected")
	m.sendInput.Focus()
}

// MarkDisconnected records that the connection ended. Idempotent so both an
// error event and the subsequent channel-close can call it.
func (m *WebSocket) MarkDisconnected(reason string) {
	if m.closed {
		return
	}
	m.closed = true
	m.connected = false
	m.closedReason = reason
	m.sendInput.Blur()
	msg := "disconnected"
	if reason != "" {
		msg += " (" + reason + ")"
	}
	m.append(wsSys, msg)
}

// AppendSent / AppendReceived / AppendSystem / AppendError add a log entry.
func (m *WebSocket) AppendSent(text string)     { m.append(wsSent, text) }
func (m *WebSocket) AppendReceived(text string) { m.append(wsRecv, text) }
func (m *WebSocket) AppendSystem(text string)   { m.append(wsSys, text) }
func (m *WebSocket) AppendError(text string)    { m.append(wsErr, text) }

func (m *WebSocket) append(kind wsLineKind, text string) {
	m.messages = append(m.messages, wsMessage{kind: kind, text: text, at: time.Now()})
	if m.autoScroll {
		m.scrollOffset = m.maxScroll()
	}
}

// --- Update ---

// Update handles input the app does not intercept: URL/send-box editing and
// log scrolling. The app handles Enter (connect / send) and Esc (close)
// itself so the network side stays at the app layer.
func (m WebSocket) Update(msg tea.Msg) (WebSocket, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch m.state {
	case WSConfig:
		var cmd tea.Cmd
		m.urlInput, cmd = m.urlInput.Update(msg)
		return m, cmd
	case WSConnected:
		switch keyMsg.String() {
		case "up":
			m.autoScroll = false
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
			return m, nil
		case "down":
			if m.scrollOffset < m.maxScroll() {
				m.scrollOffset++
			}
			if m.scrollOffset >= m.maxScroll() {
				m.autoScroll = true
			}
			return m, nil
		case "pgup":
			m.autoScroll = false
			m.scrollOffset -= 10
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			return m, nil
		case "pgdown":
			m.scrollOffset += 10
			if m.scrollOffset >= m.maxScroll() {
				m.scrollOffset = m.maxScroll()
				m.autoScroll = true
			}
			return m, nil
		}
		// Everything else edits the send box (disabled once closed).
		if m.connected {
			var cmd tea.Cmd
			m.sendInput, cmd = m.sendInput.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// --- View ---

// logLines flattens the message log into rendered, coloured lines. A message
// spanning multiple physical lines contributes one entry per line so the
// scroll window can page through long payloads.
func (m WebSocket) logLines() []string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleFor := func(k wsLineKind) (lipgloss.Style, string) {
		switch k {
		case wsSent:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("39")), "▶ "
		case wsRecv:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("42")), "◀ "
		case wsErr:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("9")), "! "
		default:
			return dim, "· "
		}
	}
	var out []string
	for _, msg := range m.messages {
		st, prefix := styleFor(msg.kind)
		ts := dim.Render(msg.at.Format("15:04:05") + " ")
		lines := strings.Split(msg.text, "\n")
		for i, ln := range lines {
			p := prefix
			if i > 0 {
				p = "  " // continuation lines align under the first
			}
			out = append(out, ts+st.Render(p+ln))
		}
	}
	return out
}

// visibleLines is how many log lines fit in the session body.
func (m WebSocket) visibleLines() int {
	v := m.height - 10
	if v < 5 {
		v = 5
	}
	return v
}

// maxScroll is the largest scrollOffset that still keeps a full window (or
// the whole log if shorter) in view.
func (m WebSocket) maxScroll() int {
	n := len(m.logLines()) - m.visibleLines()
	if n < 0 {
		return 0
	}
	return n
}

func (m WebSocket) View() string {
	switch m.state {
	case WSConfig:
		return m.viewConfig()
	case WSConnected:
		return m.viewSession()
	}
	return ""
}

func (m WebSocket) viewConfig() string {
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	body := strings.Join([]string{
		bold.Render("WebSocket — connect"),
		"",
		"URL: " + m.urlInput.View(),
		dim.Render("  headers and auth from the request editor are sent with the handshake"),
		"",
		dim.Render("Enter: connect  ·  Esc: cancel"),
	}, "\n")
	return m.box(body)
}

func (m WebSocket) viewSession() string {
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	header := bold.Render("WebSocket session")
	switch {
	case m.closed:
		header += "  " + dim.Render("(disconnected)")
	case m.connected:
		header += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("(connected)")
	default:
		header += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("(connecting…)")
	}

	lines := m.logLines()
	visible := m.visibleLines()
	start := m.scrollOffset
	if start > m.maxScroll() {
		start = m.maxScroll()
	}
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(lines) {
		end = len(lines)
	}
	window := lines[start:end]
	if len(window) == 0 {
		window = []string{dim.Render("  (no messages yet)")}
	}

	var sendRow string
	if m.connected {
		sendRow = "send: " + m.sendInput.View()
	} else {
		sendRow = dim.Render("send: (unavailable — not connected)")
	}

	hint := dim.Render("Enter: send  ·  ↑/↓ PgUp/PgDn: scroll  ·  Esc: disconnect & close")

	body := strings.Join([]string{
		header,
		"",
		strings.Join(window, "\n"),
		"",
		sendRow,
		hint,
	}, "\n")
	return m.box(body)
}

func (m WebSocket) box(body string) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("230")).
		Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
