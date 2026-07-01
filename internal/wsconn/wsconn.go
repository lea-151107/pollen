// Package wsconn is a thin wrapper around coder/websocket that adapts a
// WebSocket connection to pollen's Bubble Tea event loop: the caller drives
// the handshake with Dial, reads incoming frames off an Events channel (one
// tea.Msg per frame), and sends text with Send. Proxy / TLS settings are
// inherited from the shared httpx transport snapshot so a WebSocket dial
// honors the same config as an HTTP request.
package wsconn

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/httpx"
)

// handshakeTimeout bounds the upgrade request. The connection itself outlives
// this — only the initial dial is subject to it.
const handshakeTimeout = 30 * time.Second

// readLimit caps a single incoming frame. coder/websocket defaults to 32 KiB;
// bump it so ordinary JSON payloads aren't rejected, while still guarding
// against a runaway server.
const readLimit = 8 * 1024 * 1024 // 8 MiB

// EventKind classifies an Event pushed onto a Conn's channel.
type EventKind int

const (
	// EventText is an incoming text frame; see Event.Text.
	EventText EventKind = iota
	// EventBinary is an incoming binary frame; see Event.Data.
	EventBinary
	// EventError is an abnormal read failure; see Event.Err. Terminal.
	EventError
	// EventClosed is a clean close (peer close frame or local Close). Terminal.
	EventClosed
)

// Event is a single item read from the connection.
type Event struct {
	Kind EventKind
	Text string
	Data []byte
	Err  error
}

// Config describes a dial. Headers are applied to the handshake request, so
// pollen's Authorization / custom headers carry into the upgrade.
type Config struct {
	URL     string
	Headers []history.Header
}

// Conn is a live WebSocket connection with a background read pump.
type Conn struct {
	ws     *websocket.Conn
	events chan Event
	ctx    context.Context
	cancel context.CancelFunc
}

// Dial performs the WebSocket handshake and, on success, starts a read pump
// that streams incoming frames onto Events(). The returned Conn owns its own
// lifetime context; Close cancels it and shuts the pump down.
func Dial(cfg Config) (*Conn, error) {
	hdr := http.Header{}
	for _, h := range cfg.Headers {
		if h.Key == "" {
			continue
		}
		hdr.Add(h.Key, h.Value)
	}

	dialCtx, dialCancel := context.WithTimeout(context.Background(), handshakeTimeout)
	defer dialCancel()

	ws, resp, err := websocket.Dial(dialCtx, cfg.URL, &websocket.DialOptions{
		HTTPClient: httpx.NewHandshakeClient(),
		HTTPHeader: hdr,
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	ws.SetReadLimit(readLimit)

	ctx, cancel := context.WithCancel(context.Background())
	c := &Conn{
		ws:     ws,
		events: make(chan Event, 32),
		ctx:    ctx,
		cancel: cancel,
	}
	go c.readPump()
	return c, nil
}

// Events returns the channel of incoming frames. It is closed after a
// terminal EventError / EventClosed is delivered, so a range over it drains
// cleanly.
func (c *Conn) Events() <-chan Event { return c.events }

// readPump reads frames until an error or close and then terminates, closing
// the events channel so the consumer's range loop ends.
func (c *Conn) readPump() {
	defer close(c.events)
	for {
		typ, data, err := c.ws.Read(c.ctx)
		if err != nil {
			c.emit(c.classifyReadErr(err))
			return
		}
		switch typ {
		case websocket.MessageText:
			c.emit(Event{Kind: EventText, Text: string(data)})
		case websocket.MessageBinary:
			c.emit(Event{Kind: EventBinary, Data: data})
		}
	}
}

// classifyReadErr maps a Read error to a terminal Event. A local Close or a
// peer close frame is a clean EventClosed; anything else is EventError.
func (c *Conn) classifyReadErr(err error) Event {
	if c.ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return Event{Kind: EventClosed, Err: err}
	}
	if websocket.CloseStatus(err) != -1 {
		return Event{Kind: EventClosed, Err: err}
	}
	return Event{Kind: EventError, Err: err}
}

// emit delivers e unless the connection is already torn down.
func (c *Conn) emit(e Event) {
	select {
	case c.events <- e:
	case <-c.ctx.Done():
	}
}

// Send writes a text frame. It is safe to call from the UI goroutine.
func (c *Conn) Send(text string) error {
	return c.ws.Write(c.ctx, websocket.MessageText, []byte(text))
}

// Close initiates a normal closure and cancels the read pump. Safe to call
// more than once.
func (c *Conn) Close() error {
	c.cancel()
	return c.ws.Close(websocket.StatusNormalClosure, "")
}
