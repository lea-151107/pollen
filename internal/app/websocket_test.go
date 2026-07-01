package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/coder/websocket"

	"github.com/lea-151107/pollen/internal/ui"
)

// TestWebSocket_CtrlWOpensConfig checks the Ctrl+W global binding opens the
// WebSocket connect form and prefills it with the URL bar value, driven
// through the app's own Update routing.
func TestWebSocket_CtrlWOpensConfig(t *testing.T) {
	m := newApplyTestModel(t)
	m.urlBar.SetValue("wss://example.com/socket")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)

	if m.ws.State() != ui.WSConfig {
		t.Fatalf("Ctrl+W should open the WebSocket config form, state=%d", m.ws.State())
	}
	if got := m.ws.ConfigURL(); got != "wss://example.com/socket" {
		t.Errorf("config URL = %q, want the URL bar value", got)
	}
}

// TestWebSocket_EscFromConfigCloses confirms Esc dismisses the connect form.
func TestWebSocket_EscFromConfigCloses(t *testing.T) {
	m := newApplyTestModel(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.ws.State() != ui.WSHidden {
		t.Errorf("Esc from the connect form should hide the overlay, state=%d", m.ws.State())
	}
}

// TestWebSocket_EmptyURLDoesNotConnect confirms Enter with a blank URL is a
// no-op (no dial Cmd, form stays open) rather than dialing an empty target.
func TestWebSocket_EmptyURLDoesNotConnect(t *testing.T) {
	m := newApplyTestModel(t)
	// Open with an empty URL bar.
	m.urlBar.SetValue("")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if cmd != nil {
		t.Errorf("Enter with empty URL should not dispatch a dial command")
	}
	if m.ws.State() != ui.WSConfig {
		t.Errorf("form should stay open on empty-URL Enter, state=%d", m.ws.State())
	}
}

// TestWebSocket_EndToEnd drives the whole app path — Ctrl+W, Enter to dial,
// the wsConnectedMsg handoff, a send, and the echoed frame arriving as a
// WSEventMsg — against a real echo server, exercising the same Cmd/Msg
// plumbing the TUI uses.
func TestWebSocket_EndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusInternalError, "")
		ctx := r.Context()
		for {
			typ, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			if err := c.Write(ctx, typ, data); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	m := newApplyTestModel(t)
	m.ws.SetSize(80, 24)
	m.urlBar.SetValue(srv.URL)

	// Ctrl+W → Enter dials.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	updated, dialCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if dialCmd == nil {
		t.Fatal("Enter with a URL should dispatch a dial command")
	}

	// Run the dial; expect a wsConnectedMsg.
	connMsg := dialCmd()
	if _, ok := connMsg.(wsConnectedMsg); !ok {
		t.Fatalf("dial should yield wsConnectedMsg, got %T (%v)", connMsg, connMsg)
	}
	updated, readCmd := m.Update(connMsg)
	m = updated.(Model)
	if m.ws.State() != ui.WSConnected || m.wsConn == nil {
		t.Fatal("model should be connected after wsConnectedMsg")
	}
	defer m.closeWebSocket()

	// Type a message and send it.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ping")})
	m = updated.(Model)
	updated, sendCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if sendCmd == nil {
		t.Fatal("Enter with text should dispatch a send command")
	}
	if msg := sendCmd(); msg != nil {
		if e, ok := msg.(ui.WSErrorMsg); ok {
			t.Fatalf("send failed: %s", e.Err)
		}
	}

	// Read the echoed frame back through the pump command.
	evMsg := runWithTimeout(t, readCmd)
	ev, ok := evMsg.(ui.WSEventMsg)
	if !ok {
		t.Fatalf("expected WSEventMsg, got %T (%v)", evMsg, evMsg)
	}
	updated, _ = m.Update(ev)
	m = updated.(Model)
	if !strings.Contains(m.ws.View(), "ping") {
		t.Errorf("echoed frame should appear in the session log")
	}
}

// runWithTimeout runs a tea.Cmd on a goroutine and fails if it doesn't
// return promptly (the read pump blocks until a frame arrives).
func runWithTimeout(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for websocket event")
		return nil
	}
}
