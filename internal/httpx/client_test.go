package httpx

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

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
	// Use a small cap so the test stays fast and deterministic.
	orig := Snapshot()
	cap := 256 * 1024
	c := orig
	c.MaxResponseBytes = cap
	SetConfig(c)
	defer SetConfig(orig)

	// Send slightly more than the cap to force truncation.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		// Write cap+1024 bytes of NUL to make the receiver hit the limit.
		chunk := make([]byte, 64*1024)
		written := 0
		for written < cap+1024 {
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
	if resp.SizeBytes != cap {
		t.Errorf("expected SizeBytes=%d, got %d", cap, resp.SizeBytes)
	}
	if len(resp.BodyBytes) != cap {
		t.Errorf("expected BodyBytes len=%d, got %d", cap, len(resp.BodyBytes))
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

	orig := Snapshot()
	defer SetConfig(orig) // restore for other tests

	// Default: should fail with x509 error.
	c := orig
	c.SkipTLSVerify = false
	SetConfig(c)
	if _, err := Do(history.Request{Method: "GET", URL: srv.URL}); err == nil {
		t.Error("expected TLS verification error with default settings")
	}

	// Skip verify: should succeed.
	c.SkipTLSVerify = true
	SetConfig(c)
	resp, err := Do(history.Request{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("expected success with SkipTLSVerify, got %v", err)
	}
	if resp.Status != 200 || resp.Body != "ok" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestParseMultipartLines(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []MultipartPart
	}{
		{"empty", "", nil},
		{"text part", "name=alice", []MultipartPart{{Name: "name", Value: "alice"}}},
		{"file part", "upload=@/tmp/x.png", []MultipartPart{{Name: "upload", File: "/tmp/x.png"}}},
		{"file with type", "img=@/tmp/x.png;type=image/png",
			[]MultipartPart{{Name: "img", File: "/tmp/x.png", ContentType: "image/png"}}},
		{"mixed",
			"meta={\"k\":1}\nupload=@/etc/hosts;type=text/plain",
			[]MultipartPart{
				{Name: "meta", Value: `{"k":1}`},
				{Name: "upload", File: "/etc/hosts", ContentType: "text/plain"},
			},
		},
		{"blank lines skipped", "\n\nname=alice\n\n",
			[]MultipartPart{{Name: "name", Value: "alice"}}},
		{"empty key skipped", "=value\nname=alice",
			[]MultipartPart{{Name: "name", Value: "alice"}}},
		{"line without = skipped", "no-equals-sign\nname=alice",
			[]MultipartPart{{Name: "name", Value: "alice"}}},
	}
	for _, c := range cases {
		got := ParseMultipartLines(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%s: len got %d want %d (got=%v)", c.name, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s[%d]: got %+v want %+v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

func TestDo_MultipartFormDataAssemblesParts(t *testing.T) {
	// Stage a small file to upload so the test exercises the file
	// streaming path.
	tmpdir := t.TempDir()
	filePath := tmpdir + "/upload.txt"
	if err := os.WriteFile(filePath, []byte("hello from a file"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var gotForm map[string][]string
	var gotFiles map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server-side multipart parse.
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("server ParseMultipartForm: %v", err)
		}
		gotForm = map[string][]string{}
		for k, vs := range r.MultipartForm.Value {
			gotForm[k] = vs
		}
		gotFiles = map[string][]string{}
		for k, fhs := range r.MultipartForm.File {
			for _, fh := range fhs {
				f, _ := fh.Open()
				b, _ := io.ReadAll(f)
				gotFiles[k] = append(gotFiles[k], string(b))
				_ = f.Close()
			}
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	body := "meta=hello world\nfile=@" + filePath + ";type=text/plain"
	_, err := Do(history.Request{
		Method:   "POST",
		URL:      srv.URL,
		Body:     body,
		BodyType: history.BodyMultipart,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got := gotForm["meta"]; len(got) != 1 || got[0] != "hello world" {
		t.Errorf("meta: %v", got)
	}
	if got := gotFiles["file"]; len(got) != 1 || got[0] != "hello from a file" {
		t.Errorf("file content: %v", got)
	}
}

func TestDo_MultipartMissingFileSurfacesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	_, err := Do(history.Request{
		Method:   "POST",
		URL:      srv.URL,
		Body:     "file=@/no/such/path/exists",
		BodyType: history.BodyMultipart,
	})
	if err == nil {
		t.Error("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "multipart: open") {
		t.Errorf("error should mention multipart open: %v", err)
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

	orig := Snapshot()
	c := orig
	c.ProxyURL = "://invalid" // url.Parse rejects empty scheme
	SetConfig(c)
	defer SetConfig(orig)

	resp, err := Do(history.Request{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatalf("expected success with malformed proxy_url, got %v", err)
	}
	if resp.Status != 200 || resp.Body != "ok" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

// TestConfig_ConcurrentSetAndDo exercises the data race that motivated the
// snapshot refactor: the UI goroutine swapping config via SetConfig while a
// Send goroutine reads it inside Do. Run with -race to catch regressions.
func TestConfig_ConcurrentSetAndDo(t *testing.T) {
	orig := Snapshot()
	defer SetConfig(orig)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	done := make(chan struct{})
	stopped := make(chan struct{})
	// Writer: continuously swap config, mimicking applySettings / Ctrl+T.
	go func() {
		defer close(stopped)
		for i := 0; ; i++ {
			select {
			case <-done:
				return
			default:
			}
			c := Snapshot()
			c.SkipTLSVerify = i%2 == 0
			c.RequestTimeout = 30 * time.Second
			c.ProxyURL = ""
			SetConfig(c)
		}
	}()

	// Reader: fire requests that read the config inside Do.
	for i := 0; i < 200; i++ {
		if _, err := Do(history.Request{Method: "GET", URL: srv.URL}); err != nil {
			t.Fatalf("Do: %v", err)
		}
	}
	// Wait for the writer to actually exit before returning; otherwise it
	// could keep calling SetConfig concurrently with a subsequent test and
	// flip the global config that test relies on.
	close(done)
	<-stopped
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
