package curlparse

import (
	"encoding/base64"
	"reflect"
	"strings"
	"testing"

	"github.com/lea-151107/pollen/internal/history"
)

func TestParse_SimpleGET(t *testing.T) {
	req, err := Parse("curl https://example.com/api")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Method != "GET" || req.URL != "https://example.com/api" {
		t.Errorf("req: %+v", req)
	}
}

func TestParse_PostWithDataDefaultsPost(t *testing.T) {
	req, err := Parse(`curl https://example.com/api -d '{"a":1}'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Method != "POST" {
		t.Errorf("method should default POST when -d present, got %s", req.Method)
	}
	if req.Body != `{"a":1}` {
		t.Errorf("body: %q", req.Body)
	}
	if req.BodyType != history.BodyRaw {
		t.Errorf("expected BodyRaw, got %q", req.BodyType)
	}
}

func TestParse_ExplicitMethodWins(t *testing.T) {
	req, err := Parse(`curl -X PUT https://example.com/api -d "body"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Method != "PUT" {
		t.Errorf("method: %s", req.Method)
	}
}

func TestParse_HeadersAccumulate(t *testing.T) {
	req, err := Parse(`curl https://x -H "X-One: 1" -H "X-Two: two"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []history.Header{
		{Key: "X-One", Value: "1"},
		{Key: "X-Two", Value: "two"},
	}
	if !reflect.DeepEqual(req.Headers, want) {
		t.Errorf("headers: %+v", req.Headers)
	}
}

func TestParse_ContentTypeJSONFlipsBodyType(t *testing.T) {
	req, err := Parse(`curl https://x -H "Content-Type: application/json" -d '{"a":1}'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.BodyType != history.BodyJSON {
		t.Errorf("BodyType: %q", req.BodyType)
	}
}

func TestParse_DataUrlencode(t *testing.T) {
	req, err := Parse(`curl https://x --data-urlencode "name=alice" --data-urlencode "city=Tokyo"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.BodyType != history.BodyForm {
		t.Errorf("BodyType: %q", req.BodyType)
	}
	if req.Body != "name=alice\ncity=Tokyo" {
		t.Errorf("body: %q", req.Body)
	}
}

func TestParse_MultipartForm(t *testing.T) {
	req, err := Parse(`curl https://x -F 'meta=hello' -F 'img=@/tmp/x.png;type=image/png'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.BodyType != history.BodyMultipart {
		t.Errorf("BodyType: %q", req.BodyType)
	}
	want := "meta=hello\nimg=@/tmp/x.png;type=image/png"
	if req.Body != want {
		t.Errorf("body: got %q want %q", req.Body, want)
	}
}

func TestParse_BasicAuthBecomesHeader(t *testing.T) {
	req, err := Parse(`curl https://x -u 'alice:hunter2'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:hunter2"))
	found := false
	for _, h := range req.Headers {
		if h.Key == "Authorization" && h.Value == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Authorization header missing, got %+v", req.Headers)
	}
}

func TestParse_UserAgentAndReferer(t *testing.T) {
	req, err := Parse(`curl https://x -A "my-agent/1.0" -e "https://referrer.example"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, want := range []history.Header{
		{Key: "User-Agent", Value: "my-agent/1.0"},
		{Key: "Referer", Value: "https://referrer.example"},
	} {
		found := false
		for _, h := range req.Headers {
			if h == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing header %+v in %+v", want, req.Headers)
		}
	}
}

func TestParse_GetFlagForcesGet(t *testing.T) {
	req, err := Parse(`curl -G https://x --data-urlencode 'q=test'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Method != "GET" {
		t.Errorf("expected GET (overrides body-implied POST), got %s", req.Method)
	}
}

func TestParse_TransportFlagsIgnored(t *testing.T) {
	req, err := Parse(`curl -sLv -k https://x`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.URL != "https://x" {
		t.Errorf("url: %q", req.URL)
	}
}

func TestParse_BackslashContinuation(t *testing.T) {
	cmd := "curl https://example.com \\\n  -H 'X: y' \\\n  -d 'body'"
	req, err := Parse(cmd)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.URL != "https://example.com" || req.Body != "body" {
		t.Errorf("req: %+v", req)
	}
	if len(req.Headers) != 1 || req.Headers[0].Key != "X" {
		t.Errorf("headers: %+v", req.Headers)
	}
}

func TestParse_DoubleQuoteEscapes(t *testing.T) {
	req, err := Parse(`curl https://x -d "{\"a\":1}"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Body != `{"a":1}` {
		t.Errorf("body: %q", req.Body)
	}
}

func TestParse_SingleQuotesLiteral(t *testing.T) {
	// Single quotes don't process backslash escapes — the input
	// `'\"foo\"'` should come through as the literal `\"foo\"`.
	req, err := Parse(`curl https://x -d '\"foo\"'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Body != `\"foo\"` {
		t.Errorf("body: %q", req.Body)
	}
}

func TestParse_NoURL(t *testing.T) {
	_, err := Parse("curl -X POST")
	if err == nil {
		t.Error("expected error for missing URL")
	}
}

func TestParse_UnsupportedFlagSurfacesError(t *testing.T) {
	_, err := Parse(`curl --some-future-flag x https://example.com`)
	if err == nil {
		t.Error("expected error for unsupported flag")
	}
}

func TestParse_UnterminatedQuote(t *testing.T) {
	_, err := Parse(`curl https://x -H "X: missing-end`)
	if err == nil {
		t.Error("expected error for unterminated quote")
	}
	if !strings.Contains(err.Error(), "unterminated") {
		t.Errorf("error should mention unterminated: %v", err)
	}
}

func TestParse_NoCurlPrefix(t *testing.T) {
	// Some users paste the args alone; accept that too.
	req, err := Parse(`-X POST https://example.com -d 'body'`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.URL != "https://example.com" || req.Body != "body" {
		t.Errorf("req: %+v", req)
	}
}
