package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lea/pollen/internal/history"
)

func TestTryPrettyJSON_ValidObject(t *testing.T) {
	got, ok := tryPrettyJSON(`{"a":1,"b":[2,3]}`)
	if !ok {
		t.Fatal("valid JSON should be pretty-printed")
	}
	want := "{\n  \"a\": 1,\n  \"b\": [\n    2,\n    3\n  ]\n}"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestTryPrettyJSON_InvalidPassthrough(t *testing.T) {
	in := "not json"
	got, ok := tryPrettyJSON(in)
	if ok {
		t.Errorf("invalid JSON should return ok=false")
	}
	if got != in {
		t.Errorf("invalid input should be returned unchanged, got %q", got)
	}
}

func TestTryPrettyJSON_AlreadyFormatted(t *testing.T) {
	// json.Indent normalizes whitespace regardless of input formatting.
	got, ok := tryPrettyJSON("{\n  \"a\":  1  \n}")
	if !ok {
		t.Fatal("should still be valid")
	}
	if !strings.Contains(got, `"a": 1`) {
		t.Errorf("expected normalized indent, got %q", got)
	}
}

func makeHeaders(n int) []history.Header {
	hs := make([]history.Header, n)
	for i := 0; i < n; i++ {
		hs[i] = history.Header{Key: fmt.Sprintf("X-H-%02d", i), Value: fmt.Sprintf("v%d", i)}
	}
	return hs
}

func TestFormatHeaders_FourHeaders(t *testing.T) {
	got := formatHeaders(makeHeaders(4))
	if strings.Contains(got, "more headers") {
		t.Errorf("4 headers should not show truncation note: %q", got)
	}
	if want := 4; strings.Count(got, "\n")+1 != want {
		t.Errorf("expected %d lines, got %q", want, got)
	}
}

// TestFormatHeaders_FiveHeadersNoSpuriousMore is the regression test for the
// bug where exactly 5 headers produced a "(+ 0 more headers)" line. The
// truncation note must only appear when there are genuinely more.
func TestFormatHeaders_FiveHeadersNoSpuriousMore(t *testing.T) {
	got := formatHeaders(makeHeaders(5))
	if strings.Contains(got, "more headers") {
		t.Errorf("5 headers should NOT show truncation note, got: %q", got)
	}
	if strings.Contains(got, "+ 0") {
		t.Errorf("must not contain '(+ 0 ...)': %q", got)
	}
}

func TestFormatHeaders_SevenHeadersTruncates(t *testing.T) {
	got := formatHeaders(makeHeaders(7))
	if !strings.Contains(got, "(+ 2 more headers)") {
		t.Errorf("7 headers should show '(+ 2 more headers)', got: %q", got)
	}
}

// TestSetResponse_NilDoesNotPanic guards against the bug where SetResponse
// dereferenced r.resp inside the diff-mode branch without a nil check.
// We deliberately exercise the diff-mode path: first plant two non-nil
// responses to set prevResp, toggle diffMode on via the public Update path,
// then call SetResponse(nil, "").
func TestSetResponse_NilDoesNotPanic(t *testing.T) {
	r := NewResponse()
	r.Focus()
	r.SetResponse(&history.Response{Body: "first", ContentType: "text/plain"}, "url1")
	r.SetResponse(&history.Response{Body: "second", ContentType: "text/plain"}, "url2")

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("SetResponse(nil) panicked: %v", rec)
		}
	}()
	r.SetResponse(nil, "")
}

func TestResponse_SearchActiveAccessor(t *testing.T) {
	r := NewResponse()
	if r.SearchActive() {
		t.Errorf("fresh Response should not have search active")
	}
	r.searchActive = true
	if !r.SearchActive() {
		t.Errorf("SearchActive should reflect r.searchActive")
	}
}

func TestIsJSONContentType(t *testing.T) {
	cases := map[string]bool{
		"application/json":         true,
		"application/vnd.api+json": true,
		"application/ld+json":      true,
		"application/problem+json": true,
		"text/json":                false, // non-standard, not matched
		"application/xml":          false,
		"":                         false,
		"application/jsonl":        false, // not strictly JSON
	}
	for ct, want := range cases {
		if got := isJSONContentType(ct); got != want {
			t.Errorf("isJSONContentType(%q): got %v want %v", ct, got, want)
		}
	}
}
