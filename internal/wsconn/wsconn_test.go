package wsconn

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/httpx"
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

// TestDial_CarriesSharedCookieJar proves the WebSocket handshake sends cookies
// from the shared httpx cookie jar, matching an HTTP request. Regression guard
// for the fix where NewHandshakeClient dropped the jar, so a session cookie set
// by a prior HTTP login was silently lost on the WS upgrade.
func TestDial_CarriesSharedCookieJar(t *testing.T) {
	// The server rejects the upgrade unless the expected session cookie rides
	// along, so a successful dial means the jar's cookie was sent.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session")
		if err != nil || c.Value != "tok" {
			http.Error(w, "missing session cookie", http.StatusUnauthorized)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		conn.Close(websocket.StatusNormalClosure, "")
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "tok"}})

	// Install the jar on the shared transport snapshot, restoring afterwards so
	// the global config doesn't leak into other tests.
	orig := httpx.Snapshot()
	c := orig
	c.CookieJar = jar
	httpx.SetConfig(c)
	defer httpx.SetConfig(orig)

	conn, err := Dial(Config{URL: srv.URL})
	if err != nil {
		t.Fatalf("dial should succeed with the shared cookie jar carrying the session cookie: %v", err)
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
