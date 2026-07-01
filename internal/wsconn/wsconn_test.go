package wsconn

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/lea-151107/pollen/internal/history"
)

// echoServer accepts a WebSocket and echoes every frame back. If want is
// non-empty it also asserts the handshake carried that header value.
func echoServer(t *testing.T, headerKey, want string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want != "" && r.Header.Get(headerKey) != want {
			http.Error(w, "missing header", http.StatusBadRequest)
			return
		}
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
}

func TestDial_SendReceiveClose(t *testing.T) {
	srv := echoServer(t, "", "")
	defer srv.Close()

	conn, err := Dial(Config{URL: srv.URL})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	if err := conn.Send("hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case ev := <-conn.Events():
		if ev.Kind != EventText || ev.Text != "hello" {
			t.Errorf("got %+v, want EventText \"hello\"", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for echo")
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// After Close the events channel must drain and close so a range ends.
	drained := make(chan struct{})
	go func() {
		for range conn.Events() {
		}
		close(drained)
	}()
	select {
	case <-drained:
	case <-time.After(5 * time.Second):
		t.Fatal("events channel did not close after Close")
	}
}

func TestDial_CarriesHandshakeHeaders(t *testing.T) {
	srv := echoServer(t, "Authorization", "Bearer tok")
	defer srv.Close()

	conn, err := Dial(Config{
		URL:     srv.URL,
		Headers: []history.Header{{Key: "Authorization", Value: "Bearer tok"}},
	})
	if err != nil {
		t.Fatalf("Dial with header should succeed: %v", err)
	}
	_ = conn.Close()
}

func TestDial_HeaderMismatchFails(t *testing.T) {
	srv := echoServer(t, "Authorization", "Bearer tok")
	defer srv.Close()

	// No Authorization header → server rejects the upgrade.
	if _, err := Dial(Config{URL: srv.URL}); err == nil {
		t.Fatal("expected dial to fail without required header")
	}
}
