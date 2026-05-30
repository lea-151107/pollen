package httpx

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lea-151107/pollen/internal/history"
)

func TestDo_GET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if got := r.Header.Get("X-Test"); got != "yes" {
			t.Errorf("expected X-Test: yes, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	resp, err := Do(history.Request{
		Method:  "GET",
		URL:     srv.URL,
		Headers: []history.Header{{Key: "X-Test", Value: "yes"}},
	})
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("expected 200, got %d", resp.Status)
	}
	if resp.Body != `{"ok":true}` {
		t.Errorf("unexpected body: %s", resp.Body)
	}
}

func TestDo_POST_JSON_AutoContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected json content-type, got %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"a":1}` {
			t.Errorf("unexpected body: %s", body)
		}
		w.WriteHeader(201)
	}))
	defer srv.Close()

	resp, err := Do(history.Request{
		Method:   "POST",
		URL:      srv.URL,
		BodyType: history.BodyJSON,
		Body:     `{"a":1}`,
	})
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	if resp.Status != 201 {
		t.Errorf("expected 201, got %d", resp.Status)
	}
}

func TestDo_POST_Form(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("name") != "test" || r.PostForm.Get("foo") != "bar" {
			t.Errorf("unexpected form: %v", r.PostForm)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	resp, err := Do(history.Request{
		Method:   "POST",
		URL:      srv.URL,
		BodyType: history.BodyForm,
		Body:     "name=test\nfoo=bar",
	})
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("expected 200, got %d", resp.Status)
	}
}

func TestDo_TruncatesOversizedBody(t *testing.T) {
	// Send slightly more than the cap to force truncation.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		// Write cap+1024 bytes of NUL to make the receiver hit the limit.
		chunk := make([]byte, 64*1024)
		written := 0
		for written < MaxResponseBytes+1024 {
			n, err := w.Write(chunk)
			if err != nil {
				return
			}
			written += n
		}
	}))
	defer srv.Close()

	resp, err := Do(history.Request{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !resp.Truncated {
		t.Errorf("expected Truncated=true")
	}
	if resp.SizeBytes != MaxResponseBytes {
		t.Errorf("expected SizeBytes=%d, got %d", MaxResponseBytes, resp.SizeBytes)
	}
	if len(resp.BodyBytes) != MaxResponseBytes {
		t.Errorf("expected BodyBytes len=%d, got %d", MaxResponseBytes, len(resp.BodyBytes))
	}
}

func TestDo_DoesNotTruncateSmallBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("small"))
	}))
	defer srv.Close()

	resp, err := Do(history.Request{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Truncated {
		t.Error("did not expect truncation for small body")
	}
}

func TestDo_TLSVerify(t *testing.T) {
	// httptest.NewTLSServer uses a self-signed cert that the default client
	// does not trust.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Default: should fail with x509 error.
	SkipTLSVerify.Store(false)
	if _, err := Do(history.Request{Method: "GET", URL: srv.URL}); err == nil {
		t.Error("expected TLS verification error with default settings")
	}

	// Skip verify: should succeed.
	SkipTLSVerify.Store(true)
	defer SkipTLSVerify.Store(false) // restore for other tests
	resp, err := Do(history.Request{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("expected success with SkipTLSVerify, got %v", err)
	}
	if resp.Status != 200 || resp.Body != "ok" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestDo_GraphQLPostsJSONEnvelope(t *testing.T) {
	var got struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %q", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("server failed to parse envelope: %v (body=%q)", err, body)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	defer srv.Close()

	resp, err := Do(history.Request{
		Method:           "POST",
		URL:              srv.URL,
		Body:             "{ user(id: $id) { name } }",
		BodyType:         history.BodyGraphQL,
		GraphQLVariables: `{"id": 42}`,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("unexpected status: %d", resp.Status)
	}
	if got.Query != "{ user(id: $id) { name } }" {
		t.Errorf("query in envelope: got %q", got.Query)
	}
	if v, _ := got.Variables["id"].(float64); v != 42 {
		t.Errorf("variables.id: got %v", got.Variables["id"])
	}
}

func TestDo_GraphQLEmptyVariablesOmitsField(t *testing.T) {
	var raw map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &raw)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	if _, err := Do(history.Request{
		Method:   "POST",
		URL:      srv.URL,
		Body:     "{ ping }",
		BodyType: history.BodyGraphQL,
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if _, present := raw["variables"]; present {
		t.Errorf("variables key should be absent when GraphQLVariables is empty; got %v", raw)
	}
}

func TestDo_GraphQLInvalidVariablesDropped(t *testing.T) {
	var raw map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &raw)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	// "not json" is invalid JSON → variables should be omitted, request
	// still goes through. (Server will likely reject; pollen doesn't.)
	if _, err := Do(history.Request{
		Method:           "POST",
		URL:              srv.URL,
		Body:             "{ ping }",
		BodyType:         history.BodyGraphQL,
		GraphQLVariables: "not json",
	}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if _, present := raw["variables"]; present {
		t.Errorf("invalid variables should be silently dropped; got %v", raw)
	}
}

func TestDo_MalformedProxyURLFallsThroughToDirect(t *testing.T) {
	// A malformed proxy_url used to fall through to http.DefaultTransport's
	// ProxyFromEnvironment, silently routing via $HTTP_PROXY when set —
	// the opposite of what the user asked for. After the fix Do() must
	// force tr.Proxy = nil so the request goes direct.
	//
	// The localhost httptest server typically bypasses env proxies anyway
	// (NO_PROXY), so this test mainly pins the "malformed proxy doesn't
	// break basic requests" invariant rather than catching env-proxy leak
	// directly. The behaviour change is in client.go around line 82.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	oldProxy := ProxyURL
	ProxyURL = "://invalid" // url.Parse rejects empty scheme
	defer func() { ProxyURL = oldProxy }()

	resp, err := Do(history.Request{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("expected success with malformed proxy_url, got %v", err)
	}
	if resp.Status != 200 || resp.Body != "ok" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestDo_InvalidURL(t *testing.T) {
	_, err := Do(history.Request{Method: "GET", URL: "://bad"})
	if err == nil {
		t.Error("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "missing protocol") && !strings.Contains(err.Error(), "parse") {
		// Just want some error, the exact text varies.
		t.Logf("error: %v", err)
	}
}
