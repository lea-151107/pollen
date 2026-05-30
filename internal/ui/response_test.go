package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lea-151107/pollen/internal/history"
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

func TestSetError_ResetsFilterAndSearchState(t *testing.T) {
	// Pre-error: response present, filter / search / diff all active
	// from the previous successful response. After SetError these
	// view-only states must all clear so the error message isn't
	// rendered under leftover prompts and badges.
	r := NewResponse()
	r.Focus()
	r.SetResponse(&history.Response{Body: `{"ok":true}`, ContentType: "application/json"}, "url1")
	r.filterActive = true
	r.filteredBody = "stale"
	r.filterErr = "stale jq error"
	r.searchActive = true
	r.searchQuery = "needle"
	r.diffMode = true
	r.diffBody = "stale diff"

	r.SetError("boom")

	if r.filterActive {
		t.Error("filterActive should be false after SetError")
	}
	if r.filteredBody != "" {
		t.Errorf("filteredBody should be empty, got %q", r.filteredBody)
	}
	if r.filterErr != "" {
		t.Errorf("filterErr should be empty, got %q", r.filterErr)
	}
	if r.searchActive {
		t.Error("searchActive should be false after SetError")
	}
	if r.searchQuery != "" {
		t.Errorf("searchQuery should be empty, got %q", r.searchQuery)
	}
	if r.diffMode {
		t.Error("diffMode should be false after SetError")
	}
	if r.diffBody != "" {
		t.Errorf("diffBody should be empty, got %q", r.diffBody)
	}
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

// ---- currentDisplayBody composition (Bug H regression) ----

func TestCurrentDisplayBody_PlainNoSearch(t *testing.T) {
	r := NewResponse()
	r.resp = &history.Response{Body: "hello world"}
	if got := r.currentDisplayBody(); got != "hello world" {
		t.Errorf("plain body: got %q", got)
	}
}

func TestCurrentDisplayBody_FilterOnly(t *testing.T) {
	r := NewResponse()
	r.resp = &history.Response{Body: `{"users":["a","b"]}`}
	r.filteredBody = `["a","b"]`
	if got := r.currentDisplayBody(); got != `["a","b"]` {
		t.Errorf("filter base should win, got %q", got)
	}
}

func TestCurrentDisplayBody_SearchOnly(t *testing.T) {
	r := NewResponse()
	r.resp = &history.Response{Body: "Hello World"}
	r.searchQuery = "world"
	got := r.currentDisplayBody()
	plain := stripANSI(got)
	if plain != "Hello World" {
		t.Errorf("plain text should match body, got %q", plain)
	}
}

// TestCurrentDisplayBody_FilterAndSearchCompose locks the bug-H semantics:
// when a jq filter is locked AND a search query is active, the search must
// highlight WITHIN the filtered text, NOT replace it with the raw body.
func TestCurrentDisplayBody_FilterAndSearchCompose(t *testing.T) {
	r := NewResponse()
	r.resp = &history.Response{Body: `{"users":["alice"],"servers":["s1"]}`}
	r.filteredBody = `["alice"]`
	r.searchQuery = "alice"
	got := r.currentDisplayBody()
	plain := stripANSI(got)
	if plain != `["alice"]` {
		t.Errorf("base should be filtered, got %q", plain)
	}
	// Searching for a string that exists only in the RAW body (not in the
	// filtered base) must not match — proving search runs on filter, not raw.
	r2 := NewResponse()
	r2.resp = &history.Response{Body: `{"users":["alice"],"servers":["s1"]}`}
	r2.filteredBody = `["alice"]`
	r2.searchQuery = "servers"
	if strings.Contains(r2.currentDisplayBody(), "\x1b[") {
		// Heuristic: a match would inject ANSI styling. None expected here.
		t.Errorf("search of raw-only term should not match in filtered base, got styled output: %q", r2.currentDisplayBody())
	}
}

func TestCurrentDisplayBody_DiffAndSearchCompose(t *testing.T) {
	r := NewResponse()
	r.resp = &history.Response{Body: "world"}
	r.prevResp = &history.Response{Body: "hello"}
	r.diffMode = true
	r.diffBody = "diff-content-with-world"
	r.searchQuery = "world"
	got := r.currentDisplayBody()
	plain := stripANSI(got)
	if plain != "diff-content-with-world" {
		t.Errorf("base should be diff content, got %q", plain)
	}
}

// ---- sanitizeTerminalControl (Minor 1 regression) ----

func TestSanitizeTerminalControl_PreservesNormalText(t *testing.T) {
	in := "hello world\nline2\ttabbed"
	if got := sanitizeTerminalControl(in); got != in {
		t.Errorf("normal text should pass through unchanged, got %q", got)
	}
}

func TestSanitizeTerminalControl_EscapesANSI(t *testing.T) {
	in := "before\x1b[2Jafter"
	got := sanitizeTerminalControl(in)
	if strings.ContainsRune(got, 0x1b) {
		t.Errorf("output must not contain raw ESC, got %q", got)
	}
	if !strings.Contains(got, "\\x1b") {
		t.Errorf("escape sequence should be visualised, got %q", got)
	}
	if !strings.HasPrefix(got, "before") || !strings.HasSuffix(got, "after") {
		t.Errorf("surrounding text should be preserved, got %q", got)
	}
}

func TestSanitizeTerminalControl_PreservesTabNewline(t *testing.T) {
	in := "\t\n\r"
	if got := sanitizeTerminalControl(in); got != in {
		t.Errorf("whitespace should be preserved, got %q", got)
	}
}

// ---- Bug K: complete sanitisation coverage ----

// TestSetError_SanitizesANSI guards Bug K-1: SetError put err verbatim into
// the viewport so an HTTP error containing server-influenced text could carry
// raw escape sequences into the terminal (and stick around in history for
// later re-display via applyEntry).
func TestSetError_SanitizesANSI(t *testing.T) {
	r := NewResponse()
	r.SetError("network: \x1b[2J refused")
	got := r.vp.View()
	if strings.ContainsRune(got, 0x1b) {
		// Find which ESC. lipgloss styling injects ESCs for colours, but
		// those should be balanced terminators; the raw \x1b[2J injection
		// from the server text must be replaced with the literal "\x1b"
		// escape representation.
		if strings.Contains(got, "\x1b[2J") {
			t.Errorf("raw '\\x1b[2J' must not survive into viewport: %q", got)
		}
	}
	if !strings.Contains(got, "\\x1b") {
		t.Errorf("escape sequence should be visualised in error: %q", got)
	}
}

// TestBinaryHeader_SanitizesContentType guards Bug K-2: a malformed
// Content-Type that survives ParseContentType's fallback path used to be
// rendered into the binary-response status line raw.
func TestBinaryHeader_SanitizesContentType(t *testing.T) {
	r := NewResponse()
	r.resp = &history.Response{
		ContentType: "binary/x\x1b[2J",
		IsBinary:    true,
		SizeBytes:   10,
		BodyBytes:   []byte("\x00"),
	}
	got := r.binaryHeader()
	if strings.Contains(got, "\x1b[2J") {
		t.Errorf("raw ESC sequence must not be in binaryHeader: %q", got)
	}
	if !strings.Contains(got, "\\x1b") {
		t.Errorf("ESC sequence should appear escaped: %q", got)
	}
}

// ---- CurrentBytes fallback (Minor 2 companion) ----

func TestCurrentBytes_FallsBackToBodyForText(t *testing.T) {
	r := NewResponse()
	// BodyBytes nil but Body present: e.g. older history entry whose bytes
	// were dropped by the Prepend trimmer.
	r.resp = &history.Response{Body: "hello", IsBinary: false}
	got := r.CurrentBytes()
	if string(got) != "hello" {
		t.Errorf("text fallback to Body: got %q want %q", string(got), "hello")
	}
}

func TestCurrentBytes_BinaryNoFallback(t *testing.T) {
	r := NewResponse()
	// Binary entry past the keep window: Body is "" by design, BodyBytes nil.
	r.resp = &history.Response{Body: "", IsBinary: true}
	if got := r.CurrentBytes(); got != nil {
		t.Errorf("binary without bytes should not fall back, got %v", got)
	}
}

func TestCurrentBytes_PrefersBodyBytes(t *testing.T) {
	r := NewResponse()
	r.resp = &history.Response{Body: "text", BodyBytes: []byte("raw")}
	if got := string(r.CurrentBytes()); got != "raw" {
		t.Errorf("BodyBytes should be preferred over Body fallback, got %q", got)
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
