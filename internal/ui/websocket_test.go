package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWebSocket_ConfigToConnectedLifecycle(t *testing.T) {
	ws := NewWebSocket()
	ws.SetSize(80, 24)
	if ws.State() != WSHidden {
		t.Fatalf("new overlay should be hidden")
	}

	ws.OpenConfig("wss://example.com")
	if ws.State() != WSConfig {
		t.Fatalf("OpenConfig should enter WSConfig")
	}
	if ws.ConfigURL() != "wss://example.com" {
		t.Errorf("ConfigURL = %q", ws.ConfigURL())
	}

	ws.StartConnecting("wss://example.com")
	if ws.State() != WSConnected {
		t.Fatalf("StartConnecting should enter WSConnected")
	}
	// Not connected yet: the session view must advertise it can't send.
	if !strings.Contains(ws.View(), "not connected") {
		t.Errorf("connecting view should indicate send is unavailable")
	}

	ws.MarkConnected()
	ws.AppendReceived("hello")
	if !strings.Contains(ws.View(), "hello") {
		t.Errorf("received message should appear in the log")
	}
	if !strings.Contains(ws.View(), "connected") {
		t.Errorf("connected session should show connected state")
	}
}

func TestWebSocket_MarkDisconnectedIsIdempotent(t *testing.T) {
	ws := NewWebSocket()
	ws.SetSize(80, 24)
	ws.OpenConfig("wss://x")
	ws.StartConnecting("wss://x")
	ws.MarkConnected()

	ws.MarkDisconnected("error")
	ws.MarkDisconnected("closed") // second call must not add another log line

	view := ws.View()
	// The log line carries the reason as "disconnected (<reason>)"; the
	// header badge reads "(disconnected)" without a reason. Only the first
	// reason should have been logged.
	if got := strings.Count(view, "disconnected ("); got != 1 {
		t.Errorf("disconnect should be logged exactly once, got %d", got)
	}
	if strings.Contains(view, "closed)") {
		t.Errorf("second MarkDisconnected must not overwrite the first reason")
	}
}

func TestWebSocket_SendBoxEditsWhileConnected(t *testing.T) {
	ws := NewWebSocket()
	ws.SetSize(80, 24)
	ws.OpenConfig("wss://x")
	ws.StartConnecting("wss://x")
	ws.MarkConnected()

	ws, _ = ws.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")})
	if ws.SendInput() != "hi" {
		t.Errorf("send box should capture typed text, got %q", ws.SendInput())
	}

	ws.ClearSendInput()
	if ws.SendInput() != "" {
		t.Errorf("ClearSendInput should empty the send box")
	}
}
